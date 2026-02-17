package db

import "context"

type Favorite struct {
	ID     int
	ChatID int64
	Wallet string
	Label  string
}

func AddFavorite(ctx context.Context, chatID int64, wallet, label string) error {
	_, err := DB.ExecContext(ctx,
		`INSERT INTO favorites (chat_id, wallet, label) VALUES ($1, $2, $3)
		 ON CONFLICT (chat_id, wallet) DO NOTHING`,
		chatID, wallet, label)
	return err
}

func RemoveFavorite(ctx context.Context, chatID int64, wallet string) error {
	_, err := DB.ExecContext(ctx,
		"DELETE FROM favorites WHERE chat_id=$1 AND wallet=$2", chatID, wallet)
	return err
}

func GetFavorites(ctx context.Context, chatID int64) ([]Favorite, error) {
	rows, err := DB.QueryContext(ctx,
		"SELECT id, chat_id, wallet, label FROM favorites WHERE chat_id=$1 ORDER BY added_at DESC", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favs []Favorite
	for rows.Next() {
		var f Favorite
		rows.Scan(&f.ID, &f.ChatID, &f.Wallet, &f.Label)
		favs = append(favs, f)
	}
	return favs, nil
}


func IsFavorite(ctx context.Context, chatID int64, wallet string) (bool, error) {
	var count int
	err := DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM favorites WHERE chat_id=$1 AND wallet=$2", chatID, wallet).
		Scan(&count)
	return count > 0, err
}
