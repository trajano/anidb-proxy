package errorbodystatus

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(MinDurationHandler{})
	httpcaddyfile.RegisterHandlerDirective("min_duration", parseMinDurationCaddyfile)
}

type MinDurationHandler struct {
	Duration      caddy.Duration `json:"duration,omitempty"`
	JitterFactor  float64        `json:"jitter_factor,omitempty"`
	WaitThreshold caddy.Duration `json:"wait_threshold,omitempty"`
	WaitMode      string         `json:"wait_mode,omitempty"`
	mu            *sync.Mutex
	last          time.Time
}

func (MinDurationHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.min_duration",
		New: func() caddy.Module { return &MinDurationHandler{mu: &sync.Mutex{}} },
	}
}

func (h *MinDurationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	minDuration := time.Duration(h.Duration)
	if minDuration <= 0 {
		minDuration = 2 * time.Second
	}
	jitterFactor := h.JitterFactor
	if jitterFactor <= 0 {
		jitterFactor = 0.01
	}

	threshold := time.Duration(h.WaitThreshold)
	if threshold <= 0 {
		threshold = 5 * time.Second
	}
	mode := h.WaitMode
	if mode == "" {
		mode = "redirect"
	}

	handled, err := h.waitForSlotOrRespond(w, r, r.Context(), minDuration, jitterFactor, threshold, mode)
	if err != nil {
		return err
	}
	if handled {
		// we already wrote a response (redirect or retry-after)
		return nil
	}
	return next.ServeHTTP(w, r)
}

func (h *MinDurationHandler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "duration":
				if !d.NextArg() {
					return d.ArgErr()
				}
				parsed, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("duration must be a valid duration: %v", err)
				}
				h.Duration = caddy.Duration(parsed)
			case "jitter_factor":
				if !d.NextArg() {
					return d.ArgErr()
				}
				parsed, err := strconv.ParseFloat(d.Val(), 64)
				if err != nil {
					return d.Errf("jitter_factor must be a valid number: %v", err)
				}
				if parsed < 0 {
					return d.Err("jitter_factor must be non-negative")
				}
				h.JitterFactor = parsed
			case "wait_threshold":
				if !d.NextArg() {
					return d.ArgErr()
				}
				parsed, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("wait_threshold must be a valid duration: %v", err)
				}
				h.WaitThreshold = caddy.Duration(parsed)
			case "wait_mode":
				if !d.NextArg() {
					return d.ArgErr()
				}
				h.WaitMode = d.Val()
			default:
				return d.Errf("unknown option: %s", d.Val())
			}
		}
	}
	return nil
}

func parseMinDurationCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	handler := MinDurationHandler{mu: &sync.Mutex{}}
	if err := handler.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}
	return &handler, nil
}

func (h *MinDurationHandler) waitForSlot(ctx context.Context, minDuration time.Duration, jitterFactor float64) error {
	// legacy wait, kept for compatibility
	for {
		if h.mu == nil {
			h.mu = &sync.Mutex{}
		}
		h.mu.Lock()
		now := time.Now()
		if h.last.IsZero() || now.Sub(h.last) >= minDuration {
			h.last = now
			h.mu.Unlock()
			return nil
		}
		wait := h.last.Add(minDuration).Sub(now)
		h.mu.Unlock()

		jitter := time.Duration(float64(minDuration) * jitterFactor * rand.Float64())
		timer := time.NewTimer(wait + jitter)
		select {
		case <-timer.C:
			// Re-check against the updated last time.
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (h *MinDurationHandler) waitForSlotOrRespond(w http.ResponseWriter, r *http.Request, ctx context.Context, minDuration time.Duration, jitterFactor float64, threshold time.Duration, mode string) (bool, error) {
	for {
		if h.mu == nil {
			h.mu = &sync.Mutex{}
		}
		h.mu.Lock()
		now := time.Now()
		if h.last.IsZero() || now.Sub(h.last) >= minDuration {
			h.last = now
			h.mu.Unlock()
			return false, nil
		}
		wait := h.last.Add(minDuration).Sub(now)
		h.mu.Unlock()

		jitter := time.Duration(float64(minDuration) * jitterFactor * rand.Float64())
		totalWait := wait + jitter
		if threshold > 0 && totalWait > threshold {
			// exceed threshold: respond immediately
			switch mode {
			case "redirect", "":
				// Temporary redirect; preserve method
				http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
				return true, nil
			case "retry-after":
				secs := int(math.Ceil(totalWait.Seconds()))
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				w.WriteHeader(http.StatusServiceUnavailable)
				return true, nil
			default:
				// unknown mode: default to redirect
				http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
				return true, nil
			}
		}

		timer := time.NewTimer(totalWait)
		select {
		case <-timer.C:
			// Re-check against updated last time
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		}
	}
}
