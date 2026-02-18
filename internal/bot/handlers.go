package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"polymarket_go/internal/db"
	"polymarket_go/internal/metrics"
	"polymarket_go/internal/polymarket"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// StartHandlers begins long-polling and routes all incoming updates.
func StartHandlers(b *telego.Bot) {
	updates, _ := b.UpdatesViaLongPolling(context.Background(), nil)
	for u := range updates {
		switch {
		case u.Message != nil && u.Message.WebAppData != nil:
			go handleWebAppData(b, u.Message)
		case u.Message != nil:
			go handleMessage(b, u.Message)
		case u.CallbackQuery != nil:
			go handleCallback(b, u.CallbackQuery)
		}
	}
}

// HandleRedirect is an HTTP handler for tracked redirect links (/r/{shortID}).
func HandleRedirect(w http.ResponseWriter, r *http.Request) {
	shortID := strings.TrimPrefix(r.URL.Path, "/r/")
	if shortID == "" {
		http.NotFound(w, r)
		return
	}
	marketURL, err := db.IncrementClick(context.Background(), shortID)
	if err != nil || marketURL == "" {
		http.NotFound(w, r)
		return
	}
	metrics.ClicksTotal.Inc()
	http.Redirect(w, r, marketURL, http.StatusFound)
}

// ─── Message handler ───────────────────────────────────────────────────────

func handleMessage(b *telego.Bot, msg *telego.Message) {
	ctx := context.Background()
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if state, ok := getSession(chatID); ok {
		handleSettingsInput(b, ctx, chatID, state, text)
		return
	}

	switch text {
	case "/start":
		db.Subscribe(ctx, chatID)
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"✅ <b>Подписка активна!</b>\n\n"+
				"Бот будет присылать алерты о новых крупных игроках на Polymarket.\n\n"+
				"Используйте кнопки внизу для навигации.",
		).WithParseMode(telego.ModeHTML).WithReplyMarkup(MainMenuKeyboard()))

	case "/stop", "🔕 Отписаться":
		db.Unsubscribe(ctx, chatID)
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Подписка отменена. Напишите /start чтобы снова подписаться."))

	case "/settings", "⚙️ Настройки":
		handleSettingsCommand(b, ctx, chatID)

	case "⬅️ Главное меню":
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "🏠 Главное меню").
			WithReplyMarkup(MainMenuKeyboard()))

	case "/watchlist", "👁 Слежка":
		handleWatchlistCommand(b, ctx, chatID)

	case "/help", "❓ Помощь":
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"ℹ️ <b>Справка</b>\n\n"+
				"Бот отслеживает крупные сделки новичков на <b>Polymarket</b>.\n\n"+
				"📌 <b>Как пользоваться:</b>\n"+
				"• Когда кит-новичок делает ставку — бот присылает алерт\n"+
				"• Нажмите <b>📊 Статистика кита</b> — увидите историю игрока\n"+
				"• Нажмите <b>🔔 Следить</b> — получать уведомления о каждой сделке этого кита (макс. 10)\n"+
				"• Нажмите <b>🔗 Перейти к сделке</b> — откроет рынок на Polymarket\n\n"+
				"⚙️ <b>Настройки (кнопка ниже):</b>\n"+
				"• Мин. сумма ставки — скрыть маленькие ставки\n"+
				"• Макс. сделок новичка — насколько «новичок» должен быть новым\n"+
				"• Мин. коэффициент — фильтр по шансам\n"+
				"• Категории — только интересные вам разделы\n\n"+
				"👁 <b>Следить</b> — реал-тайм уведомления без фильтров (макс. 10 кошельков)",
		).WithParseMode(telego.ModeHTML))
	}
}

func handleSettingsCommand(b *telego.Bot, ctx context.Context, chatID int64) {
	if oldID := getSettingsMsgID(chatID); oldID != 0 {
		b.DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(chatID),
			MessageID: oldID,
		})
		clearSettingsMsgID(chatID)
	}

	s, err := db.GetSettings(ctx, chatID)
	if err != nil || s == nil {
		s = &db.UserSettings{ChatID: chatID, MinAmount: 1000, MaxTrades: 5, MinOdds: 1.15}
	}

	if base := GetMiniAppURL(); base != "" {
		watches, _ := db.GetWatchlist(ctx, chatID)
		appURL := buildMiniAppURL(base, s, watches)
		kb := &telego.ReplyKeyboardMarkup{
			Keyboard: [][]telego.KeyboardButton{
				{{
					Text:   "🌐 Открыть настройки",
					WebApp: &telego.WebAppInfo{URL: appURL},
				}},
				{{Text: "⬅️ Главное меню"}},
			},
			ResizeKeyboard:  true,
			OneTimeKeyboard: true,
		}
		catsLabel := "все"
		if len(s.Categories) > 0 && s.Categories[0] != "__none__" {
			catsLabel = fmt.Sprintf("%d выбрано", len(s.Categories))
		} else if len(s.Categories) > 0 && s.Categories[0] == "__none__" {
			catsLabel = "нет"
		}
		text := fmt.Sprintf(
			"⚙️ <b>Текущие настройки</b>\n\n"+
				"💰 Мин. ставка: <b>$%.0f</b>\n"+
				"📈 Макс. сделок: <b>%d</b>\n"+
				"📊 Мин. коэф.: <b>x%.2f</b>\n"+
				"🏷️ Категории: <b>%s</b>\n"+
				"👁 Следить: <b>%d/10</b>\n\n"+
				"Нажмите кнопку ниже чтобы открыть настройки.",
			s.MinAmount, s.MaxTrades, s.MinOdds, catsLabel, len(watches),
		)
		sent, err := b.SendMessage(ctx, tu.Message(tu.ID(chatID), text).
			WithParseMode(telego.ModeHTML).WithReplyMarkup(kb))
		if err == nil && sent != nil {
			setSettingsMsgID(chatID, sent.MessageID)
		}
		return
	}

	// Fallback: inline keyboard.
	sent, err := b.SendMessage(ctx, tu.Message(tu.ID(chatID),
		"⚙️ <b>Ваши настройки</b>\n\nНажмите на параметр чтобы изменить его.",
	).WithParseMode(telego.ModeHTML).WithReplyMarkup(SettingsKeyboard(s)))
	if err == nil && sent != nil {
		setSettingsMsgID(chatID, sent.MessageID)
	}
}

func handleWatchlistCommand(b *telego.Bot, ctx context.Context, chatID int64) {
	watches, err := db.GetWatchlist(ctx, chatID)
	if err != nil || len(watches) == 0 {
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"👁 <b>Список слежки пуст</b>\n\nНажмите «🔔 Следить» в алерте, чтобы отслеживать кита в реальном времени (макс. 10).",
		).WithParseMode(telego.ModeHTML))
		return
	}
	lines := fmt.Sprintf("👁 <b>Слежка (%d/10)</b>\n\n", len(watches))
	for i, w := range watches {
		label := w.Label
		if label == "" && len(w.Wallet) >= 10 {
			label = w.Wallet[:6] + "..." + w.Wallet[len(w.Wallet)-4:]
		}
		lines += fmt.Sprintf("%d. <a href='https://polymarket.com/profile/%s'>%s</a>\n",
			i+1, w.Wallet, label)
	}
	lines += "\nУправляйте списком через ⚙️ Настройки."
	b.SendMessage(ctx, tu.Message(tu.ID(chatID), lines).
		WithParseMode(telego.ModeHTML).
		WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true}))
}

// handleSettingsInput processes a text reply when waiting for a settings value.
func handleSettingsInput(b *telego.Bot, ctx context.Context, chatID int64, state, text string) {
	clearSession(chatID)
	s, _ := db.GetSettings(ctx, chatID)
	if s == nil {
		s = &db.UserSettings{ChatID: chatID, MinAmount: 1000, MaxTrades: 5, MinOdds: 1.15}
	}

	var errMsg string
	switch state {
	case "set:amount":
		val, err := strconv.ParseFloat(strings.ReplaceAll(text, "$", ""), 64)
		if err != nil || val < 10 {
			errMsg = "❌ Введите число от 10 (например: <code>500</code>)"
		} else {
			s.MinAmount = val
		}
	case "set:trades":
		val, err := strconv.Atoi(text)
		if err != nil || val < 1 || val > 1000 {
			errMsg = "❌ Введите целое число от 1 до 1000"
		} else {
			s.MaxTrades = val
		}
	case "set:odds":
		val, err := strconv.ParseFloat(strings.ReplaceAll(text, "x", ""), 64)
		if err != nil || val < 1.0 {
			errMsg = "❌ Введите коэффициент ≥ 1.0 (например: <code>1.5</code>)"
		} else {
			s.MinOdds = val
		}
	}

	if errMsg != "" {
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), errMsg).WithParseMode(telego.ModeHTML))
		return
	}

	db.UpsertSettings(ctx, s)
	b.SendMessage(ctx, tu.Message(tu.ID(chatID),
		"✅ <b>Настройки сохранены!</b>",
	).WithParseMode(telego.ModeHTML).WithReplyMarkup(SettingsKeyboard(s)))
}

// ─── Callback handler ──────────────────────────────────────────────────────

func handleCallback(b *telego.Bot, cb *telego.CallbackQuery) {
	ctx := context.Background()
	chatID := cb.From.ID
	data := cb.Data

	defer b.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})

	switch {
	// ── Settings ────────────────────────────────────────────────────────
	case data == "set:close":
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.DeleteMessage(ctx, &telego.DeleteMessageParams{
				ChatID:    tu.ID(chatID),
				MessageID: msg.MessageID,
			})
			clearSettingsMsgID(chatID)
		}

	case data == "set:back":
		clearPendingCats(chatID)
		s, _ := db.GetSettings(ctx, chatID)
		if s == nil {
			s = &db.UserSettings{ChatID: chatID, MinAmount: 1000, MaxTrades: 5, MinOdds: 1.15}
		}
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.EditMessageText(ctx, &telego.EditMessageTextParams{
				ChatID:      tu.ID(chatID),
				MessageID:   msg.MessageID,
				Text:        "⚙️ <b>Ваши настройки</b>\n\nНажмите на параметр чтобы изменить его.",
				ParseMode:   telego.ModeHTML,
				ReplyMarkup: SettingsKeyboard(s),
			})
		} else {
			handleSettingsCommand(b, ctx, chatID)
		}

	case data == "set:amount":
		setSession(chatID, "set:amount")
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"💰 Введите минимальную сумму ставки в USD:\n<i>Например: <code>500</code></i>",
		).WithParseMode(telego.ModeHTML))

	case data == "set:trades":
		setSession(chatID, "set:trades")
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"📈 Введите максимальное кол-во сделок у новичка:\n<i>Например: <code>10</code></i>",
		).WithParseMode(telego.ModeHTML))

	case data == "set:odds":
		setSession(chatID, "set:odds")
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			"📊 Введите минимальный коэффициент:\n<i>Например: <code>1.5</code></i>",
		).WithParseMode(telego.ModeHTML))

	case data == "set:cats":
		s, _ := db.GetSettings(ctx, chatID)
		if s == nil {
			s = &db.UserSettings{ChatID: chatID}
		}
		initPendingCats(chatID, s.Categories)
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.EditMessageText(ctx, &telego.EditMessageTextParams{
				ChatID:      tu.ID(chatID),
				MessageID:   msg.MessageID,
				Text:        "🏷️ <b>Выберите категории</b>\n\n✅ = включена  ☐ = выключена\n<i>Пустой список = все категории</i>",
				ParseMode:   telego.ModeHTML,
				ReplyMarkup: CategoriesKeyboard(s.Categories),
			})
		} else {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID),
				"🏷️ <b>Выберите категории</b>\n\n✅ = включена  ☐ = выключена",
			).WithParseMode(telego.ModeHTML).WithReplyMarkup(CategoriesKeyboard(s.Categories)))
		}

	// ── Category toggles ─────────────────────────────────────────────────
	case strings.HasPrefix(data, "cat:"):
		handleCategoryCallback(b, ctx, chatID, cb, data)

	// ── Click tracking ───────────────────────────────────────────────────
	case strings.HasPrefix(data, "click:"):
		shortID := strings.TrimPrefix(data, "click:")
		marketURL, err := db.IncrementClick(ctx, shortID)
		if err != nil || marketURL == "" {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Ссылка не найдена."))
			return
		}
		metrics.ClicksTotal.Inc()
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			fmt.Sprintf("🔗 <a href='%s'>Открыть сделку на Polymarket ↗</a>", marketURL),
		).WithParseMode(telego.ModeHTML).
			WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true}))

	// ── Wallet stats ─────────────────────────────────────────────────────
	case strings.HasPrefix(data, "stats:"):
		wallet := strings.TrimPrefix(data, "stats:")
		stats := polymarket.GetWalletStats(ctx, httpClient, wallet)
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), FormatWalletStats(wallet, stats)).
			WithParseMode(telego.ModeHTML).
			WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true}))

	// ── Watchlist ────────────────────────────────────────────────────────
	case strings.HasPrefix(data, "watch:add:"):
		wallet := strings.TrimPrefix(data, "watch:add:")
		err := db.AddWatch(ctx, chatID, wallet, "")
		if err == db.ErrWatchlistFull {
			b.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cb.ID,
				Text:            "❌ Лимит: можно следить максимум за 10 кошельками",
				ShowAlert:       true,
			})
			return
		}
		b.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "🔔 Теперь вы следите за этим кошельком!",
			ShowAlert:       true,
		})
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
				ChatID:      tu.ID(chatID),
				MessageID:   msg.MessageID,
				ReplyMarkup: AlertKeyboard(wallet, extractShortID(msg), true),
			})
		}

	case strings.HasPrefix(data, "watch:del:"):
		wallet := strings.TrimPrefix(data, "watch:del:")
		db.RemoveWatch(ctx, chatID, wallet)
		b.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "👁 Вы перестали следить за этим кошельком",
			ShowAlert:       true,
		})
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
				ChatID:      tu.ID(chatID),
				MessageID:   msg.MessageID,
				ReplyMarkup: AlertKeyboard(wallet, extractShortID(msg), false),
			})
		}
	}
}

// extractShortID scans a message's inline keyboard for a "click:" callback and returns the shortID.
func extractShortID(msg *telego.Message) string {
	if msg.ReplyMarkup == nil {
		return ""
	}
	for _, row := range msg.ReplyMarkup.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, "click:") {
				return strings.TrimPrefix(btn.CallbackData, "click:")
			}
		}
	}
	return ""
}

// handleCategoryCallback manages category toggle state in-memory (no DB until Save).
func handleCategoryCallback(b *telego.Bot, ctx context.Context, chatID int64, cb *telego.CallbackQuery, data string) {
	key := strings.TrimPrefix(data, "cat:")

	cats, ok := getPendingCats(chatID)
	if !ok {
		s, _ := db.GetSettings(ctx, chatID)
		if s == nil {
			s = &db.UserSettings{ChatID: chatID}
		}
		cats = s.Categories
		initPendingCats(chatID, cats)
	}

	switch key {
	case "all":
		setPendingCats(chatID, []string{})
		cats = []string{}

	case "none":
		setPendingCats(chatID, []string{"__none__"})
		cats = []string{"__none__"}

	case "save":
		s, _ := db.GetSettings(ctx, chatID)
		if s == nil {
			s = &db.UserSettings{ChatID: chatID, MinAmount: 1000, MaxTrades: 5, MinOdds: 1.15}
		}
		s.Categories = cats
		db.UpsertSettings(ctx, s)
		clearPendingCats(chatID)
		b.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID, Text: "💾 Категории сохранены!", ShowAlert: true,
		})
		if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
			b.EditMessageText(ctx, &telego.EditMessageTextParams{
				ChatID:      tu.ID(chatID),
				MessageID:   msg.MessageID,
				Text:        "⚙️ <b>Ваши настройки</b>\n\nНажмите на параметр чтобы изменить его.",
				ParseMode:   telego.ModeHTML,
				ReplyMarkup: SettingsKeyboard(s),
			})
		}
		return

	default:
		var newCats []string
		if len(cats) == 0 {
			for k := range polymarket.AllCategories {
				if k != key {
					newCats = append(newCats, k)
				}
			}
		} else {
			found := false
			for _, c := range cats {
				if c == "__none__" {
					continue
				}
				if c == key {
					found = true
					continue
				}
				newCats = append(newCats, c)
			}
			if !found {
				newCats = append(newCats, key)
			}
			// When all categories are manually selected, treat as "empty = all enabled".
			if len(newCats) >= len(polymarket.AllCategories) {
				newCats = []string{}
			}
			// When all manually deselected, use __none__ sentinel (not empty = all).
			if len(newCats) == 0 && found {
				newCats = []string{"__none__"}
			}
		}
		cats = newCats
		setPendingCats(chatID, cats)
	}

	if msg, ok := cb.Message.(*telego.Message); ok && msg != nil {
		b.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
			ChatID:      tu.ID(chatID),
			MessageID:   msg.MessageID,
			ReplyMarkup: CategoriesKeyboard(cats),
		})
	}
}

// ─── Mini App data handler ─────────────────────────────────────────────────────

func handleWebAppData(b *telego.Bot, msg *telego.Message) {
	ctx := context.Background()
	chatID := msg.Chat.ID
	data := msg.WebAppData.Data

	var base struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(data), &base); err != nil || base.Action == "" {
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Ошибка данных из Mini App").
			WithReplyMarkup(MainMenuKeyboard()))
		return
	}

	switch base.Action {
	case "save_settings":
		var body struct {
			MinAmount  float64  `json:"min_amount"`
			MaxTrades  int      `json:"max_trades"`
			MinOdds    float64  `json:"min_odds"`
			Categories []string `json:"categories"`
		}
		if err := json.Unmarshal([]byte(data), &body); err != nil {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Ошибка данных настроек").
				WithReplyMarkup(MainMenuKeyboard()))
			return
		}
		if body.MinAmount < 100 {
			body.MinAmount = 100
		}
		if body.MaxTrades < 1 {
			body.MaxTrades = 1
		}
		if body.MinOdds < 1.01 {
			body.MinOdds = 1.01
		}
		s := &db.UserSettings{
			ChatID:     chatID,
			MinAmount:  body.MinAmount,
			MaxTrades:  body.MaxTrades,
			MinOdds:    body.MinOdds,
			Categories: body.Categories,
		}
		if err := db.UpsertSettings(ctx, s); err != nil {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Ошибка сохранения настроек").
				WithReplyMarkup(MainMenuKeyboard()))
			return
		}
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "✅ Настройки сохранены!").
			WithReplyMarkup(MainMenuKeyboard()))

	case "delete_watches":
		var body struct {
			Wallets []string `json:"wallets"`
		}
		if err := json.Unmarshal([]byte(data), &body); err != nil || len(body.Wallets) == 0 {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Ошибка: нет кошельков для удаления").
				WithReplyMarkup(MainMenuKeyboard()))
			return
		}
		for _, wallet := range body.Wallets {
			db.RemoveWatch(ctx, chatID, wallet)
		}
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			fmt.Sprintf("👁 Перестали следить за %d кошельками", len(body.Wallets))).
			WithReplyMarkup(MainMenuKeyboard()))

	case "add_watch":
		var body struct {
			Wallet string `json:"wallet"`
		}
		if err := json.Unmarshal([]byte(data), &body); err != nil || len(body.Wallet) < 10 {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Неверный адрес кошелька").
				WithReplyMarkup(MainMenuKeyboard()))
			return
		}
		if err := db.AddWatch(ctx, chatID, body.Wallet, ""); err == db.ErrWatchlistFull {
			b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Лимит: можно следить максимум за 10 кошельками").
				WithReplyMarkup(MainMenuKeyboard()))
			return
		}
		b.SendMessage(ctx, tu.Message(tu.ID(chatID),
			fmt.Sprintf("🔔 Теперь вы следите за:\n<code>%s</code>", body.Wallet)).
			WithParseMode(telego.ModeHTML).WithReplyMarkup(MainMenuKeyboard()))

	case "back":
		// User closed Mini App without saving — restore main keyboard.
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "🏠 Главное меню").
			WithReplyMarkup(MainMenuKeyboard()))

	default:
		b.SendMessage(ctx, tu.Message(tu.ID(chatID), "❌ Неизвестное действие").
			WithReplyMarkup(MainMenuKeyboard()))
	}
}
