package db

import (
	"context"

	"github.com/lib/pq"
)

// Subscriber holds a user's chat ID and their current filter settings.
type Subscriber struct {
	ChatID     int64
	MinAmount  float64
	MaxTrades  int
	MinOdds    float64
	Categories []string // empty = all categories
}

func Subscribe(ctx context.Context, chatID int64) error {
	_, err := DB.ExecContext(ctx,
		"INSERT INTO users (chat_id) VALUES ($1) ON CONFLICT DO NOTHING", chatID)
	return err
}

func Unsubscribe(ctx context.Context, chatID int64) error {
	_, err := DB.ExecContext(ctx, "DELETE FROM users WHERE chat_id=$1", chatID)
	return err
}

func CountActiveUsers(ctx context.Context) (int, error) {
	var n int
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

// AllSubscribers returns all users with their personal filter settings.
// Users without custom settings get the defaults (1000/$5 trades/1.15 odds/all cats).
func AllSubscribers(ctx context.Context) ([]Subscriber, error) {
	rows, err := DB.QueryContext(ctx, `
		SELECT u.chat_id,
		       COALESCE(s.min_amount, 1000),
		       COALESCE(s.max_trades, 5),
		       COALESCE(s.min_odds,   1.15),
		       COALESCE(s.categories, '{}')
		FROM users u
		LEFT JOIN user_settings s ON s.chat_id = u.chat_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscriber
	for rows.Next() {
		var sub Subscriber
		var cats pq.StringArray
		if err := rows.Scan(&sub.ChatID, &sub.MinAmount, &sub.MaxTrades, &sub.MinOdds, &cats); err != nil {
			continue
		}
		sub.Categories = []string(cats)
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
