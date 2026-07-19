package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestActivationFlowDoesNotDependOnTelemetry(t *testing.T) {
	store, handler, mailer, _ := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	if len(hash) != 36 {
		t.Fatalf("expected uuid hash, got %q", hash)
	}
	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Users != 1 || stats.PCs != 1 || stats.Events != 0 {
		t.Fatalf("unexpected stats: users=%d pcs=%d events=%d", stats.Users, stats.PCs, stats.Events)
	}
}

func TestLegacyNestedEventDataIsAcceptedAsNoOp(t *testing.T) {
	store, handler, mailer, databasePath := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	postJSON(t, handler, "/registerEvent", map[string]any{
		"hash":      hash,
		"eventName": "cardClick",
		"eventData": map[string]any{
			"card": map[string]any{
				"title":     "private card text",
				"imagePath": "/private/path/image.png",
			},
			"nested": []any{map[string]string{"message": "private error"}},
		},
	}, http.StatusOK, nil)

	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 0 {
		t.Fatalf("expected no events, got %d", stats.Events)
	}

	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var eventCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM "Event"`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 {
		t.Fatalf("expected legacy request to create no event rows, got %d", eventCount)
	}
	var version string
	if err := database.QueryRow(`SELECT version FROM "Pc" WHERE hash = ?`, hash).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "" {
		t.Fatalf("expected legacy request not to update PC metadata, got version %q", version)
	}
}

func TestCurrentConsentStoresEventWithNullContent(t *testing.T) {
	store, handler, mailer, databasePath := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	postJSON(t, handler, "/registerEvent", map[string]any{
		"hash":      hash,
		"eventName": "start",
		"consent": map[string]any{
			"policy":  currentConsentPolicy,
			"version": currentConsentPolicyVer,
			"granted": true,
		},
	}, http.StatusOK, nil)

	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 1 {
		t.Fatalf("expected one event, got %d", stats.Events)
	}

	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var eventType string
	var content sql.NullString
	if err := database.QueryRow(`SELECT type, content FROM "Event" ORDER BY id DESC LIMIT 1`).Scan(&eventType, &content); err != nil {
		t.Fatal(err)
	}
	if eventType != "start" {
		t.Fatalf("expected start event, got %q", eventType)
	}
	if content.Valid {
		t.Fatalf("expected event content to be NULL, got %q", content.String)
	}
}

func TestCurrentConsentRejectsUnknownEventName(t *testing.T) {
	store, handler, mailer, _ := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	postJSON(t, handler, "/registerEvent", map[string]any{
		"hash":      hash,
		"eventName": "privateText",
		"consent": map[string]any{
			"policy":  currentConsentPolicy,
			"version": currentConsentPolicyVer,
			"granted": true,
		},
	}, http.StatusBadRequest, nil)
	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 0 {
		t.Fatalf("unknown event was stored: %d", stats.Events)
	}
}

func TestOutdatedConsentIsAcceptedAsNoOp(t *testing.T) {
	store, handler, mailer, _ := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	postJSON(t, handler, "/registerEvent", map[string]any{
		"hash":      hash,
		"eventName": "start",
		"consent": map[string]any{
			"policy":  currentConsentPolicy,
			"version": currentConsentPolicyVer - 1,
			"granted": true,
		},
	}, http.StatusOK, nil)

	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 0 {
		t.Fatalf("expected outdated consent to create no events, got %d", stats.Events)
	}
}

func TestRegisterEventRejectsTrailingJSON(t *testing.T) {
	store, handler, mailer, _ := newTestServer(t)
	hash := activateTestPC(t, handler, mailer)
	payload, err := json.Marshal(map[string]any{"hash": hash, "eventName": "start"})
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, []byte(`{"eventData":{"message":"must not be parsed"}}`)...)
	postRawJSON(t, handler, "/registerEvent", payload, http.StatusBadRequest)

	stats, err := store.LoadStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 0 {
		t.Fatalf("expected no events, got %d", stats.Events)
	}
}

func TestStatsPageRenders(t *testing.T) {
	_, handler, _, _ := newTestServer(t)

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

func TestPrivacyPageDescribesOptInAndExcludedData(t *testing.T) {
	_, handler, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/privacy", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	for _, text := range []string{
		"не зависят от согласия",
		"выключена до явного выбора",
		"без актуального маркера согласия",
		"содержимое карточек",
		"названия файлов",
		"тексты ошибок",
		"могла сохранять содержимое",
	} {
		if !bytes.Contains(res.Body.Bytes(), []byte(text)) {
			t.Fatalf("privacy page does not contain %q", text)
		}
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

func postRawJSON(t *testing.T, handler http.Handler, path string, payload []byte, wantStatus int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "looks/3.1.0")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("%s: expected %d, got %d: %s", path, wantStatus, res.Code, res.Body.String())
	}
}

func newTestServer(t *testing.T) (*db.Store, http.Handler, *fakeMailer, string) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "metric.db")
	store, err := db.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	mailer := &fakeMailer{}
	handler := New(testConfig(), store, mailer, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	return store, handler, mailer, databasePath
}

func activateTestPC(t *testing.T, handler http.Handler, mailer *fakeMailer) string {
	t.Helper()
	postJSON(t, handler, "/requestActivation", map[string]string{"email": "User@Example.com"}, http.StatusOK, nil)
	if len(mailer.code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", mailer.code)
	}
	var activation struct {
		Hash string `json:"hash"`
	}
	postJSON(t, handler, "/activate", map[string]string{"email": "user@example.com", "code": mailer.code}, http.StatusOK, &activation)
	return activation.Hash
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
