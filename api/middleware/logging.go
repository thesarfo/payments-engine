package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		routePattern := ""
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			routePattern = rctx.RoutePattern()
		}
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		requestID := chimiddleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = "-"
		}

		duration := time.Since(start)
		log.Printf(
			"http_request req_id=%s method=%s route=%s path=%s status=%d bytes=%d duration_ms=%d remote_ip=%s",
			requestID,
			r.Method,
			routePattern,
			r.URL.Path,
			rec.status,
			rec.bytes,
			duration.Milliseconds(),
			r.RemoteAddr,
		)
	})
}
