package db

import (
	"context"
	"errors"
)

const MaxWatchlistSize = 10

// ErrWatchlistFull is returned when a user tries to add a 6th wallet to follow.
var ErrWatchlistFull = errors.New("watchlist is full (max 5)")

// Watchlist holds a single watchlist entry.
type Watchlist struct {
	ID     int
	ChatID int64
	Wallet string
	Label  string
}

func AddWatch(ctx context.Context, chatID int64, wallet, label string) error {
	var count int
	if err := DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM watchlist WHERE chat_id=$1", chatID).Scan(&count); err != nil {
		return err
	}
	if count >= MaxWatchlistSize {
		return ErrWatchlistFull
	}
	_, err := DB.ExecContext(ctx,
		`INSERT INTO watchlist (chat_id, wallet, label) VALUES ($1, $2, $3)
		 ON CONFLICT (chat_id, wallet) DO NOTHING`,
		chatID, wallet, label)
	return err
}

func RemoveWatch(ctx context.Context, chatID int64, wallet string) error {
	_, err := DB.ExecContext(ctx,
		"DELETE FROM watchlist WHERE chat_id=$1 AND wallet=$2", chatID, wallet)
	return err
}

func GetWatchlist(ctx context.Context, chatID int64) ([]Watchlist, error) {
	rows, err := DB.QueryContext(ctx,
		"SELECT id, chat_id, wallet, label FROM watchlist WHERE chat_id=$1 ORDER BY added_at DESC", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Watchlist
	for rows.Next() {
		var w Watchlist
		rows.Scan(&w.ID, &w.ChatID, &w.Wallet, &w.Label)
		list = append(list, w)
	}
	return list, nil
}

func IsWatched(ctx context.Context, chatID int64, wallet string) (bool, error) {
	var count int
	err := DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM watchlist WHERE chat_id=$1 AND wallet=$2", chatID, wallet).Scan(&count)
	return count > 0, err
}

// GetAllWatchedWallets returns a map of wallet → []chatID for all watchlist entries.
func GetAllWatchedWallets(ctx context.Context) (map[string][]int64, error) {
	rows, err := DB.QueryContext(ctx, "SELECT wallet, chat_id FROM watchlist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string][]int64)
	for rows.Next() {
		var wallet string
		var chatID int64
		rows.Scan(&wallet, &chatID)
		m[wallet] = append(m[wallet], chatID)
	}
	return m, nil
}
