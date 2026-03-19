package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus counters for the reengage service.
type Metrics struct {
	registry   *prometheus.Registry
	runs       *prometheus.CounterVec
	candidates prometheus.Counter
	firstSent  prometheus.Counter
	secondSent prometheus.Counter
	cancelled  prometheus.Counter
}

func New() *Metrics {
	reg := prometheus.NewRegistry()

	runs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bachata_reengage_runs_total",
		Help: "Total sync runs partitioned by result (success/failed).",
	}, []string{"result"})

	candidates := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bachata_reengage_candidates_total",
		Help: "Total unique dialogs inspected.",
	})

	firstSent := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bachata_reengage_first_reminders_total",
		Help: "Total first reminders simulated.",
	})

	secondSent := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bachata_reengage_second_reminders_total",
		Help: "Total second reminders simulated.",
	})

	cancelled := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bachata_reengage_cancellations_total",
		Help: "Total flows cancelled because phone was received.",
	})

	reg.MustRegister(runs, candidates, firstSent, secondSent, cancelled)

	return &Metrics{
		registry:   reg,
		runs:       runs,
		candidates: candidates,
		firstSent:  firstSent,
		secondSent: secondSent,
		cancelled:  cancelled,
	}
}

func (m *Metrics) ObserveRun(result string) { m.runs.WithLabelValues(result).Inc() }
func (m *Metrics) ObserveCandidates(n int)  { m.candidates.Add(float64(n)) }
func (m *Metrics) ObserveFirstReminder()    { m.firstSent.Inc() }
func (m *Metrics) ObserveSecondReminder()   { m.secondSent.Inc() }
func (m *Metrics) ObserveCancellation()     { m.cancelled.Inc() }

// Handler returns an HTTP handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
