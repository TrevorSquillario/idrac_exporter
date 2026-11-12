package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestCollector() *Collector {
	return &Collector{
		DellAverageFanPWM: prometheus.NewDesc(
			"idrac_dell_average_fan_pwm",
			"Average fan PWM (pulse-width modulation) percentage reported by the chassis environment metrics",
			nil, nil,
		),
	}
}

func TestNewDellAverageFanPWM_Value(t *testing.T) {
	mc := newTestCollector()
	ch := make(chan prometheus.Metric, 1)

	m := &EnvironmentMetrics{}
	m.Oem.Dell = &DellEnvironmentMetrics{AverageFanPWM: 17.0}

	mc.NewDellAverageFanPWM(ch, m)

	select {
	case metric := <-ch:
		d := &dto.Metric{}
		if err := metric.Write(d); err != nil {
			t.Fatalf("metric write: %v", err)
		}
		if d.GetGauge() == nil {
			t.Fatalf("expected gauge metric")
		}
		if d.GetGauge().GetValue() != 17 {
			t.Fatalf("expected 17, got %v", d.GetGauge().GetValue())
		}
		if len(d.GetLabel()) != 0 {
			t.Fatalf("expected no labels, got %d", len(d.GetLabel()))
		}
	default:
		t.Fatalf("expected a metric to be emitted")
	}
}

func TestNewDellAverageFanPWM_ZeroIsEmitted(t *testing.T) {
	mc := newTestCollector()
	ch := make(chan prometheus.Metric, 1)

	m := &EnvironmentMetrics{}
	m.Oem.Dell = &DellEnvironmentMetrics{AverageFanPWM: 0.0}

	mc.NewDellAverageFanPWM(ch, m)

	select {
	case metric := <-ch:
		d := &dto.Metric{}
		if err := metric.Write(d); err != nil {
			t.Fatalf("metric write: %v", err)
		}
		if d.GetGauge().GetValue() != 0 {
			t.Fatalf("expected 0, got %v", d.GetGauge().GetValue())
		}
	default:
		t.Fatalf("expected a metric to be emitted for a zero reading")
	}
}

func TestNewDellAverageFanPWM_NonDellSkipped(t *testing.T) {
	mc := newTestCollector()
	ch := make(chan prometheus.Metric, 1)

	m := &EnvironmentMetrics{}

	mc.NewDellAverageFanPWM(ch, m)

	select {
	case <-ch:
		t.Fatalf("expected no metric for a non-Dell system")
	default:
	}
}

func TestNewDellAverageFanPWM_AbsentValueSkipped(t *testing.T) {
	mc := newTestCollector()
	ch := make(chan prometheus.Metric, 1)

	m := &EnvironmentMetrics{}
	m.Oem.Dell = &DellEnvironmentMetrics{}

	mc.NewDellAverageFanPWM(ch, m)

	select {
	case <-ch:
		t.Fatalf("expected no metric when AverageFanPWM is absent")
	default:
	}
}

func TestNewDellAverageFanPWM_UnparseableValueSkipped(t *testing.T) {
	mc := newTestCollector()
	ch := make(chan prometheus.Metric, 1)

	m := &EnvironmentMetrics{}
	m.Oem.Dell = &DellEnvironmentMetrics{AverageFanPWM: "N/A"}

	mc.NewDellAverageFanPWM(ch, m)

	select {
	case <-ch:
		t.Fatalf("expected no metric for an unparseable AverageFanPWM")
	default:
	}
}
