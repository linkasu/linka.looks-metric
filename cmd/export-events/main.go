package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linkasu/linka.looks-metric/internal/events"
	_ "github.com/mattn/go-sqlite3"
)

type exportEvent struct {
	Source         string `json:"source"`
	SourceRecordID string `json:"source_record_id"`
	SourceSubject  string `json:"source_subject"`
	Product        string `json:"product"`
	OccurredAt     string `json:"occurred_at"`
	Kind           string `json:"kind"`
	AppVersion     string `json:"app_version"`
	Platform       string `json:"platform"`
}

func main() {
	databasePath := flag.String("database", defaultDatabasePath(), "path to the metric SQLite database")
	fromID := flag.Int64("from-id", 0, "export Event rows with id greater than this value")
	flag.Parse()
	if err := export(context.Background(), *databasePath, *fromID, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "export events:", err)
		os.Exit(1)
	}
}

func export(ctx context.Context, databasePath string, fromID int64, output io.Writer) error {
	if strings.TrimSpace(databasePath) == "" || fromID < 0 {
		return fmt.Errorf("database path and non-negative from-id are required")
	}
	database, err := sql.Open("sqlite3", "file:"+databasePath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer database.Close()
	rows, err := database.QueryContext(ctx, `
		SELECT Event.id, Event.type, Event.date, Pc.hash, Pc.version
		FROM Event
		JOIN Pc ON Pc.id = Event.pcId
		WHERE Event.id > ?
		ORDER BY Pc.hash, Event.id`, fromID)
	if err != nil {
		return err
	}
	defer rows.Close()
	encoder := json.NewEncoder(output)
	var count uint64
	for rows.Next() {
		var id, dateMS int64
		var kind, subject, version string
		if err := rows.Scan(&id, &kind, &dateMS, &subject, &version); err != nil {
			return err
		}
		if !events.Allowed(kind) {
			return fmt.Errorf("Event %d has unregistered type", id)
		}
		if _, err := uuid.Parse(subject); err != nil {
			return fmt.Errorf("Event %d has invalid PC hash", id)
		}
		occurredAt := time.UnixMilli(dateMS).UTC()
		if occurredAt.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || !occurredAt.Before(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("Event %d timestamp is outside the supported range", id)
		}
		if version == "" {
			version = "unknown"
		}
		if err := encoder.Encode(exportEvent{
			Source: "looks-sqlite", SourceRecordID: fmt.Sprint(id), SourceSubject: subject, Product: "linka-looks",
			OccurredAt: occurredAt.Format("2006-01-02T15:04:05.000Z07:00"), Kind: kind, AppVersion: version, Platform: "unknown",
		}); err != nil {
			return err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "exported_events=%d\n", count)
	return nil
}

func defaultDatabasePath() string {
	value := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if value == "" {
		value = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if strings.HasPrefix(value, "file:") {
		value = strings.TrimPrefix(value, "file:")
		if query := strings.IndexByte(value, '?'); query >= 0 {
			value = value[:query]
		}
	}
	return value
}
