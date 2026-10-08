package main

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

var trackReads atomic.Int64
var jingleReads atomic.Int64

func main() {
	audio := testWave()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /radio-library", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing signed S3 request", http.StatusUnauthorized)
			return
		}
		prefix := r.URL.Query().Get("prefix")
		if prefix != "tracks/" && prefix != "jingles/" {
			http.Error(w, "unexpected prefix", http.StatusBadRequest)
			return
		}
		key := prefix + "tone.wav"
		_ = xml.NewEncoder(w).Encode(struct {
			XMLName     xml.Name `xml:"ListBucketResult"`
			IsTruncated bool     `xml:"IsTruncated"`
			Contents    []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
		}{
			XMLName: xml.Name{Local: "ListBucketResult"},
			Contents: []struct {
				Key string `xml:"Key"`
			}{{Key: key}},
		})
	})
	mux.HandleFunc("GET /radio-library/{key...}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing signed S3 request", http.StatusUnauthorized)
			return
		}
		key := r.PathValue("key")
		switch {
		case strings.HasPrefix(key, "tracks/"):
			trackReads.Add(1)
		case strings.HasPrefix(key, "jingles/"):
			jingleReads.Add(1)
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Content-Length", fmt.Sprint(len(audio)))
		_, _ = w.Write(audio)
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "tracks=%d jingles=%d\n", trackReads.Load(), jingleReads.Load())
	})

	log.Fatal(http.ListenAndServe("127.0.0.1:8091", mux))
}

func testWave() []byte {
	const (
		sampleRate = 44100
		seconds    = 2
		channels   = 1
		bitDepth   = 16
	)
	dataSize := sampleRate * seconds * channels * bitDepth / 8
	wave := make([]byte, 44+dataSize)
	copy(wave[0:4], "RIFF")
	binary.LittleEndian.PutUint32(wave[4:8], uint32(len(wave)-8))
	copy(wave[8:12], "WAVE")
	copy(wave[12:16], "fmt ")
	binary.LittleEndian.PutUint32(wave[16:20], 16)
	binary.LittleEndian.PutUint16(wave[20:22], 1)
	binary.LittleEndian.PutUint16(wave[22:24], channels)
	binary.LittleEndian.PutUint32(wave[24:28], sampleRate)
	binary.LittleEndian.PutUint32(wave[28:32], sampleRate*channels*bitDepth/8)
	binary.LittleEndian.PutUint16(wave[32:34], channels*bitDepth/8)
	binary.LittleEndian.PutUint16(wave[34:36], bitDepth)
	copy(wave[36:40], "data")
	binary.LittleEndian.PutUint32(wave[40:44], uint32(dataSize))
	return wave
}
