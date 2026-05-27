package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/christopherime/raidforge/internal/config"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
}

func TestHealthz(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "ok\n" {
		t.Fatalf("body = %q, want %q", got, "ok\n")
	}
}

func TestRecovererTurnsPanicInto500(t *testing.T) {
	srv := testServer(t)
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })

	rec := httptest.NewRecorder()
	srv.recoverer(panicking).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
