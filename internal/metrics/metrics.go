package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry           *prometheus.Registry
	runsTotal          *prometheus.CounterVec
	candidatesTotal    prometheus.Counter
	firstReminders     prometheus.Counter
	secondReminders    prometheus.Counter
	cancellationsTotal prometheus.Counter
}

func New() *Metrics {
	registry := prometheus.NewRegistry()
	factory := promauto.With(registry)

	return &Metrics{
		registry: registry,
		runsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "bachata_reengage_runs_total",
			Help: "Number of reminder task runs partitioned by result.",
		}, []string{"result"}),
		candidatesTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "bachata_reengage_candidates_total",
			Help: "Number of unique dialog candidates inspected.",
		}),
		firstReminders: factory.NewCounter(prometheus.CounterOpts{
			Name: "bachata_reengage_first_reminders_total",
			Help: "Number of simulated first reminders.",
		}),
		secondReminders: factory.NewCounter(prometheus.CounterOpts{
			Name: "bachata_reengage_second_reminders_total",
			Help: "Number of simulated second reminders.",
		}),
		cancellationsTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "bachata_reengage_cancellations_total",
			Help: "Number of reminder flows cancelled because a phone was received.",
		}),
	}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveRun(result string) {
	m.runsTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) ObserveCandidates(n int) {
	m.candidatesTotal.Add(float64(n))
}

func (m *Metrics) ObserveFirstReminder() {
	m.firstReminders.Inc()
}

func (m *Metrics) ObserveSecondReminder() {
	m.secondReminders.Inc()
}

func (m *Metrics) ObserveCancellation() {
	m.cancellationsTotal.Inc()
}
