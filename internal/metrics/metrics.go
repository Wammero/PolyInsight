package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventLag = promauto.NewSummary(prometheus.SummaryOpts{
		Name:       "bot_event_processed_lag_seconds",
		Help:       "Полное время от события WS до отправки в TG",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	BufferGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "bot_worker_buffer_messages",
		Help: "Заполнение внутреннего канала задач",
	})

	WSMessages = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_ws_messages_total",
		Help: "Всего сообщений из сокета",
	})

	WhaleEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_whale_events_total",
		Help: "Сделок выше порога (киты)",
	})

	PerfectAlerts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_perfect_alerts_total",
		Help: "Отправлено алертов в TG",
	})

	ClicksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_link_clicks_total",
		Help: "Всего кликов по трекинговым ссылкам",
	})
)
