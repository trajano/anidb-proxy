package errorbodystatus

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Handler{})
	httpcaddyfile.RegisterHandlerDirective("error_body_status", parseCaddyfile)
}

type Handler struct {
	Prefix          string `json:"prefix,omitempty"`
	Status          int    `json:"status,omitempty"`
	NotFoundMessage string `json:"not_found_message,omitempty"`
	NotFoundStatus  int    `json:"not_found_status,omitempty"`
	MaxBytes        int    `json:"max_bytes,omitempty"`
}

// CaddyModule returns the module information for Caddy.
func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.error_body_status",
		New: func() caddy.Module { return new(Handler) },
	}
}

// ServeHTTP rewrites status codes when the response body matches the configured error prefix.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	prefix := []byte(h.Prefix)
	if len(prefix) == 0 {
		prefix = []byte(`<error code="500">`)
	}
	notFoundMessage := []byte(h.NotFoundMessage)
	if len(notFoundMessage) == 0 {
		prefix = []byte(`<error>Anime not found</error>`)
	}

	status := h.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	notFoundStatus := h.NotFoundStatus
	if notFoundStatus == 0 {
		notFoundStatus = http.StatusNotFound
	}

	maxBytes := h.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64
	}
	if maxBytes < len(prefix) {
		maxBytes = len(prefix)
	}

	bw := &bufferingWriter{
		ResponseWriter:  w,
		prefix:          prefix,
		status:          status,
		notFoundStatus:  notFoundStatus,
		notFoundMessage: notFoundMessage,
		maxBytes:        maxBytes,
	}
	err := next.ServeHTTP(bw, r)
	if err != nil {
		return err
	}
	bw.flushIfNeeded()
	return nil
}

// UnmarshalCaddyfile configures the handler from Caddyfile tokens.
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "prefix":
				if !d.NextArg() {
					return d.ArgErr()
				}
				h.Prefix = d.Val()
			case "not_found_message":
				if !d.NextArg() {
					return d.ArgErr()
				}
				h.NotFoundMessage = d.Val()
			case "status":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("status must be an integer: %v", err)
				}
				h.Status = val
			case "not_found_status":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("not_found_status must be an integer: %v", err)
				}
				h.NotFoundStatus = val
			case "max_bytes":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := strconv.Atoi(d.Val())
				if err != nil {
					return d.Errf("max_bytes must be an integer: %v", err)
				}
				h.MaxBytes = val
			default:
				return d.Errf("unknown option: %s", d.Val())
			}
		}
	}
	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var handler Handler
	if err := handler.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}
	return &handler, nil
}

type bufferingWriter struct {
	http.ResponseWriter

	prefix          []byte
	status          int
	notFoundMessage []byte
	notFoundStatus  int
	maxBytes        int

	code        int
	wroteHeader bool
	decided     bool
	streamed    bool
	buf         bytes.Buffer
}

func (bw *bufferingWriter) WriteHeader(code int) {
	if bw.wroteHeader {
		return
	}
	bw.wroteHeader = true
	bw.code = code
}

func (bw *bufferingWriter) Write(p []byte) (int, error) {
	if !bw.wroteHeader {
		bw.WriteHeader(http.StatusOK)
	}

	if bw.decided {
		bw.writeHeaderIfNeeded()
		return bw.ResponseWriter.Write(p)
	}

	_, _ = bw.buf.Write(p)

	if bw.buf.Len() >= bw.maxBytes {
		bw.decided = true
		matchesError, matchesNotFound := bw.matchBodyTokens()
		if matchesNotFound {
			bw.code = bw.notFoundStatus
		}
		if matchesError {
			bw.code = bw.status
			if !matchesNotFound {
				bw.ResponseWriter.Header().Set("Cache-Control", "no-store")
			}
		}
		bw.streamed = true
		bw.writeHeaderIfNeeded()
		_, err := bw.ResponseWriter.Write(bw.buf.Bytes())
		bw.buf.Reset()
		return len(p), err
	}

	return len(p), nil
}

func (bw *bufferingWriter) Flush() {
	if !bw.decided && bw.buf.Len() < bw.maxBytes {
		matchesError, matchesNotFound := bw.matchBodyTokens()
		if !matchesError && !matchesNotFound {
			// Suppress streaming flushes so we can inspect the initial bytes.
			return
		}
	}
	if !bw.decided {
		bw.decided = true
		matchesError, matchesNotFound := bw.matchBodyTokens()
		if matchesNotFound {
			bw.code = bw.notFoundStatus
		}
		if matchesError {
			bw.code = bw.status
			if !matchesNotFound {
				bw.ResponseWriter.Header().Set("Cache-Control", "no-store")
			}
		}
		bw.streamed = true
		bw.writeHeaderIfNeeded()
		if bw.buf.Len() > 0 {
			bw.ResponseWriter.Write(bw.buf.Bytes())
			bw.buf.Reset()
		}
	}
	if flusher, ok := bw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (bw *bufferingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	bw.flushIfNeeded()
	hijacker, ok := bw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (bw *bufferingWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := bw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (bw *bufferingWriter) Unwrap() http.ResponseWriter {
	return bw.ResponseWriter
}

func (bw *bufferingWriter) writeHeaderIfNeeded() {
	if bw.code != 0 {
		bw.ResponseWriter.WriteHeader(bw.code)
		bw.code = 0
	}
}

func (bw *bufferingWriter) flushIfNeeded() {
	if bw.decided {
		return
	}

	matchesError, matchesNotFound := bw.matchBodyTokens()
	if matchesNotFound {
		bw.code = bw.notFoundStatus
	}
	if matchesError {
		bw.code = bw.status
		if !matchesNotFound {
			bw.ResponseWriter.Header().Set("Cache-Control", "no-store")
		}
	}

	if !bw.wroteHeader {
		bw.code = http.StatusOK
	}
	if !bw.streamed {
		bw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(bw.buf.Len()))
	}
	bw.writeHeaderIfNeeded()
	if bw.buf.Len() > 0 {
		bw.ResponseWriter.Write(bw.buf.Bytes())
		bw.buf.Reset()
	}
	bw.decided = true
}

func (bw *bufferingWriter) matchBodyTokens() (bool, bool) {
	payload := bw.buf.Bytes()
	if bw.isGzipEncoded() {
		decoded, err := bw.decodeGzip(payload, bw.maxBytes)
		if err == nil && len(decoded) > 0 {
			payload = decoded
		}
	}
	if len(payload) > bw.maxBytes {
		payload = payload[:bw.maxBytes]
	}
	if len(payload) == 0 {
		return false, false
	}
	matchesError := len(bw.prefix) > 0 && bytes.Contains(payload, bw.prefix)
	matchesNotFound := len(bw.notFoundMessage) > 0 && bytes.Contains(payload, bw.notFoundMessage)
	return matchesError, matchesNotFound
}

func (bw *bufferingWriter) isGzipEncoded() bool {
	encoding := bw.ResponseWriter.Header().Get("Content-Encoding")
	return strings.Contains(strings.ToLower(encoding), "gzip")
}

func (bw *bufferingWriter) decodeGzip(data []byte, maxBytes int) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	limited := &io.LimitedReader{R: reader, N: int64(maxBytes)}
	out, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return out, nil
}
