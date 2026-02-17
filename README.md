# Polymarket Whale Tracker — Telegram Bot

## Project Overview

**Polymarket Whale Tracker** is a real-time Telegram bot that monitors large trades by new participants ("whales") on the Polymarket prediction market and delivers instant personalized alerts to subscribers.

The bot connects directly to Polymarket's WebSocket feed and processes every trade in real time, filtering for significant activity from new players — the kind of traders whose moves signal high-conviction bets and are most interesting to follow. Users receive rich alerts with wallet statistics, P&L, win rate, and one-click access to the market and trader profile.

---

## Problem

Polymarket has hundreds of active markets and thousands of trades per day. For a regular user, it is virtually impossible to manually monitor when a new high-stakes player enters a market. Yet these moments are among the most actionable signals in prediction markets — a large bet by an unknown wallet placing $10,000+ on a 30% outcome deserves attention.

There is no native Polymarket notification system for this kind of activity. Users either have to constantly refresh the website or miss these opportunities entirely.

---

## Solution

The bot solves this by doing three things:

1. **Real-time detection** — subscribes to Polymarket's WebSocket and processes every trade as it happens, with no polling delay.
2. **Intelligent filtering** — each user sets their own filters (minimum bet size, maximum trade count to define "newbie", minimum odds, market categories). Only relevant alerts are delivered.
3. **Rich context** — every alert includes the wallet's all-time P&L, win rate, number of past trades, market category, odds, transaction hash, and links to Polymarket profile, DeBank portfolio, and PolygonScan.

---

## Key Features

### For users
- **Instant whale alerts** with full wallet context (W/L ratio, total P&L, trade history)
- **Fully personalized filters**: minimum bet amount, "newbie" threshold (max N trades ever), minimum odds multiplier, and market categories
- **20 market categories** supported: politics, elections, crypto, finance, economy, geopolitics, tech, culture, sports (NBA, NFL, NHL, UFC, tennis, esports, cricket, rugby, baseball, table tennis, and more)
- **Wallet watchlist** — follow up to 10 specific wallets and receive a dedicated notification every time they trade, regardless of your global filters
- **Telegram Mini App** for settings management — sliders, manual input, category selector with Telegram CloudStorage sync (category selections persist across sessions automatically)
- **One-click navigation** — inline buttons in every alert to view market, check DeBank portfolio, and inspect the transaction on PolygonScan
- **Click tracking** — optional redirect server to track link engagement per alert

### Technical
- **WebSocket → worker pool** architecture: 16 parallel Go goroutines consume a 5,000-item buffered channel, ensuring no trades are missed even during traffic spikes
- **In-memory follow cache** — watchlist data is cached in memory and refreshed every 30 seconds; no database query per trade for real-time follow detection
- **Redis caching** — wallet stats cached 15 minutes, trade counts 10 minutes; Polymarket API is never hammered
- **PostgreSQL** for persistent user data (settings, watchlist, click tracking)
- **Prometheus + Grafana** — built-in metrics for alert volume, processing lag, buffer fill, and click counts
- **Docker Compose** — fully containerized, one-command deployment

---

## Architecture

```
Polymarket WebSocket
        │
        ▼
   jobs channel (5,000 buffer)
        │
   ┌────┴────┐
   │ Workers │  ×16 goroutines
   └────┬────┘
        │  per trade:
        │  1. Volume pre-filter (global minimum)
        │  2. Ban-word filter (title)
        │  3. Category metadata (Redis-cached)
        │  4. Trade count → newbie check (Redis-cached)
        │  5. Wallet stats (Redis-cached, 15 min)
        │  6. SendAlert → per-user filter + DB fan-out
        │  7. SendFollowAlert → in-memory follow cache lookup
        ▼
   Telegram Bot API
```

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.22 |
| Telegram library | [telego](https://github.com/mymmrac/telego) |
| Database | PostgreSQL 15 |
| Cache | Redis 7 |
| Metrics | Prometheus + Grafana |
| Frontend (Mini App) | Vanilla HTML/JS, hosted on GitHub Pages |
| Infrastructure | Docker Compose |
| Data source | Polymarket WebSocket + REST API |

---

## Data Sources Used

All data comes exclusively from public Polymarket APIs:

- **WebSocket** `wss://ws-subscriptions-clob.polymarket.com/ws/market` — live trade feed
- `data-api.polymarket.com/traded?user=` — trade count per wallet
- `data-api.polymarket.com/v1/leaderboard?user=&timePeriod=ALL` — all-time P&L
- `data-api.polymarket.com/closed-positions?user=&limit=500` — win/loss breakdown
- `gamma-api.polymarket.com/markets?slug=` — market category metadata

---

## Impact on Polymarket Ecosystem

**Increased engagement** — users who receive timely alerts visit Polymarket more often and are more likely to participate in markets they discover through the bot.

**New user acquisition** — the bot reaches Telegram users who may not have a habit of actively visiting the Polymarket website. Alerts with direct deep links into specific markets lower the barrier to entry.

**Market discovery** — by surfacing large bets across 20+ categories, the bot helps users discover markets they would not have found on their own, including niche sports, geopolitics, and emerging crypto events.

**Whale signal amplification** — when experienced traders (high P&L, strong win rate) make a large bet, the community benefits from knowing about it. The bot makes this information accessible instantly, contributing to better market efficiency.

**Community building** — the Telegram format is inherently social. Users discuss alerts in groups, share interesting wallets, and follow the same trades together — creating a community layer on top of Polymarket activity.

---

## Current Status

The bot is fully functional and deployed. It is actively processing Polymarket trades in real time and delivering alerts to subscribers. All core features are implemented:

- ✅ Real-time WebSocket trade processing
- ✅ Per-user personalized filters (amount, trades, odds, categories)
- ✅ Wallet statistics (P&L, win rate, trade history) via Polymarket API
- ✅ Wallet watchlist with real-time follow notifications (max 10 wallets)
- ✅ Telegram Mini App for settings (GitHub Pages, no server required)
- ✅ Prometheus/Grafana monitoring
- ✅ Full Docker Compose deployment

---

## Grant Usage

The grant would be used to:

1. **Scale infrastructure** — move from local deployment to a cloud VPS for 24/7 uptime and lower latency to Polymarket's WebSocket
2. **Expand language support** — add English-language alerts and UI to reach a global audience beyond the current Russian-speaking user base
3. **Enhance alert quality** — integrate additional on-chain signals (wallet age, historical category performance, open position size) to make alerts more informative
4. **Bot discovery** — run Telegram community outreach and create educational content showing how to use whale signals on Polymarket

---

## Links

- **Telegram Mini App (settings UI):** https://wammero.github.io/poly_site/
- **Source code:** private (available for review upon request)
