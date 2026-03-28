package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/thesarfo/payments-engine/pkg/logctx"
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

func RequestLogger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			ctx := logctx.WithRequestLogFields(r.Context())
			r = r.WithContext(ctx)

			requestID := chimiddleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = "-"
			}

			traceID := traceIDFromTraceparent(r.Header.Get("traceparent"))
			if traceID == "" {
				traceID = requestID
			}
			if traceID == "" {
				traceID = "-"
			}
			logctx.SetTraceID(ctx, traceID)

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

			duration := time.Since(start)
			outcome := statusOutcome(rec.status)

			event := logger.Info()
			switch {
			case rec.status >= http.StatusInternalServerError:
				event = logger.Error()
			case rec.status >= http.StatusBadRequest:
				event = logger.Warn()
			}

			event = event.
				Str("event", "http_request").
				Str("trace_id", traceID).
				Str("method", r.Method).
				Str("route", routePattern).
				Str("path", r.URL.Path).
				Int("status", rec.status).
				Int("bytes", rec.bytes).
				Int64("duration_ms", duration.Milliseconds()).
				Str("remote_ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Str("outcome", outcome)

			if rec.status >= http.StatusBadRequest {
				errorCode, errorDetail, ok := logctx.Error(r.Context())
				if ok {
					event = event.Str("error", errorCode).Str("error_detail", errorDetail)
				}
			}

			event.Msg("request completed")
		})
	}
}

func statusOutcome(statusCode int) string {
	switch {
	case statusCode >= http.StatusInternalServerError:
		return "server_error"
	case statusCode >= http.StatusBadRequest:
		return "client_error"
	default:
		return "success"
	}
}

func traceIDFromTraceparent(traceparent string) string {
	traceparent = strings.TrimSpace(traceparent)
	if traceparent == "" {
		return ""
	}

	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}

	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || !isHexString(traceID) || strings.Trim(traceID, "0") == "" {
		return ""
	}

	return traceID
}

func isHexString(value string) bool {
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}
