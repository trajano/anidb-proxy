package errorbodystatus

import (
	"context"
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
	JitterPercent float64        `json:"jitter_percent,omitempty"`
	mu            sync.Mutex
	last          time.Time
}

func (MinDurationHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.min_duration",
		New: func() caddy.Module { return new(MinDurationHandler) },
	}
}

func (h *MinDurationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	minDuration := time.Duration(h.Duration)
	if minDuration <= 0 {
		minDuration = 2 * time.Second
	}
	jitterPercent := h.JitterPercent
	if jitterPercent <= 0 {
		jitterPercent = 1
	}

	if err := h.waitForSlot(r.Context(), minDuration, jitterPercent); err != nil {
		return err
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
			case "jitter_percent":
				if !d.NextArg() {
					return d.ArgErr()
				}
				parsed, err := strconv.ParseFloat(d.Val(), 64)
				if err != nil {
					return d.Errf("jitter_percent must be a valid number: %v", err)
				}
				if parsed < 0 {
					return d.Err("jitter_percent must be non-negative")
				}
				h.JitterPercent = parsed
			default:
				return d.Errf("unknown option: %s", d.Val())
			}
		}
	}
	return nil
}

func parseMinDurationCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var handler MinDurationHandler
	if err := handler.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}
	return &handler, nil
}

func (h *MinDurationHandler) waitForSlot(ctx context.Context, minDuration time.Duration, jitterPercent float64) error {
	for {
		h.mu.Lock()
		now := time.Now()
		if h.last.IsZero() || now.Sub(h.last) >= minDuration {
			h.last = now
			h.mu.Unlock()
			return nil
		}
		wait := h.last.Add(minDuration).Sub(now)
		h.mu.Unlock()

		jitter := time.Duration(float64(minDuration) * (jitterPercent / 100) * rand.Float64())
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
