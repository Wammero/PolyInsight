package db

import (
	"context"
	"database/sql"
)

func RegisterAlert(ctx context.Context, shortID, wallet, slug, marketURL, txHash, category string, chatID int64) error {
	_, err := DB.ExecContext(ctx,
		`INSERT INTO alert_clicks (short_id, chat_id, wallet, market_slug, market_url, tx_hash, category)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (short_id) DO NOTHING`,
		shortID, chatID, wallet, slug, marketURL, txHash, category)
	return err
}

// LogClickEvent stores a single click event permanently for analytics.
// Category is looked up from alert_clicks so it's always consistent.
func LogClickEvent(ctx context.Context, shortID, country, linkType string) {
	var category string
	DB.QueryRowContext(ctx,
		`SELECT COALESCE(category, '') FROM alert_clicks WHERE short_id=$1`,
		shortID).Scan(&category)
	DB.ExecContext(ctx,
		`INSERT INTO click_events (short_id, country, link_type, category) VALUES ($1, $2, $3, $4)`,
		shortID, country, linkType, category)
}

// GetAlertLinks returns the wallet address and tx hash stored for a given shortID.
// Used by click-tracking callbacks to construct redirect URLs without storing them in callback data.
func GetAlertLinks(ctx context.Context, shortID string) (wallet, txHash string, err error) {
	err = DB.QueryRowContext(ctx,
		`SELECT COALESCE(wallet,''), COALESCE(tx_hash,'') FROM alert_clicks WHERE short_id=$1`,
		shortID).Scan(&wallet, &txHash)
	return
}

// IncrementClick increments the click counter for the given shortID and returns the market URL.
func IncrementClick(ctx context.Context, shortID string) (string, error) {
	var url string
	err := DB.QueryRowContext(ctx,
		`UPDATE alert_clicks SET clicks=clicks+1 WHERE short_id=$1 RETURNING market_url`,
		shortID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return url, err
}

func TotalClicks(ctx context.Context) (int64, error) {
	var total int64
	err := DB.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(clicks), 0) FROM alert_clicks").Scan(&total)
	return total, err
}

func TotalAlerts(ctx context.Context) (int64, error) {
	var total int64
	err := DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM alert_clicks").Scan(&total)
	return total, err
}
