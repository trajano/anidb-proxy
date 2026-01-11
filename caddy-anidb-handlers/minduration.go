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
	JitterFactor  float64        `json:"jitter_factor,omitempty"`
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

	if err := h.waitForSlot(r.Context(), minDuration, jitterFactor); err != nil {
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
