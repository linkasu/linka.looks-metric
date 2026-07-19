package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestExportReadsOnlySanitizedEventProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metric.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE Pc (id INTEGER PRIMARY KEY, hash TEXT NOT NULL, version TEXT NOT NULL)`,
		`CREATE TABLE Event (id INTEGER PRIMARY KEY, type TEXT NOT NULL, content TEXT, date INTEGER NOT NULL, pcId INTEGER NOT NULL, userId INTEGER)`,
		`INSERT INTO Pc (id, hash, version) VALUES (1, '00000000-0000-4000-8000-000000000000', '3.2.8')`,
		`INSERT INTO Event (id, type, content, date, pcId, userId) VALUES (42, 'cardClick', 'private legacy content', 1784390400000, 1, 7)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	_ = database.Close()
	var output strings.Builder
	if err := export(context.Background(), path, 0, &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, `"source_record_id":"42"`) || !strings.Contains(text, `"kind":"cardClick"`) || strings.Contains(text, "private legacy content") || strings.Contains(text, `"user_id"`) {
		t.Fatalf("unexpected export: %s", text)
	}
	if !strings.Contains(text, time.UnixMilli(1784390400000).UTC().Format("2006-01-02T15:04:05.000Z07:00")) {
		t.Fatal("export timestamp was not normalized")
	}
}

func TestExportRejectsUnknownEventKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metric.db")
	database, _ := sql.Open("sqlite3", path)
	_, _ = database.Exec(`CREATE TABLE Pc (id INTEGER PRIMARY KEY, hash TEXT NOT NULL, version TEXT NOT NULL)`)
	_, _ = database.Exec(`CREATE TABLE Event (id INTEGER PRIMARY KEY, type TEXT NOT NULL, content TEXT, date INTEGER NOT NULL, pcId INTEGER NOT NULL, userId INTEGER)`)
	_, _ = database.Exec(`INSERT INTO Pc VALUES (1, '00000000-0000-4000-8000-000000000000', '3.2.8')`)
	_, _ = database.Exec(`INSERT INTO Event VALUES (1, 'unknown', NULL, 1784390400000, 1, NULL)`)
	_ = database.Close()
	var output strings.Builder
	if err := export(context.Background(), path, 0, &output); err == nil {
		t.Fatal("unknown event type was exported")
	}
}

func TestExportAcceptsSQLiteDatetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metric.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE Pc (id INTEGER PRIMARY KEY, hash TEXT NOT NULL, version TEXT NOT NULL)`,
		`CREATE TABLE Event (id INTEGER PRIMARY KEY, type TEXT NOT NULL, date DATETIME NOT NULL, pcId INTEGER NOT NULL)`,
		`INSERT INTO Pc VALUES (1, '00000000-0000-4000-8000-000000000000', '3.2.8')`,
		`INSERT INTO Event VALUES (42, 'cardClick', '2023-12-15 07:15:06.059+00:00', 1)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	_ = database.Close()
	var output strings.Builder
	if err := export(context.Background(), path, 0, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"occurred_at":"2023-12-15T07:15:06.059Z"`) {
		t.Fatalf("datetime was not normalized: %s", output.String())
	}
}
