package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/thesarfo/payments-engine/pkg/logctx"
)

func TestRequestLogger_UsesTraceparentAndLogsOutcome(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Get("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	fields := decodeLogLine(t, buf.Bytes())

	if fields["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected trace_id from traceparent, got %v", fields["trace_id"])
	}
	if fields["outcome"] != "success" {
		t.Fatalf("expected success outcome, got %v", fields["outcome"])
	}
	if fields["status"] != float64(http.StatusCreated) {
		t.Fatalf("expected status %d, got %v", http.StatusCreated, fields["status"])
	}
	if fields["level"] != "info" {
		t.Fatalf("expected info level, got %v", fields["level"])
	}
	if _, ok := fields["duration_ms"]; !ok {
		t.Fatal("expected duration_ms field in log")
	}
	if fields["user_agent"] != "test-agent" {
		t.Fatalf("expected user agent test-agent, got %v", fields["user_agent"])
	}
}

func TestRequestLogger_FallsBackToRequestIDAndServerErrorOutcome(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Get("/api/v1/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/fail", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	fields := decodeLogLine(t, buf.Bytes())

	traceID, ok := fields["trace_id"].(string)
	if !ok || traceID == "" || traceID == "-" {
		t.Fatalf("expected non-empty trace_id fallback, got %v", fields["trace_id"])
	}
	if fields["outcome"] != "server_error" {
		t.Fatalf("expected server_error outcome, got %v", fields["outcome"])
	}
	if fields["level"] != "error" {
		t.Fatalf("expected error level, got %v", fields["level"])
	}
}

func TestRequestLogger_UsesWarnAndIncludesErrorFieldsFor4xx(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Get("/api/v1/bad", func(w http.ResponseWriter, r *http.Request) {
		logctx.SetError(r.Context(), "invalid_transfer", "amount must be > 0")
		writeErr := dtoErrorResponse(w, http.StatusBadRequest, "invalid transfer request")
		if writeErr != nil {
			t.Fatalf("write response: %v", writeErr)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bad", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	fields := decodeLogLine(t, buf.Bytes())

	if fields["level"] != "warn" {
		t.Fatalf("expected warn level for 4xx, got %v", fields["level"])
	}
	if fields["error"] != "invalid_transfer" {
		t.Fatalf("expected error code invalid_transfer, got %v", fields["error"])
	}
	if fields["error_detail"] != "amount must be > 0" {
		t.Fatalf("expected error_detail field, got %v", fields["error_detail"])
	}
}

func dtoErrorResponse(w http.ResponseWriter, statusCode int, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(`{"error":"` + message + `"}`))
	return err
}

func decodeLogLine(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("expected at least one log line")
	}

	var fields map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &fields); err != nil {
		t.Fatalf("failed to decode log JSON: %v", err)
	}

	return fields
}
