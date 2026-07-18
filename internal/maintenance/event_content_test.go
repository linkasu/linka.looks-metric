package maintenance

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

var legacyPayloads = [][]byte{
	[]byte("LEGACY_PRIVATE_FILENAME_7f23a8c1.linka"),
	[]byte("LEGACY_PRIVATE_CARD_TEXT_41d90e6b"),
}

func TestSanitizeEventHistoryDryRunOnlyCounts(t *testing.T) {
	databasePath := seedLegacyEvents(t)
	result, err := SanitizeEventHistory(context.Background(), databasePath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Found != 2 || result.Removed != 0 || result.Applied {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if got := countContent(t, databasePath); got != 2 {
		t.Fatalf("dry-run changed database content: %d rows remain", got)
	}
	assertPayloadsPresent(t, databasePath, legacyPayloads)
}

func TestSanitizeEventHistoryRequiresNewNonSymlinkBackupPath(t *testing.T) {
	databasePath := seedLegacyEvents(t)
	if _, err := SanitizeEventHistory(context.Background(), databasePath, "", true); err == nil {
		t.Fatal("expected apply without backup to fail")
	}
	if _, err := SanitizeEventHistory(context.Background(), databasePath, databasePath, true); err == nil {
		t.Fatal("expected database path as backup to fail")
	}
	existingBackup := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(existingBackup, []byte("do not replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SanitizeEventHistory(context.Background(), databasePath, existingBackup, true); err == nil {
		t.Fatal("expected existing backup path to be rejected")
	}

	symlinkTarget := filepath.Join(t.TempDir(), "symlink-target.db")
	if err := os.WriteFile(symlinkTarget, []byte("do not follow"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkBackup := filepath.Join(t.TempDir(), "backup-symlink.db")
	if err := os.Symlink(symlinkTarget, symlinkBackup); err != nil {
		t.Fatal(err)
	}
	if _, err := SanitizeEventHistory(context.Background(), databasePath, symlinkBackup, true); err == nil {
		t.Fatal("expected symlink backup path to be rejected")
	}
	targetBytes, err := os.ReadFile(symlinkTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetBytes) != "do not follow" {
		t.Fatalf("symlink target was changed: %q", targetBytes)
	}
	if got := countContent(t, databasePath); got != 2 {
		t.Fatalf("failed sanitize changed database content: %d rows remain", got)
	}
}

func TestSanitizeEventHistoryPhysicallyRemovesPayloadAfterSecureBackup(t *testing.T) {
	databasePath := seedLegacyEvents(t)
	assertPayloadsPresentInFile(t, databasePath, legacyPayloads)
	walKeeper, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer walKeeper.Close()
	walKeeper.SetMaxOpenConns(1)
	if _, err := walKeeper.Exec(`PRAGMA wal_autocheckpoint = 0`); err != nil {
		t.Fatal(err)
	}
	var busy, logFrames, checkpointedFrames int
	if err := walKeeper.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		t.Fatal(err)
	}
	if busy != 0 {
		t.Fatalf("could not prepare WAL fixture: %d log frames, %d checkpointed", logFrames, checkpointedFrames)
	}
	if _, err := walKeeper.Exec(`UPDATE "Event" SET type = type || '-wal-fixture' WHERE content IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	assertPayloadsPresentInFile(t, databasePath+"-wal", legacyPayloads)
	backupPath := filepath.Join(t.TempDir(), "metric-before-sanitize.db")

	result, err := SanitizeEventHistory(context.Background(), databasePath, backupPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Found != 2 || result.Removed != 2 || !result.Applied || result.BackupPath != backupPath {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	backupInfo, err := os.Lstat(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.Mode().IsRegular() {
		t.Fatalf("expected regular backup file, got %s", backupInfo.Mode())
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected private backup permissions, got %o", backupInfo.Mode().Perm())
	}

	assertPayloadsAbsent(t, databasePath, legacyPayloads)
	assertPayloadsPresent(t, backupPath, legacyPayloads)
	if got := countContent(t, databasePath); got != 0 {
		t.Fatalf("expected live content to be cleared, got %d rows", got)
	}
	if got := countContent(t, backupPath); got != 2 {
		t.Fatalf("expected backup to retain two content rows, got %d", got)
	}
}

func seedLegacyEvents(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metric.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.Exec(`
		PRAGMA journal_mode = WAL;
		CREATE TABLE "Event" (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL, content TEXT);
		INSERT INTO "Event" (type, content) VALUES ('openSet', '{"filename":"LEGACY_PRIVATE_FILENAME_7f23a8c1.linka"}');
		INSERT INTO "Event" (type, content) VALUES ('cardClick', '{"title":"LEGACY_PRIVATE_CARD_TEXT_41d90e6b"}');
		INSERT INTO "Event" (type, content) VALUES ('start', NULL);
	`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func countContent(t *testing.T, path string) int64 {
	t.Helper()
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var count int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM "Event" WHERE content IS NOT NULL`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func assertPayloadsPresent(t *testing.T, databasePath string, payloads [][]byte) {
	t.Helper()
	data := databaseAndWALBytes(t, databasePath)
	for _, payload := range payloads {
		if !bytes.Contains(data, payload) {
			t.Fatalf("expected %q in database or WAL bytes", payload)
		}
	}
}

func assertPayloadsPresentInFile(t *testing.T, path string, payloads [][]byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range payloads {
		if !bytes.Contains(data, payload) {
			t.Fatalf("expected %q in %s", payload, path)
		}
	}
}

func assertPayloadsAbsent(t *testing.T, databasePath string, payloads [][]byte) {
	t.Helper()
	data := databaseAndWALBytes(t, databasePath)
	for _, payload := range payloads {
		if bytes.Contains(data, payload) {
			t.Fatalf("found legacy payload %q in database or WAL bytes after sanitizing", payload)
		}
	}
}

func databaseAndWALBytes(t *testing.T, databasePath string) []byte {
	t.Helper()
	var result []byte
	for _, path := range []string{databasePath, databasePath + "-wal"} {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		result = append(result, data...)
	}
	return result
}
