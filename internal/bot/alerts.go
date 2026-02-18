package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"polymarket_go/internal/db"
	"polymarket_go/internal/metrics"
	"polymarket_go/internal/polymarket"
)


// SendAlert broadcasts a whale alert to all subscribers that match their personal filters.
// followers is the set of chatIDs who are watching this wallet (from the follow cache).
func SendAlert(
	ctx context.Context,
	b *telego.Bot,
	p polymarket.TradePayload,
	trades int,
	stats *polymarket.WalletStats,
	meta *polymarket.CategoryMeta,
	startTime time.Time,
	baseURL string,
	followers []int64,
) {
	subs := GetSubscribers() // from in-memory cache, no DB call
	if len(subs) == 0 {
		return
	}

	volume := p.Size * p.Price
	odds := 1.0 / p.Price
	probability := p.Price * 100

	// Short ID for click tracking: strip 0x prefix, take 20 hex chars (80-bit entropy).
	// 20 chars vs original 12: collision probability drops from ~10^-14 to ~10^-24 per million txs.
	shortID := strings.TrimPrefix(p.TransactionHash, "0x")
	if len(shortID) > 20 {
		shortID = shortID[:20]
	}

	marketURL := fmt.Sprintf("https://polymarket.com/market/%s?utm_source=whalebot&utm_medium=telegram&utm_campaign=alert&tid=%s",
		p.Slug, p.TransactionHash)

	// Register alert for click tracking (first occurrence wins due to ON CONFLICT DO NOTHING).
	db.RegisterAlert(ctx, shortID, p.ProxyWallet, p.Slug, marketURL, p.TransactionHash, 0)

	// Build tracked URLs.
	// Priority: redirect server (self-hosted, full Prometheus metrics) >
	//           is.gd (free external, stats on is.gd website) >
	//           direct links (no tracking).
	rawProfileURL := "https://polymarket.com/profile/" + p.ProxyWallet
	rawDebankURL := "https://debank.com/profile/" + p.ProxyWallet
	rawPgsanURL := "https://polygonscan.com/tx/" + p.TransactionHash

	var trackURL, profileURL, debankURL, pgsanURL string
	if baseURL != "" {
		// Self-hosted redirect server: all metrics go to Prometheus/Grafana.
		trackURL = fmt.Sprintf("%s/r/%s", baseURL, shortID)
		profileURL = fmt.Sprintf("%s/r/%s/profile", baseURL, shortID)
		debankURL = fmt.Sprintf("%s/r/%s/debank", baseURL, shortID)
		pgsanURL = fmt.Sprintf("%s/r/%s/pgsan", baseURL, shortID)
	} else {
		// No redirect server: use direct links.
		// is.gd is NOT used here — even with caching, HTTP calls block workers
		// and cause buffer overflow under load.
		trackURL = marketURL
		profileURL = rawProfileURL
		debankURL = rawDebankURL
		pgsanURL = rawPgsanURL
	}

	name := p.Pseudonym
	if name == "" {
		walletShort := p.ProxyWallet
		if len(walletShort) > 8 {
			walletShort = walletShort[:6] + "..." + walletShort[len(walletShort)-4:]
		}
		name = walletShort
	}

	// Win-rate line with wins/losses counts (closed positions only).
	wrLine := "  🎯 Результат: —"
	if stats.WinRate >= 0 {
		wrLine = fmt.Sprintf("  🎯 Результат: %d✅ / %d❌ (%.0f%%)", stats.Wins, stats.Losses, stats.WinRate)
	}

	// P&L line — all-time, optional.
	playerBlock := wrLine
	if stats.TotalPnL > -999998 {
		playerBlock += fmt.Sprintf("\n  💼 P&L: %s", fmtPnL(stats.TotalPnL))
	}

	text := fmt.Sprintf(
		"%s <b>КИТ-НОВИЧОК: %s</b>\n\n"+
			"💰 <b>Сумма:</b> $%.0f\n"+
			"📊 <b>Коэф:</b> x%.2f (%.1f%%)\n"+
			"📈 <b>Всего сделок:</b> %d\n"+
			"👤 <b>Игрок:</b> <code>%s</code>\n"+
			"%s\n\n"+
			"🎯 <b>Рынок:</b> %s\n"+
			"📌 <b>Ставка на:</b> %s\n\n"+
			"🔗 <b>Ссылки:</b>\n"+
			"• <a href='%s'>Профиль Polymarket</a>\n"+
			"• <a href='%s'>Портфель DeBank</a>\n"+
			"• <a href='%s'>PolygonScan</a>",
		meta.Emoji, strings.ToUpper(meta.Label),
		volume, odds, probability,
		trades,
		name,
		playerBlock,
		p.Title,
		strings.ToUpper(p.Outcome),
		profileURL, debankURL, pgsanURL,
	)

	// Build a lookup set of follower chatIDs to avoid extra DB calls per subscriber.
	followerSet := make(map[int64]bool, len(followers))
	for _, id := range followers {
		followerSet[id] = true
	}

	for _, sub := range subs {
		// Per-user amount filter.
		if volume < sub.MinAmount {
			continue
		}
		// Per-user trade count filter.
		if trades >= sub.MaxTrades {
			continue
		}
		// Per-user odds filter.
		if odds < sub.MinOdds {
			continue
		}
		// Per-user category filter (empty = all).
		if len(sub.Categories) > 0 {
			allowed := false
			for _, cat := range sub.Categories {
				if cat == meta.Category {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		isWatch := followerSet[sub.ChatID]
		kb := AlertKeyboard(p.ProxyWallet, shortID, isWatch)

		msg := tu.Message(tu.ID(sub.ChatID), text+
			fmt.Sprintf("\n• <a href='%s'>Перейти к сделке ↗</a>", trackURL)).
			WithParseMode(telego.ModeHTML).
			WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true}).
			WithReplyMarkup(kb)

		Enqueue(msg) // rate-limited queue: 25 msg/sec, never exceeds Telegram limit
		metrics.PerfectAlerts.Inc()
	}

	metrics.EventLag.Observe(time.Since(startTime).Seconds())
}

// SendFollowAlert sends a dedicated notification to users who are following this wallet.
// Sent regardless of filter settings — followers always want to know about their wallets.
func SendFollowAlert(
	ctx context.Context,
	b *telego.Bot,
	p polymarket.TradePayload,
	trades int,
	stats *polymarket.WalletStats,
	meta *polymarket.CategoryMeta,
	followers []int64,
) {
	volume := p.Size * p.Price
	odds := 1.0 / p.Price

	name := p.Pseudonym
	if name == "" {
		if len(p.ProxyWallet) > 10 {
			name = p.ProxyWallet[:6] + "..." + p.ProxyWallet[len(p.ProxyWallet)-4:]
		} else {
			name = p.ProxyWallet
		}
	}

	wrLine := "—"
	if stats.WinRate >= 0 {
		wrLine = fmt.Sprintf("%d✅/%d❌ (%.0f%%)", stats.Wins, stats.Losses, stats.WinRate)
	}

	text := fmt.Sprintf(
		"🔔 <b>Ваш кит совершил сделку!</b>\n\n"+
			"👤 <code>%s</code>\n"+
			"💰 <b>Сумма:</b> $%.0f  📊 <b>Коэф:</b> x%.2f\n"+
			"🎯 <b>Ставка на:</b> %s\n"+
			"📝 <b>Рынок:</b> %s\n"+
			"📈 <b>Всего сделок:</b> %d  🏆 <b>WR:</b> %s\n\n"+
			"🔗 <a href='https://polymarket.com/profile/%s'>Профиль на Polymarket</a>",
		name,
		volume, odds,
		strings.ToUpper(p.Outcome),
		p.Title,
		trades, wrLine,
		p.ProxyWallet,
	)

	for _, chatID := range followers {
		Enqueue(tu.Message(tu.ID(chatID), text).
			WithParseMode(telego.ModeHTML).
			WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true}))
	}
}

// fmtPnL formats a P&L value. Returns "—" when unknown (-999999 sentinel).
func fmtPnL(v float64) string {
	if v <= -999998 {
		return "—"
	}
	sign := "+"
	if v < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s$%.0f", sign, v)
}

// FormatWalletStats returns a formatted text block for a wallet's statistics.
func FormatWalletStats(wallet string, stats *polymarket.WalletStats) string {
	wrStr := "—"
	if stats.WinRate >= 0 {
		wrStr = fmt.Sprintf("%d✅ / %d❌ (%.1f%%)", stats.Wins, stats.Losses, stats.WinRate)
	}

	walletShort := wallet
	if len(walletShort) > 12 {
		walletShort = wallet[:6] + "..." + wallet[len(wallet)-4:]
	}

	return fmt.Sprintf(
		"📊 <b>Статистика кошелька</b>\n"+
			"<code>%s</code>\n\n"+
			"📈 Всего сделок: <b>%d</b>\n"+
			"🎯 Победы/Поражения: <b>%s</b>\n"+
			"💹 P&L (ALL): <b>%s</b>\n\n"+
			"🔗 <a href='https://polymarket.com/profile/%s'>Профиль Polymarket</a>\n"+
			"🏦 <a href='https://debank.com/profile/%s'>Портфель DeBank</a>",
		walletShort,
		stats.TotalTrades,
		wrStr,
		fmtPnL(stats.TotalPnL),
		wallet, wallet,
	)
}
