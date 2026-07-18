package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mattn/go-sqlite3"
)

type EventHistorySanitizeResult struct {
	Found      int64
	Removed    int64
	Applied    bool
	BackupPath string
}

func SanitizeEventHistory(ctx context.Context, databasePath string, backupPath string, apply bool) (EventHistorySanitizeResult, error) {
	result := EventHistorySanitizeResult{}
	databasePath, err := existingDatabasePath(databasePath)
	if err != nil {
		return result, err
	}

	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return result, err
	}
	defer database.Close()
	database.SetMaxOpenConns(2)
	database.SetMaxIdleConns(2)

	connection, err := database.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer connection.Close()

	if !apply {
		if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Event" WHERE content IS NOT NULL`).Scan(&result.Found); err != nil {
			return result, fmt.Errorf("count event content: %w", err)
		}
		return result, nil
	}

	backupPath, err = availableBackupPath(databasePath, backupPath)
	if err != nil {
		return result, err
	}
	backupFile, backupInfo, err := createSecureBackupFile(backupPath)
	if err != nil {
		return result, err
	}
	removeIncompleteBackup := true
	defer func() {
		_ = backupFile.Close()
		if removeIncompleteBackup {
			removeBackupIfUnchanged(backupPath, backupInfo)
		}
	}()

	if _, err := connection.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		return result, fmt.Errorf("enable secure delete: %w", err)
	}
	if _, err := connection.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return result, fmt.Errorf("lock database for sanitizing: %w", err)
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			_, _ = connection.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Event" WHERE content IS NOT NULL`).Scan(&result.Found); err != nil {
		return result, fmt.Errorf("count event content: %w", err)
	}
	sourceConnection, err := database.Conn(ctx)
	if err != nil {
		return result, fmt.Errorf("connect sqlite source for backup: %w", err)
	}
	copyErr := copySQLiteBackup(ctx, sourceConnection, backupPath, backupInfo)
	closeErr := sourceConnection.Close()
	if copyErr != nil {
		return result, copyErr
	}
	if closeErr != nil {
		return result, fmt.Errorf("close sqlite source after backup: %w", closeErr)
	}
	if err := backupFile.Sync(); err != nil {
		return result, fmt.Errorf("sync sqlite backup: %w", err)
	}
	if err := verifyBackup(ctx, backupPath, backupInfo, result.Found); err != nil {
		return result, err
	}
	if err := backupFile.Close(); err != nil {
		return result, fmt.Errorf("close sqlite backup: %w", err)
	}
	removeIncompleteBackup = false
	result.BackupPath = backupPath

	updated, err := connection.ExecContext(ctx, `UPDATE "Event" SET content = NULL WHERE content IS NOT NULL`)
	if err != nil {
		return result, fmt.Errorf("clear event content: %w", err)
	}
	result.Removed, err = updated.RowsAffected()
	if err != nil {
		return result, fmt.Errorf("read sanitize count: %w", err)
	}
	if result.Removed != result.Found {
		return result, fmt.Errorf("sanitize count differs from backup: found %d, removed %d", result.Found, result.Removed)
	}
	if _, err := connection.ExecContext(ctx, `COMMIT`); err != nil {
		return result, fmt.Errorf("commit event content removal: %w", err)
	}
	transactionOpen = false

	if err := checkpointWAL(ctx, connection); err != nil {
		return result, err
	}
	if _, err := connection.ExecContext(ctx, `VACUUM`); err != nil {
		return result, fmt.Errorf("vacuum sanitized database: %w", err)
	}
	if err := checkpointWAL(ctx, connection); err != nil {
		return result, err
	}
	var remaining int64
	if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Event" WHERE content IS NOT NULL`).Scan(&remaining); err != nil {
		return result, fmt.Errorf("verify sanitized event content: %w", err)
	}
	if remaining != 0 {
		return result, fmt.Errorf("event content remains after sanitizing: %d rows", remaining)
	}

	result.Applied = true
	return result, nil
}

func existingDatabasePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("database path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("database path must be a regular file")
	}
	return absolute, nil
}

func availableBackupPath(databasePath string, backupPath string) (string, error) {
	if backupPath == "" {
		return "", errors.New("backup path is required with --apply")
	}
	absolute, err := filepath.Abs(backupPath)
	if err != nil {
		return "", err
	}
	if absolute == databasePath {
		return "", errors.New("backup path must differ from database path")
	}
	directoryInfo, err := os.Stat(filepath.Dir(absolute))
	if err != nil {
		return "", fmt.Errorf("stat backup directory: %w", err)
	}
	if !directoryInfo.IsDir() {
		return "", errors.New("backup parent must be a directory")
	}
	if _, err := os.Lstat(absolute); err == nil {
		return "", errors.New("backup path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat backup path: %w", err)
	}
	return absolute, nil
}

func createSecureBackupFile(path string) (*os.File, os.FileInfo, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("create sqlite backup: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("protect sqlite backup: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("stat sqlite backup: %w", err)
	}
	if err := verifySecureBackupPath(path, info); err != nil {
		file.Close()
		removeBackupIfUnchanged(path, info)
		return nil, nil, err
	}
	return file, info, nil
}

func copySQLiteBackup(ctx context.Context, source *sql.Conn, backupPath string, backupInfo os.FileInfo) error {
	destinationDatabase, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		return fmt.Errorf("open sqlite backup: %w", err)
	}
	defer destinationDatabase.Close()
	destinationDatabase.SetMaxOpenConns(1)
	destinationDatabase.SetMaxIdleConns(1)
	destination, err := destinationDatabase.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect sqlite backup: %w", err)
	}
	defer destination.Close()
	if err := verifySecureBackupPath(backupPath, backupInfo); err != nil {
		return err
	}

	err = destination.Raw(func(destinationDriver any) error {
		destinationSQLite, ok := destinationDriver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("unexpected sqlite backup connection type")
		}
		return source.Raw(func(sourceDriver any) error {
			sourceSQLite, ok := sourceDriver.(*sqlite3.SQLiteConn)
			if !ok {
				return errors.New("unexpected sqlite source connection type")
			}
			backup, err := destinationSQLite.Backup("main", sourceSQLite, "main")
			if err != nil {
				return err
			}
			for {
				done, stepErr := backup.Step(-1)
				if stepErr != nil {
					_ = backup.Finish()
					return stepErr
				}
				if done {
					return backup.Finish()
				}
				select {
				case <-ctx.Done():
					_ = backup.Finish()
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		})
	})
	if err != nil {
		return fmt.Errorf("copy sqlite backup: %w", err)
	}
	if err := verifySecureBackupPath(backupPath, backupInfo); err != nil {
		return err
	}
	return nil
}

func verifyBackup(ctx context.Context, backupPath string, backupInfo os.FileInfo, expectedCount int64) error {
	if err := verifySecureBackupPath(backupPath, backupInfo); err != nil {
		return err
	}
	backup, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		return fmt.Errorf("open backup for verification: %w", err)
	}
	defer backup.Close()
	backup.SetMaxOpenConns(1)
	backup.SetMaxIdleConns(1)
	connection, err := backup.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect backup for verification: %w", err)
	}
	defer connection.Close()
	if err := verifySecureBackupPath(backupPath, backupInfo); err != nil {
		return err
	}
	var integrity string
	if err := connection.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("verify backup integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("backup integrity check returned %q", integrity)
	}
	var contentCount int64
	if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Event" WHERE content IS NOT NULL`).Scan(&contentCount); err != nil {
		return fmt.Errorf("verify backup event content: %w", err)
	}
	if contentCount != expectedCount {
		return fmt.Errorf("backup content count mismatch: expected %d, got %d", expectedCount, contentCount)
	}
	return verifySecureBackupPath(backupPath, backupInfo)
}

func verifySecureBackupPath(path string, expected os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat sqlite backup path: %w", err)
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() {
		return errors.New("backup path must remain a regular file, not a symlink")
	}
	if !os.SameFile(expected, current) {
		return errors.New("backup path changed while sanitizing")
	}
	if current.Mode().Perm() != 0o600 {
		return fmt.Errorf("backup permissions changed to %o", current.Mode().Perm())
	}
	return nil
}

func removeBackupIfUnchanged(path string, expected os.FileInfo) {
	current, err := os.Lstat(path)
	if err == nil && current.Mode()&os.ModeSymlink == 0 && os.SameFile(expected, current) {
		_ = os.Remove(path)
	}
}

func checkpointWAL(ctx context.Context, connection *sql.Conn) error {
	var busy, logFrames, checkpointedFrames int
	if err := connection.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return fmt.Errorf("truncate sqlite WAL: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("truncate sqlite WAL: database is busy (%d log frames, %d checkpointed)", logFrames, checkpointedFrames)
	}
	return nil
}
