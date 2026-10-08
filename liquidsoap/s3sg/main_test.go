package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(endpoint string) config {
	parsed, _ := url.Parse(endpoint)
	return config{
		endpoint:      parsed,
		bucket:        "radio-library",
		region:        "us-east-1",
		accessKey:     "test-access-key",
		secretKey:     "test-secret-key",
		sessionToken:  "test-session-token",
		forcePath:     true,
		tracksPrefix:  "tracks/",
		jinglesPrefix: "jingles/",
		refresh:       time.Hour,
		maxObjects:    100,
	}
}

func TestListObjectsPaginatesAndFiltersAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSignedRequest(t, r)
		if r.URL.Path != "/radio-library" || r.URL.Query().Get("list-type") != "2" {
			t.Fatalf("unexpected list request: %s", r.URL.String())
		}
		if r.URL.Query().Get("prefix") != "tracks/" {
			t.Fatalf("unexpected prefix: %q", r.URL.Query().Get("prefix"))
		}

		switch r.URL.Query().Get("continuation-token") {
		case "":
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>page-two</NextContinuationToken><Contents><Key>tracks/one.MP3</Key></Contents><Contents><Key>tracks/readme.txt</Key></Contents></ListBucketResult>`)
		case "page-two":
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>tracks/two song.flac</Key></Contents></ListBucketResult>`)
		default:
			t.Fatalf("unexpected continuation token: %q", r.URL.Query().Get("continuation-token"))
		}
	}))
	defer server.Close()

	gateway, err := newGateway(testConfig(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := gateway.listObjects(context.Background(), "tracks/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tracks/one.MP3", "tracks/two song.flac"}
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		t.Fatalf("got keys %q, want %q", keys, want)
	}
}

func TestStreamProxiesObjectBytesAsTheyArrive(t *testing.T) {
	firstChunkSent := make(chan struct{})
	releaseSecondChunk := make(chan struct{})
	objectRequested := make(chan struct{}, 1)
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseSecondChunk) })
	}
	defer release()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSignedRequest(t, r)
		switch {
		case r.URL.Path == "/radio-library":
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>tracks/song.mp3</Key></Contents></ListBucketResult>`)
		case r.URL.Path == "/radio-library/tracks/song.mp3":
			objectRequested <- struct{}{}
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
			if _, err := io.WriteString(w, "first"); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			close(firstChunkSent)
			select {
			case <-releaseSecondChunk:
				_, _ = io.WriteString(w, "second")
			case <-r.Context().Done():
			}
		default:
			t.Errorf("unexpected upstream request: %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	gateway, err := newGateway(testConfig(upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(gateway.handler())
	defer local.Close()

	response, err := http.Get(local.URL + "/stream/tracks")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("got HTTP %d, want 200", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("unexpected content type %q", response.Header.Get("Content-Type"))
	}

	first := make([]byte, 5)
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstChunkSent:
	case <-time.After(time.Second):
		t.Fatal("the object response was buffered instead of streamed")
	}
	if string(first) != "first" {
		t.Fatalf("got first chunk %q", first)
	}
	select {
	case <-objectRequested:
	case <-time.After(time.Second):
		t.Fatal("S3 object was not requested")
	}

	release()
	rest, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "second" {
		t.Fatalf("got remaining bytes %q, want second", rest)
	}
}

func TestSignedRequestIncludesSessionToken(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://s3.example.test/bucket/a%20b.mp3?list-type=2&prefix=tracks%2F", nil)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := newGateway(testConfig("https://s3.example.test"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.signRequest(request, time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(request.Header.Get("Authorization"), "SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token") {
		t.Fatalf("unexpected signed headers: %s", request.Header.Get("Authorization"))
	}
	if request.Header.Get("x-amz-security-token") != "test-session-token" {
		t.Fatal("session token was not attached to the signed request")
	}
	if awsEscape("two words~") != "two%20words~" {
		t.Fatalf("AWS query escaping mismatch: %q", awsEscape("two words~"))
	}
	if got := canonicalURI(request.URL); got != "/bucket/a%20b.mp3" {
		t.Fatalf("got canonical URI %q", got)
	}
}

func TestLoadConfigRequiresCredentialPairAndUniquePrefixes(t *testing.T) {
	values := map[string]string{
		"S3_BUCKET":            "radio-library",
		"S3_ACCESS_KEY_ID":     "key",
		"S3_SECRET_ACCESS_KEY": "secret",
		"S3_TRACKS_PREFIX":     "music/tracks/",
		"S3_JINGLES_PREFIX":    "music/jingles/",
	}
	getenv := func(key string) string { return values[key] }
	if _, err := loadConfig(getenv); err != nil {
		t.Fatalf("valid S3 configuration rejected: %v", err)
	}

	delete(values, "S3_SECRET_ACCESS_KEY")
	if _, err := loadConfig(getenv); err == nil {
		t.Fatal("configuration without a secret key was accepted")
	}
	values["S3_SECRET_ACCESS_KEY"] = "secret"
	values["S3_JINGLES_PREFIX"] = "music/tracks"
	if _, err := loadConfig(getenv); err == nil {
		t.Fatal("identical tracks and jingles prefixes were accepted")
	}
}

func assertSignedRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if !strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-access-key/") {
		t.Errorf("missing SigV4 Authorization header")
	}
	if request.Header.Get("x-amz-date") == "" || request.Header.Get("x-amz-content-sha256") == "" {
		t.Errorf("missing SigV4 headers")
	}
	if request.Header.Get("x-amz-security-token") != "test-session-token" {
		t.Errorf("missing S3 session token")
	}
}
