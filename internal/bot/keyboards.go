package bot

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"polymarket_go/internal/db"
	"polymarket_go/internal/polymarket"
)

// MainMenuKeyboard returns the persistent reply keyboard.
func MainMenuKeyboard() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{
				{Text: "⚙️ Настройки"},
				{Text: "👁 Слежка"},
			},
			{
				{Text: "❓ Помощь"},
				{Text: "🔕 Отписаться"},
			},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}
}

// AlertKeyboard builds the inline keyboard shown under each whale alert.
// isWatch indicates the current user is already following this wallet.
func AlertKeyboard(wallet, shortID string, isWatch bool) *telego.InlineKeyboardMarkup {
	watchLabel := "🔔 Следить"
	watchCallback := "watch:add:" + wallet
	if isWatch {
		watchLabel = "👁 Не следить"
		watchCallback = "watch:del:" + wallet
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📊 Статистика кита").WithCallbackData("stats:"+wallet),
			tu.InlineKeyboardButton(watchLabel).WithCallbackData(watchCallback),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Перейти к сделке").WithCallbackData("click:"+shortID),
		),
	)
}

// walletEntry is the compact JSON structure passed to the Mini App.
type walletEntry struct {
	W string `json:"w"`
	L string `json:"l"`
}

// buildMiniAppURL encodes settings and watchlist as query params for the Mini App.
func buildMiniAppURL(base string, s *db.UserSettings, watches []db.Watchlist) string {
	cats := ""
	if len(s.Categories) > 0 {
		if s.Categories[0] == "__none__" {
			cats = "none"
		} else {
			cats = strings.Join(s.Categories, ",")
		}
	}
	q := url.Values{}
	q.Set("amount", fmt.Sprintf("%.0f", s.MinAmount))
	q.Set("trades", fmt.Sprintf("%d", s.MaxTrades))
	q.Set("odds", fmt.Sprintf("%.2f", s.MinOdds))
	q.Set("cats", cats)

	if len(watches) > 0 {
		entries := make([]walletEntry, 0, len(watches))
		for _, w := range watches {
			lbl := w.Label
			if lbl == "" && len(w.Wallet) >= 10 {
				lbl = w.Wallet[:6] + "…" + w.Wallet[len(w.Wallet)-4:]
			}
			entries = append(entries, walletEntry{W: w.Wallet, L: lbl})
		}
		if data, err := json.Marshal(entries); err == nil {
			q.Set("watches", string(data))
		}
	}

	return base + "?" + q.Encode()
}

// SettingsKeyboard is the fallback inline keyboard when Mini App is not configured.
func SettingsKeyboard(s *db.UserSettings) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(
				fmt.Sprintf("💰 Мин. ставка: $%.0f", s.MinAmount),
			).WithCallbackData("set:amount"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(
				fmt.Sprintf("📈 Макс. сделок новичка: %d", s.MaxTrades),
			).WithCallbackData("set:trades"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(
				fmt.Sprintf("📊 Мин. коэффициент: x%.2f", s.MinOdds),
			).WithCallbackData("set:odds"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🏷️ Категории").WithCallbackData("set:cats"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ Закрыть").WithCallbackData("set:close"),
		),
	)
}

// CategoriesKeyboard builds the category selection keyboard.
func CategoriesKeyboard(enabled []string) *telego.InlineKeyboardMarkup {
	enabledSet := make(map[string]bool, len(enabled))
	for _, c := range enabled {
		enabledSet[c] = true
	}
	allEnabled := len(enabled) == 0

	var rows [][]telego.InlineKeyboardButton
	for _, group := range polymarket.CategoryOrder {
		var row []telego.InlineKeyboardButton
		for _, key := range group {
			info := polymarket.AllCategories[key]
			prefix := "☐ "
			if allEnabled || enabledSet[key] {
				prefix = "✅ "
			}
			row = append(row, tu.InlineKeyboardButton(
				prefix+info.Emoji+" "+info.Label,
			).WithCallbackData("cat:"+key))
		}
		rows = append(rows, row)
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("✅ Все").WithCallbackData("cat:all"),
		tu.InlineKeyboardButton("❌ Сбросить").WithCallbackData("cat:none"),
		tu.InlineKeyboardButton("💾 Сохранить").WithCallbackData("cat:save"),
	))
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("🔙 Назад к настройкам").WithCallbackData("set:back"),
	))

	return &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
}
