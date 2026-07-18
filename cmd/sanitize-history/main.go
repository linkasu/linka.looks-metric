package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/linkasu/linka.looks-metric/internal/maintenance"
)

func main() {
	databasePath := flag.String("database", defaultDatabasePath(), "path to the metric SQLite database")
	backupPath := flag.String("backup", "", "new SQLite backup path required with --apply")
	apply := flag.Bool("apply", false, "create and verify the backup, then physically remove legacy Event.content")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := maintenance.SanitizeEventHistory(ctx, *databasePath, *backupPath, *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sanitize event history:", err)
		os.Exit(1)
	}
	if !result.Applied {
		fmt.Printf("dry-run: %d Event rows contain legacy content; no data changed\n", result.Found)
		return
	}
	fmt.Printf("backup created and verified at %s; physically removed content from %d Event rows\n", result.BackupPath, result.Removed)
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
