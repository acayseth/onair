package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultListenAddress = "127.0.0.1:8090"
	defaultRegion        = "us-east-1"
	defaultRefresh       = 5 * time.Minute
	defaultMaxObjects    = 100000
	listPageSize         = 1000
	catalogRetryDelay    = 30 * time.Second
)

type config struct {
	endpoint      *url.URL
	bucket        string
	region        string
	accessKey     string
	secretKey     string
	sessionToken  string
	forcePath     bool
	tracksPrefix  string
	jinglesPrefix string
	refresh       time.Duration
	maxObjects    int
}

type objectList struct {
	XMLName               xml.Name `xml:"ListBucketResult"`
	IsTruncated           bool     `xml:"IsTruncated"`
	NextContinuationToken string   `xml:"NextContinuationToken"`
	Contents              []struct {
		Key string `xml:"Key"`
	} `xml:"Contents"`
}

type catalog struct {
	mu          sync.Mutex
	keys        []string
	remaining   []string
	lastRefresh time.Time
	retryAfter  time.Time
	lastError   error
}

type gateway struct {
	config config
	client *http.Client
	rng    *rand.Rand
	rngMu  sync.Mutex

	catalogs map[string]*catalog
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--health-check" {
		if err := checkHealth(); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		log.Fatal("unexpected command-line arguments")
	}

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	server, err := newGateway(cfg)
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:              defaultListenAddress,
		Handler:           server.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}

	log.Printf("S3 streaming gateway listening on %s", defaultListenAddress)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func checkHealth() error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + defaultListenAddress + "/healthz")
	if err != nil {
		return fmt.Errorf("S3 streaming gateway is not ready: %w", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("S3 streaming gateway health check returned HTTP %d", response.StatusCode)
	}
	return nil
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		bucket:        strings.TrimSpace(getenv("S3_BUCKET")),
		region:        strings.TrimSpace(getenv("S3_REGION")),
		accessKey:     strings.TrimSpace(getenv("S3_ACCESS_KEY_ID")),
		secretKey:     getenv("S3_SECRET_ACCESS_KEY"),
		sessionToken:  getenv("S3_SESSION_TOKEN"),
		tracksPrefix:  cleanPrefix(getenv("S3_TRACKS_PREFIX"), "tracks/"),
		jinglesPrefix: cleanPrefix(getenv("S3_JINGLES_PREFIX"), "jingles/"),
	}

	if cfg.region == "" {
		cfg.region = defaultRegion
	}
	if cfg.bucket == "" {
		return config{}, errors.New("S3_BUCKET is required when starting the S3 streaming gateway")
	}
	if cfg.accessKey == "" || cfg.secretKey == "" {
		return config{}, errors.New("S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required")
	}
	if strings.Contains(cfg.bucket, "/") || strings.Contains(cfg.bucket, "?") || strings.Contains(cfg.bucket, "#") {
		return config{}, errors.New("S3_BUCKET must be a bucket name, not a URL or path")
	}
	if cfg.tracksPrefix == cfg.jinglesPrefix {
		return config{}, errors.New("S3_TRACKS_PREFIX and S3_JINGLES_PREFIX must be different")
	}
	if strings.HasPrefix(cfg.tracksPrefix, cfg.jinglesPrefix) || strings.HasPrefix(cfg.jinglesPrefix, cfg.tracksPrefix) {
		return config{}, errors.New("S3_TRACKS_PREFIX and S3_JINGLES_PREFIX must not overlap")
	}

	endpointValue := strings.TrimSpace(getenv("S3_ENDPOINT_URL"))
	if endpointValue == "" {
		endpointValue = "https://s3." + cfg.region + ".amazonaws.com"
	}
	endpoint, err := url.Parse(endpointValue)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return config{}, errors.New("S3_ENDPOINT_URL must be an HTTP(S) origin with no credentials, query, or fragment")
	}
	cfg.endpoint = endpoint

	pathStyleValue := strings.TrimSpace(getenv("S3_FORCE_PATH_STYLE"))
	cfg.forcePath = true
	if pathStyleValue != "" {
		cfg.forcePath, err = strconv.ParseBool(pathStyleValue)
		if err != nil {
			return config{}, errors.New("S3_FORCE_PATH_STYLE must be true or false")
		}
	}

	cfg.refresh, err = parsePositiveDuration(getenv("S3_CATALOG_REFRESH_SECONDS"), defaultRefresh)
	if err != nil {
		return config{}, fmt.Errorf("S3_CATALOG_REFRESH_SECONDS: %w", err)
	}
	cfg.maxObjects, err = parsePositiveInt(getenv("S3_MAX_OBJECTS"), defaultMaxObjects)
	if err != nil {
		return config{}, fmt.Errorf("S3_MAX_OBJECTS: %w", err)
	}

	return cfg, nil
}

func cleanPrefix(value, fallback string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		value = strings.Trim(fallback, "/")
	}
	return value + "/"
}

func parsePositiveDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0, errors.New("must be a positive number of seconds")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parsePositiveInt(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil || count <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return count, nil
}

func newGateway(cfg config) (*gateway, error) {
	if cfg.endpoint == nil || cfg.bucket == "" || cfg.accessKey == "" || cfg.secretKey == "" {
		return nil, errors.New("incomplete S3 gateway configuration")
	}
	return &gateway{
		config: cfg,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          32,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 20 * time.Second,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		catalogs: map[string]*catalog{
			"tracks":  {},
			"jingles": {},
		},
	}, nil
}

func (g *gateway) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /stream/tracks", g.streamHandler("tracks"))
	mux.HandleFunc("GET /stream/jingles", g.streamHandler("jingles"))
	return mux
}

func (g *gateway) streamHandler(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefix := g.config.tracksPrefix
		if kind == "jingles" {
			prefix = g.config.jinglesPrefix
		}

		key, err := g.nextObject(r.Context(), kind, prefix)
		if err != nil {
			log.Printf("S3 %s catalog unavailable: %v", kind, err)
			http.Error(w, "audio catalog temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		upstream, err := g.getObject(r, key)
		if err != nil {
			log.Printf("S3 %s object request failed: %v", kind, err)
			http.Error(w, "audio object temporarily unavailable", http.StatusBadGateway)
			return
		}
		defer upstream.Body.Close()

		copyHeader(w.Header(), upstream.Header, "Content-Type", "Content-Length", "Accept-Ranges", "Content-Range", "ETag", "Last-Modified")
		w.Header().Set("icy-title", path.Base(key))
		w.Header().Set("icy-radio-jingle", strconv.FormatBool(kind == "jingles"))
		w.WriteHeader(upstream.StatusCode)
		if upstream.StatusCode == http.StatusNoContent || r.Method == http.MethodHead {
			return
		}

		flusher, canFlush := w.(http.Flusher)
		if canFlush {
			flusher.Flush()
		}
		buffer := make([]byte, 32*1024)
		for {
			n, readErr := upstream.Body.Read(buffer)
			if n > 0 {
				if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
					return
				}
				if canFlush {
					flusher.Flush()
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, context.Canceled) {
					log.Printf("S3 %s stream ended with an error", kind)
				}
				return
			}
		}
	}
}

func copyHeader(dst, src http.Header, names ...string) {
	for _, name := range names {
		for _, value := range src.Values(name) {
			dst.Add(name, value)
		}
	}
}

func (g *gateway) nextObject(ctx context.Context, kind, prefix string) (string, error) {
	state := g.catalogs[kind]
	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()
	if now.Before(state.retryAfter) {
		if len(state.keys) == 0 {
			return "", state.lastError
		}
	} else if time.Since(state.lastRefresh) >= g.config.refresh || state.keys == nil {
		keys, err := g.listObjects(ctx, prefix)
		if err != nil {
			state.retryAfter = now.Add(catalogRetryDelay)
			state.lastError = err
			if len(state.keys) == 0 {
				return "", err
			}
			log.Printf("S3 %s catalog refresh failed; continuing with cached catalog", kind)
		} else {
			state.keys = keys
			state.remaining = nil
			state.lastRefresh = now
			state.retryAfter = time.Time{}
			state.lastError = nil
		}
		if err == nil {
			log.Printf("Loaded %d %s objects", len(state.keys), kind)
		}
	}

	if len(state.keys) == 0 {
		if state.lastError != nil {
			return "", state.lastError
		}
		return "", fmt.Errorf("no playable audio objects under prefix %q", prefix)
	}

	if len(state.remaining) == 0 {
		state.remaining = append(state.remaining, state.keys...)
		g.rngMu.Lock()
		g.rng.Shuffle(len(state.remaining), func(i, j int) {
			state.remaining[i], state.remaining[j] = state.remaining[j], state.remaining[i]
		})
		g.rngMu.Unlock()
	}

	last := len(state.remaining) - 1
	key := state.remaining[last]
	state.remaining = state.remaining[:last]
	return key, nil
}

func (g *gateway) listObjects(ctx context.Context, prefix string) ([]string, error) {
	keys := make([]string, 0)
	continuationToken := ""
	for {
		query := url.Values{
			"list-type": {"2"},
			"max-keys":  {strconv.Itoa(listPageSize)},
			"prefix":    {prefix},
		}
		if continuationToken != "" {
			query.Set("continuation-token", continuationToken)
		}
		objectURL := g.bucketURL("", query)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create S3 list request: %w", err)
		}
		if err := g.signRequest(request, time.Now()); err != nil {
			return nil, err
		}

		listCtx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
		request = request.WithContext(listCtx)
		response, err := g.client.Do(request)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("request S3 object list: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			cancel()
			return nil, fmt.Errorf("S3 object listing returned HTTP %d", response.StatusCode)
		}

		var page objectList
		decodeErr := xml.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&page)
		response.Body.Close()
		cancel()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode S3 object listing: %w", decodeErr)
		}

		for _, item := range page.Contents {
			if strings.HasPrefix(item.Key, prefix) && playableKey(item.Key) {
				keys = append(keys, item.Key)
				if len(keys) > g.config.maxObjects {
					return nil, fmt.Errorf("S3 prefix exceeds S3_MAX_OBJECTS (%d)", g.config.maxObjects)
				}
			}
		}
		if !page.IsTruncated {
			break
		}
		if page.NextContinuationToken == "" || page.NextContinuationToken == continuationToken {
			return nil, errors.New("S3 returned an invalid continuation token")
		}
		continuationToken = page.NextContinuationToken
	}
	sort.Strings(keys)
	return keys, nil
}

func playableKey(key string) bool {
	switch strings.ToLower(path.Ext(key)) {
	case ".mp3", ".ogg", ".opus", ".flac", ".wav", ".aif", ".aiff", ".aac", ".m4a":
		return true
	default:
		return false
	}
}

func (g *gateway) getObject(incoming *http.Request, key string) (*http.Response, error) {
	objectURL := g.bucketURL(key, nil)
	request, err := http.NewRequestWithContext(incoming.Context(), http.MethodGet, objectURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create S3 object request: %w", err)
	}
	for _, header := range []string{"Range", "If-Range"} {
		if value := incoming.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	if err := g.signRequest(request, time.Now()); err != nil {
		return nil, err
	}
	response, err := g.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request S3 object: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		response.Body.Close()
		return nil, fmt.Errorf("S3 object request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

func (g *gateway) bucketURL(key string, query url.Values) *url.URL {
	result := *g.config.endpoint
	escapedBucket := escapePath(g.config.bucket)
	escapedKey := escapePath(key)

	if g.config.forcePath {
		result.Path = strings.TrimRight(result.Path, "/") + "/" + g.config.bucket
		result.RawPath = strings.TrimRight(g.config.endpoint.EscapedPath(), "/") + "/" + escapedBucket
		if key != "" {
			result.Path += "/" + key
			result.RawPath += "/" + escapedKey
		}
	} else {
		result.Host = g.config.bucket + "." + result.Host
		result.Path = strings.TrimRight(result.Path, "/")
		result.RawPath = strings.TrimRight(g.config.endpoint.EscapedPath(), "/")
		if key != "" {
			result.Path += "/" + key
			result.RawPath += "/" + escapedKey
		}
	}
	result.RawQuery = query.Encode()
	result.ForceQuery = false
	return &result
}

func escapePath(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = awsEscape(part)
	}
	return strings.Join(parts, "/")
}

func (g *gateway) signRequest(request *http.Request, now time.Time) error {
	payloadHash := sha256Hex(nil)
	dateTime := now.UTC().Format("20060102T150405Z")
	date := now.UTC().Format("20060102")
	request.Header.Set("x-amz-date", dateTime)
	request.Header.Set("x-amz-content-sha256", payloadHash)
	if g.config.sessionToken != "" {
		request.Header.Set("x-amz-security-token", g.config.sessionToken)
	}

	headers := map[string]string{
		"host":                 request.URL.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           dateTime,
	}
	if g.config.sessionToken != "" {
		headers["x-amz-security-token"] = g.config.sessionToken
	}
	signedHeaders, canonicalHeaders := canonicalHeaders(headers)
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI(request.URL),
		canonicalQuery(request.URL.Query()),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	scope := date + "/" + g.config.region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + dateTime + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	signature := hex.EncodeToString(hmacSHA256(signingKey(g.config.secretKey, date, g.config.region), stringToSign))
	request.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		g.config.accessKey,
		scope,
		signedHeaders,
		signature,
	))
	return nil
}

func canonicalHeaders(headers map[string]string) (string, string) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	var canonical strings.Builder
	for _, name := range names {
		canonical.WriteString(name)
		canonical.WriteByte(':')
		canonical.WriteString(strings.TrimSpace(headers[name]))
		canonical.WriteByte('\n')
	}
	return strings.Join(names, ";"), canonical.String()
}

func canonicalURI(requestURL *url.URL) string {
	uri := escapePath(requestURL.Path)
	if uri == "" {
		return "/"
	}
	if !strings.HasPrefix(uri, "/") {
		return "/" + uri
	}
	return uri
}

func canonicalQuery(query url.Values) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(query))
	for _, key := range keys {
		values := append([]string(nil), query[key]...)
		sort.Strings(values)
		if len(values) == 0 {
			values = []string{""}
		}
		for _, value := range values {
			parts = append(parts, awsEscape(key)+"="+awsEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func awsEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func signingKey(secret, date, region string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), date)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, "s3")
	return hmacSHA256(serviceKey, "aws4_request")
}
