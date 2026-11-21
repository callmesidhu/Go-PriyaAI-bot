package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON",
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, instagram_id TEXT NOT NULL UNIQUE, username TEXT NOT NULL, preferred_name TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS conversations (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL UNIQUE REFERENCES users(id), summary TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS messages (id INTEGER PRIMARY KEY, conversation_id INTEGER NOT NULL REFERENCES conversations(id), external_id TEXT, direction TEXT NOT NULL CHECK(direction IN ('in','out')), content TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'sent', created_at TEXT NOT NULL, UNIQUE(conversation_id, external_id))`,
		`CREATE TABLE IF NOT EXISTS user_memories (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), category TEXT NOT NULL, value TEXT NOT NULL, created_at TEXT NOT NULL, UNIQUE(user_id, category, value))`,
		`CREATE TABLE IF NOT EXISTS processed_messages (external_id TEXT PRIMARY KEY, processed_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS bot_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	return nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339) }
func (s *Store) IsProcessed(ctx context.Context, externalID string) (bool, error) {
	var one int
	err := s.DB.QueryRowContext(ctx, "SELECT 1 FROM processed_messages WHERE external_id=?", externalID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
func (s *Store) MarkProcessed(ctx context.Context, externalID string) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO processed_messages(external_id, processed_at) VALUES (?,?)", externalID, now())
	return err
}
func (s *Store) Status(ctx context.Context) (string, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, "SELECT value FROM bot_settings WHERE key='automation_state'").Scan(&v)
	if err == sql.ErrNoRows {
		return "paused", nil
	}
	return v, err
}
func (s *Store) SetStatus(ctx context.Context, status string) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO bot_settings(key,value) VALUES('automation_state',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", status)
	return err
}
