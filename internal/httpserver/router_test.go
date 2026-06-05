package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/linkasu/linka.looks-metric/internal/config"
	"github.com/linkasu/linka.looks-metric/internal/db"
)

type fakeMailer struct {
	code string
}

func (f *fakeMailer) SendActivationCode(_ context.Context, _ string, code string) error {
	f.code = code
	return nil
}

func TestActivationAndRegisterEventFlow(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	mailer := &fakeMailer{}
	handler := New(testConfig(), store, mailer, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	postJSON(t, handler, "/requestActivation", map[string]string{"email": "User@Example.com"}, http.StatusOK, nil)
	if len(mailer.code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", mailer.code)
	}

	var activation struct {
		Hash string `json:"hash"`
	}
	postJSON(t, handler, "/activate", map[string]string{"email": "user@example.com", "code": mailer.code}, http.StatusOK, &activation)
	if len(activation.Hash) != 36 {
		t.Fatalf("expected uuid hash, got %q", activation.Hash)
	}

	postJSON(t, handler, "/registerEvent", map[string]any{"hash": activation.Hash, "eventName": "start", "eventData": map[string]string{"ok": "yes"}}, http.StatusOK, nil)

	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Users != 1 || stats.PCs != 1 || stats.Events != 1 {
		t.Fatalf("unexpected stats: users=%d pcs=%d events=%d", stats.Users, stats.PCs, stats.Events)
	}
}

func TestStatsPageRenders(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	handler := New(testConfig(), store, &fakeMailer{}, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !bytes.Contains(res.Body.Bytes(), []byte("Метрики")) {
		t.Fatalf("stats page did not render expected heading")
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, body any, wantStatus int, response any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "looks/3.1.0")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("%s: expected %d, got %d: %s", path, wantStatus, res.Code, res.Body.String())
	}
	if response != nil {
		if err := json.NewDecoder(res.Body).Decode(response); err != nil {
			t.Fatal(err)
		}
	}
}

func testConfig() config.Config {
	return config.Config{
		AppHost:       "127.0.0.1",
		AppPort:       "30812",
		DatabasePath:  ":memory:",
		PublicBaseURL: "https://metric.linka.su",
		Mail:          config.MailConfig{DryRun: true},
	}
}
