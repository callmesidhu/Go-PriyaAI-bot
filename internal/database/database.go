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
type Conversation struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Summary   string `json:"summary"`
	UpdatedAt string `json:"updated_at"`
}
type Message struct {
	ID        int64  `json:"id"`
	Direction string `json:"direction"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

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
func (s *Store) RecordExchange(ctx context.Context, externalID, senderID, username, incoming, reply string) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var found string
	err = tx.QueryRowContext(ctx, "SELECT external_id FROM processed_messages WHERE external_id=?", externalID).Scan(&found)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	n := now()
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(instagram_id,username,created_at) VALUES(?,?,?) ON CONFLICT(instagram_id) DO UPDATE SET username=excluded.username`, senderID, username, n); err != nil {
		return false, err
	}
	var userID, conversationID int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE instagram_id=?", senderID).Scan(&userID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO conversations(user_id,updated_at) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET updated_at=excluded.updated_at`, userID, n); err != nil {
		return false, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT id FROM conversations WHERE user_id=?", userID).Scan(&conversationID); err != nil {
		return false, err
	}
	for _, m := range []struct{ dir, content, status string }{{"in", incoming, "received"}, {"out", reply, "sent"}} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO messages(conversation_id,external_id,direction,content,status,created_at) VALUES(?,?,?,?,?,?)", conversationID, sql.NullString{String: externalID, Valid: m.dir == "in"}, m.dir, m.content, m.status, n); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO processed_messages(external_id,processed_at) VALUES(?,?)", externalID, n); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (s *Store) Conversations(ctx context.Context) ([]Conversation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id,u.username,c.summary,c.updated_at FROM conversations c JOIN users u ON u.id=c.user_id ORDER BY c.updated_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var x Conversation
		if err := rows.Scan(&x.ID, &x.Username, &x.Summary, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) Messages(ctx context.Context, id int64) ([]Message, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,direction,content,status,created_at FROM messages WHERE conversation_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var x Message
		if err := rows.Scan(&x.ID, &x.Direction, &x.Content, &x.Status, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
