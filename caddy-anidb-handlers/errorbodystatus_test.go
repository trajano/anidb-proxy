package errorbodystatus

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBufferingWriterFlushErrorHandling(t *testing.T) {
	t.Run("error_without_notfound_disables_cache", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		bw := &bufferingWriter{
			ResponseWriter:  recorder,
			prefix:          []byte("ERR"),
			notFoundMessage: []byte("NF"),
			status:          http.StatusInternalServerError,
			maxBytes:        8,
		}
		payload := []byte("ERRxxxxxx")
		bw.buf.Write(payload)
		bw.Flush()

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
		}
		if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
			t.Fatalf("expected Cache-Control no-store, got %q", cacheControl)
		}
		if body := recorder.Body.String(); body != string(payload) {
			t.Fatalf("expected body %q, got %q", string(payload), body)
		}
	})

	t.Run("error_with_notfound_allows_cache", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		bw := &bufferingWriter{
			ResponseWriter:  recorder,
			prefix:          []byte("ERR"),
			notFoundMessage: []byte("NF"),
			status:          http.StatusInternalServerError,
			maxBytes:        8,
		}
		payload := []byte("ERRNFxxx")
		bw.buf.Write(payload)
		bw.Flush()

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
		}
		if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl == "no-store" {
			t.Fatalf("expected Cache-Control to allow caching, got %q", cacheControl)
		}
		if body := recorder.Body.String(); body != string(payload) {
			t.Fatalf("expected body %q, got %q", string(payload), body)
		}
	})
}
