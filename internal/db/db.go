package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init(url string) {
	var err error
	DB, err = sql.Open("postgres", url)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	// Tune connection pool to avoid exhausting PostgreSQL's max_connections.
	DB.SetMaxOpenConns(20)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)
	if err = DB.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	migrate()
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}

func migrate() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			chat_id      BIGINT PRIMARY KEY,
			subscribed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_settings (
			chat_id    BIGINT PRIMARY KEY REFERENCES users(chat_id) ON DELETE CASCADE,
			min_amount NUMERIC  DEFAULT 1000,
			max_trades INT      DEFAULT 5,
			min_odds   NUMERIC  DEFAULT 1.15,
			categories TEXT[]   DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS favorites (
			id         SERIAL PRIMARY KEY,
			chat_id    BIGINT NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
			wallet     TEXT   NOT NULL,
			label      TEXT   DEFAULT '',
			added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (chat_id, wallet)
		)`,
		`CREATE TABLE IF NOT EXISTS alert_clicks (
			short_id   TEXT PRIMARY KEY,
			chat_id    BIGINT,
			wallet     TEXT,
			market_slug TEXT,
			market_url  TEXT,
			clicks     INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			id         SERIAL PRIMARY KEY,
			chat_id    BIGINT NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
			wallet     TEXT   NOT NULL,
			label      TEXT   DEFAULT '',
			added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (chat_id, wallet)
		)`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			log.Fatalf("migration failed: %v\nSQL: %s", err, s)
		}
	}
}
