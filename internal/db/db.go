package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID    int64
	Email string
	PCs   []PC
}

type PC struct {
	ID      int64
	Hash    string
	UserID  int64
	Version string
}

type EventRow struct {
	ID     int64
	Type   string
	DateMs int64
	PCID   int64
	UserID sql.NullInt64
}

type CountByName struct {
	Name  string
	Count int64
}

type DailyActivity struct {
	Day       string
	Events    int64
	ActivePCs int64
}

type Stats struct {
	Users          int64
	PCs            int64
	Events         int64
	Active7Days    int64
	Active30Days   int64
	EventsByType   []CountByName
	Versions       []CountByName
	Daily          []DailyActivity
	RecentUsers    []UserSummary
	RecentEvents   []EventRow
	GeneratedAtUTC time.Time
}

type UserSummary struct {
	ID      int64
	Email   string
	PCCount int64
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	return &Store{db: database}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Migrate() error {
	statements := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		`CREATE TABLE IF NOT EXISTS "User" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "email" TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "User_email_key" ON "User"("email")`,
		`CREATE TABLE IF NOT EXISTS "Pc" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "hash" TEXT NOT NULL, "userId" INTEGER NOT NULL, "version" TEXT NOT NULL, CONSTRAINT "Pc_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User" ("id") ON DELETE RESTRICT ON UPDATE CASCADE)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "Pc_hash_key" ON "Pc"("hash")`,
		`CREATE TABLE IF NOT EXISTS "Event" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "type" TEXT NOT NULL, "content" TEXT, "date" INTEGER NOT NULL, "pcId" INTEGER NOT NULL, "userId" INTEGER, CONSTRAINT "Event_pcId_fkey" FOREIGN KEY ("pcId") REFERENCES "Pc" ("id") ON DELETE RESTRICT ON UPDATE CASCADE, CONSTRAINT "Event_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User" ("id") ON DELETE SET NULL ON UPDATE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS "ActivationMail" ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "code" TEXT NOT NULL, "userId" INTEGER NOT NULL, CONSTRAINT "ActivationMail_userId_fkey" FOREIGN KEY ("userId") REFERENCES "User" ("id") ON DELETE RESTRICT ON UPDATE CASCADE)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "ActivationMail_code_key" ON "ActivationMail"("code")`,
		`CREATE TABLE IF NOT EXISTS "Session" ("id" TEXT NOT NULL PRIMARY KEY, "sid" TEXT NOT NULL, "data" TEXT NOT NULL, "expiresAt" DATETIME NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "Session_sid_key" ON "Session"("sid")`,
		`CREATE INDEX IF NOT EXISTS idx_event_date_pc ON "Event"("date", "pcId")`,
		`CREATE INDEX IF NOT EXISTS idx_event_type ON "Event"("type")`,
		`CREATE INDEX IF NOT EXISTS idx_event_pc_date ON "Event"("pcId", "date")`,
		`CREATE INDEX IF NOT EXISTS idx_event_id_desc ON "Event"("id")`,
		`CREATE INDEX IF NOT EXISTS idx_pc_version ON "Pc"("version")`,
		`CREATE INDEX IF NOT EXISTS idx_pc_user ON "Pc"("userId")`,
		`CREATE INDEX IF NOT EXISTS idx_activation_user_code ON "ActivationMail"("userId", "code")`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) FindOrCreateUser(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `SELECT id, email FROM "User" WHERE email = ?`, email).Scan(&user.ID, &user.Email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return user, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO "User" (email) VALUES (?)`, email)
	if err != nil {
		return user, err
	}
	user.ID, err = res.LastInsertId()
	if err != nil {
		return user, err
	}
	user.Email = email
	return user, nil
}

func (s *Store) SaveActivationCode(ctx context.Context, userID int64, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM "ActivationMail" WHERE "userId" = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "ActivationMail" ("userId", code) VALUES (?, ?)`, userID, code); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Activate(ctx context.Context, email string, code string, hash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM "User" WHERE email = ?`, email).Scan(&userID); err != nil {
		return err
	}
	var mailID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM "ActivationMail" WHERE "userId" = ? AND code = ? LIMIT 1`, userID, code).Scan(&mailID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "Pc" (hash, "userId", version) VALUES (?, ?, '')`, hash, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM "ActivationMail" WHERE id = ?`, mailID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RegisterEvent(ctx context.Context, hash string, eventName string, version string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pc PC
	if err := tx.QueryRowContext(ctx, `SELECT id, hash, "userId", version FROM "Pc" WHERE hash = ?`, hash).Scan(&pc.ID, &pc.Hash, &pc.UserID, &pc.Version); err != nil {
		return err
	}
	if version != "" && pc.Version != version {
		if _, err := tx.ExecContext(ctx, `UPDATE "Pc" SET version = ? WHERE hash = ?`, version, hash); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "Event" (type, content, date, "pcId", "userId") VALUES (?, NULL, ?, ?, ?)`, eventName, time.Now().UnixMilli(), pc.ID, pc.UserID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LoadStats(ctx context.Context) (Stats, error) {
	stats := Stats{GeneratedAtUTC: time.Now().UTC()}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "User"`).Scan(&stats.Users); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Pc"`).Scan(&stats.PCs); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "Event"`).Scan(&stats.Events); err != nil {
		return stats, err
	}
	now := time.Now().UnixMilli()
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT "pcId") FROM "Event" WHERE date >= ? AND date <= ?`, time.Now().AddDate(0, 0, -7).UnixMilli(), now).Scan(&stats.Active7Days); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT "pcId") FROM "Event" WHERE date >= ? AND date <= ?`, time.Now().AddDate(0, 0, -30).UnixMilli(), now).Scan(&stats.Active30Days); err != nil {
		return stats, err
	}

	var err error
	stats.EventsByType, err = s.countByName(ctx, `SELECT type, COUNT(*) FROM "Event" GROUP BY type ORDER BY COUNT(*) DESC`)
	if err != nil {
		return stats, err
	}
	stats.Versions, err = s.countByName(ctx, `SELECT CASE WHEN version = '' THEN '(empty)' ELSE version END, COUNT(*) FROM "Pc" GROUP BY version ORDER BY COUNT(*) DESC`)
	if err != nil {
		return stats, err
	}
	stats.Daily, err = s.dailyActivity(ctx)
	if err != nil {
		return stats, err
	}
	stats.RecentUsers, err = s.recentUsers(ctx)
	if err != nil {
		return stats, err
	}
	stats.RecentEvents, err = s.recentEvents(ctx)
	return stats, err
}

func (s *Store) countByName(ctx context.Context, query string) ([]CountByName, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CountByName
	for rows.Next() {
		var item CountByName
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) dailyActivity(ctx context.Context) ([]DailyActivity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date(date / 1000, 'unixepoch') AS day, COUNT(*), COUNT(DISTINCT "pcId") FROM "Event" WHERE date >= ? GROUP BY day ORDER BY day DESC LIMIT 30`, time.Now().AddDate(0, 0, -30).UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DailyActivity
	for rows.Next() {
		var item DailyActivity
		if err := rows.Scan(&item.Day, &item.Events, &item.ActivePCs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) recentUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.email, COUNT(p.id) FROM "User" u LEFT JOIN "Pc" p ON p."userId" = u.id GROUP BY u.id, u.email ORDER BY u.id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []UserSummary
	for rows.Next() {
		var user UserSummary
		if err := rows.Scan(&user.ID, &user.Email, &user.PCCount); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) recentEvents(ctx context.Context) ([]EventRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, CAST(date AS INTEGER), "pcId", "userId" FROM "Event" ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []EventRow
	for rows.Next() {
		var event EventRow
		if err := rows.Scan(&event.ID, &event.Type, &event.DateMs, &event.PCID, &event.UserID); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (e EventRow) Date() string {
	return time.UnixMilli(e.DateMs).UTC().Format(time.RFC3339)
}

func (u UserSummary) MaskedEmail() string {
	parts := strings.SplitN(u.Email, "@", 2)
	if len(parts) != 2 || len(parts[0]) <= 2 {
		return u.Email
	}
	return parts[0][:2] + "***@" + parts[1]
}
