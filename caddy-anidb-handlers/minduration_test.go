package errorbodystatus

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type recordingHandler struct {
	starts   chan time.Time
	releases []chan struct{}
	count    int64
}

func (h *recordingHandler) ServeHTTP(http.ResponseWriter, *http.Request) error {
	index := int(atomic.AddInt64(&h.count, 1) - 1)
	h.starts <- time.Now()
	if index < len(h.releases) {
		<-h.releases[index]
	}
	return nil
}

func TestMinDurationHandler_WaitsAfterCompletion(t *testing.T) {
	minDuration := 80 * time.Millisecond
	handler := MinDurationHandler{Duration: caddy.Duration(minDuration), JitterFactor: 0}

	starts := make(chan time.Time, 2)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	close(releaseSecond)

	next := &recordingHandler{
		starts:   starts,
		releases: []chan struct{}{releaseFirst, releaseSecond},
	}

	req1 := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	err1 := make(chan error, 1)
	go func() {
		err := handler.ServeHTTP(httptest.NewRecorder(), req1, caddyhttp.HandlerFunc(next.ServeHTTP))
		err1 <- err
	}()

	var start1 time.Time
	select {
	case start1 = <-starts:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for first request to reach upstream")
	}

	err2 := make(chan error, 1)
	go func() {
		err2 <- handler.ServeHTTP(httptest.NewRecorder(), req2, caddyhttp.HandlerFunc(next.ServeHTTP))
	}()

	select {
	case <-starts:
		t.Fatal("second request reached upstream before min duration elapsed")
	case <-time.After(minDuration / 2):
	}

	var start2 time.Time
	select {
	case start2 = <-starts:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second request to reach upstream")
	}

	close(releaseFirst)

	select {
	case err := <-err1:
		if err != nil {
			t.Fatalf("first request failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for first request error")
	}

	select {
	case err := <-err2:
		if err != nil {
			t.Fatalf("second request failed: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second request error")
	}

	if gap := start2.Sub(start1); gap < minDuration {
		t.Fatalf("expected at least %v between upstream calls, got %v", minDuration, gap)
	}
}
