package db

import (
	"context"
	"errors"
)

const MaxWatchlistSize = 10

// ErrWatchlistFull is returned when a user tries to add a wallet beyond the 10-wallet limit.
var ErrWatchlistFull = errors.New("watchlist is full (max 10)")

// Watchlist holds a single watchlist entry.
type Watchlist struct {
	ID     int
	ChatID int64
	Wallet string
	Label  string
}

// AddWatch adds wallet to the user's watchlist atomically.
// The COUNT check and INSERT happen in a single statement — eliminates TOCTOU race.
func AddWatch(ctx context.Context, chatID int64, wallet, label string) error {
	res, err := DB.ExecContext(ctx, `
		INSERT INTO watchlist (chat_id, wallet, label)
		SELECT $1, $2, $3
		WHERE (SELECT COUNT(*) FROM watchlist WHERE chat_id = $1) < $4
		ON CONFLICT (chat_id, wallet) DO NOTHING`,
		chatID, wallet, label, MaxWatchlistSize)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// 0 rows affected: either limit reached or duplicate wallet. Determine which.
		var exists int
		if err := DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM watchlist WHERE chat_id=$1 AND wallet=$2",
			chatID, wallet).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return nil // already watching — idempotent, OK
		}
		return ErrWatchlistFull
	}
	return nil
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
		if err := rows.Scan(&w.ID, &w.ChatID, &w.Wallet, &w.Label); err != nil {
			continue
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func IsWatched(ctx context.Context, chatID int64, wallet string) (bool, error) {
	var count int
	err := DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM watchlist WHERE chat_id=$1 AND wallet=$2", chatID, wallet).Scan(&count)
	return count > 0, err
}

func CountWatchlistEntries(ctx context.Context) (int, error) {
	var n int
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM watchlist").Scan(&n)
	return n, err
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
		if err := rows.Scan(&wallet, &chatID); err != nil {
			continue
		}
		m[wallet] = append(m[wallet], chatID)
	}
	return m, rows.Err()
}
