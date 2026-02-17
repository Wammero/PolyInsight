package db

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
)

type UserSettings struct {
	ChatID     int64
	MinAmount  float64
	MaxTrades  int
	MinOdds    float64
	Categories []string // empty = all categories enabled
}

func GetSettings(ctx context.Context, chatID int64) (*UserSettings, error) {
	s := &UserSettings{ChatID: chatID}
	var cats pq.StringArray
	err := DB.QueryRowContext(ctx,
		`SELECT min_amount, max_trades, min_odds, categories
		 FROM user_settings WHERE chat_id=$1`, chatID).
		Scan(&s.MinAmount, &s.MaxTrades, &s.MinOdds, &cats)
	if err == sql.ErrNoRows {
		return &UserSettings{ChatID: chatID, MinAmount: 1000, MaxTrades: 5, MinOdds: 1.15}, nil
	}
	if err != nil {
		return nil, err
	}
	s.Categories = []string(cats)
	return s, nil
}

func UpsertSettings(ctx context.Context, s *UserSettings) error {
	_, err := DB.ExecContext(ctx, `
		INSERT INTO user_settings (chat_id, min_amount, max_trades, min_odds, categories)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chat_id) DO UPDATE
		  SET min_amount=$2, max_trades=$3, min_odds=$4, categories=$5, updated_at=NOW()`,
		s.ChatID, s.MinAmount, s.MaxTrades, s.MinOdds, pq.Array(s.Categories))
	return err
}
