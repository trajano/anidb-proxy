package errorbodystatus

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
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
	Duration caddy.Duration `json:"duration,omitempty"`
}

func (MinDurationHandler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.min_duration",
		New: func() caddy.Module { return new(MinDurationHandler) },
	}
}

func (h MinDurationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	minDuration := time.Duration(h.Duration)
	if minDuration <= 0 {
		minDuration = 2 * time.Second
	}

	bw := &minDurationWriter{
		ResponseWriter: w,
		header:         make(http.Header),
		minDuration:    minDuration,
		start:          time.Now(),
	}
	err := next.ServeHTTP(bw, r)
	if err != nil {
		return err
	}
	bw.flush()
	return nil
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

type minDurationWriter struct {
	http.ResponseWriter

	header      http.Header
	status      int
	wroteHeader bool
	buf         bytes.Buffer

	minDuration time.Duration
	start       time.Time
	flushed     bool
}

func (bw *minDurationWriter) Header() http.Header {
	return bw.header
}

func (bw *minDurationWriter) WriteHeader(status int) {
	if bw.wroteHeader {
		return
	}
	bw.wroteHeader = true
	bw.status = status
}

func (bw *minDurationWriter) Write(p []byte) (int, error) {
	if !bw.wroteHeader {
		bw.WriteHeader(http.StatusOK)
	}
	_, _ = bw.buf.Write(p)
	return len(p), nil
}

func (bw *minDurationWriter) Flush() {
	// Intentionally suppress streaming flushes to honor the minimum duration.
}

func (bw *minDurationWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	bw.flush()
	hijacker, ok := bw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (bw *minDurationWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := bw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (bw *minDurationWriter) Unwrap() http.ResponseWriter {
	return bw.ResponseWriter
}

func (bw *minDurationWriter) flush() {
	if bw.flushed {
		return
	}
	bw.flushed = true

	if !bw.wroteHeader {
		bw.status = http.StatusOK
	}

	elapsed := time.Since(bw.start)
	if remaining := bw.minDuration - elapsed; remaining > 0 {
		time.Sleep(remaining)
	}

	dst := bw.ResponseWriter.Header()
	for key, values := range bw.header {
		dst[key] = append([]string(nil), values...)
	}
	bw.ResponseWriter.WriteHeader(bw.status)
	if bw.buf.Len() > 0 {
		bw.ResponseWriter.Write(bw.buf.Bytes())
		bw.buf.Reset()
	}
}
