package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ── WebSocket ──────────────────────────────────────────────────────────
	WSMessages = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_ws_messages_total",
		Help: "Всего сообщений из WebSocket",
	})

	WSDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_ws_dropped_total",
		Help: "Сообщений сброшено: буфер был полон",
	})

	WSReconnects = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_ws_reconnects_total",
		Help: "Переподключений к WebSocket Polymarket",
	})

	// ── Buffer ─────────────────────────────────────────────────────────────
	BufferGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_worker_buffer_messages",
		Help: "Текущее кол-во сообщений в буфере (канале задач)",
	})

	BufferCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_worker_buffer_capacity",
		Help: "Максимальный размер буфера (константа)",
	})

	// ── Pipeline ───────────────────────────────────────────────────────────
	WhaleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_whale_events_total",
		Help: "Сделок прошедших порог объёма (киты)",
	})

	PerfectAlerts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_perfect_alerts_total",
		Help: "Алертов отправлено в Telegram",
	})

	FollowAlerts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_follow_alerts_total",
		Help: "Алертов «ваш кит совершил сделку»",
	})

	ClicksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bot_link_clicks_total",
		Help: "Кликов по трекинговым ссылкам",
	}, []string{"type"})

	GeoClicksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bot_link_clicks_by_country_total",
		Help: "Кликов по трекинговым ссылкам с разбивкой по стране",
	}, []string{"country"})

	// ── Filtering ──────────────────────────────────────────────────────────
	FilteredVolume = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_trades_filtered_volume_total",
		Help: "Отфильтровано: объём ниже порога",
	})

	FilteredBanWord = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_trades_filtered_banword_total",
		Help: "Отфильтровано: стоп-слово в названии рынка",
	})

	FilteredNoMeta = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_trades_filtered_no_meta_total",
		Help: "Отфильтровано: метаданные рынка не найдены",
	})

	FilteredAPIErr = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_trades_filtered_api_error_total",
		Help: "Пропущено: ошибка API при запросе кошелька",
	})

	// ── Latency ────────────────────────────────────────────────────────────
	EventLag = promauto.NewSummary(prometheus.SummaryOpts{
		Name:       "bot_event_processed_lag_seconds",
		Help:       "Полное время от WS-события до отправки алерта в TG",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	WorkerDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "bot_worker_duration_seconds",
		Help:    "Время обработки одной сделки воркером (включая API вызовы)",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	})

	WorkerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_worker_count",
		Help: "Кол-во запущенных воркеров",
	})

	// ── Send queue ─────────────────────────────────────────────────────────
	SendQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_send_queue_size",
		Help: "Сообщений в очереди отправки в Telegram",
	})

	SendQueueDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_send_queue_dropped_total",
		Help: "Сообщений сброшено: очередь отправки переполнена",
	})

	// ── Users ──────────────────────────────────────────────────────────────
	AllUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_all_users",
		Help: "Всего подписчиков бота (текущих)",
	})

	WatchlistEntries = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_watchlist_entries",
		Help: "Всего записей в watchlist (слежка за кошельками)",
	})
)
