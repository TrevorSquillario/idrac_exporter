package collector

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricType describes how a telemetry metric should be exposed to Prometheus.
type MetricType string

const (
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeCounter   MetricType = "counter"
	MetricTypeTimestamp MetricType = "timestamp"
)

// The keyword sets below are matched against the sanitized (lowercase,
// snake_case) metric name, split into whole "_"-separated tokens. Matching
// whole tokens instead of bare substrings avoids misclassifying gauges such as
// "bytes_free" or "bytes_per_second" as counters.
//
// The slices are package-level so they are not reallocated on every scrape.

// gaugeQualifiers are checked against the LAST token of the name. A trailing
// qualifier means the metric is a point-in-time reading (a gauge) even when an
// earlier token looks like a counter, e.g. "bytes_free" or "energy_percent".
var gaugeQualifiers = []string{
	"free",
	"available",
	"used",
	"percent",
	"pct",
	"ratio",
	"utilization",
	"second",
	"minute",
	"hour",
	"avg",
	"average",
	"min",
	"max",
	"capacity",
	"size",
	"reading",
	"threshold",
	"limit",
	"rate",
}

// counterTokens mark a metric as a monotonically increasing counter when they
// appear as any whole token (accumulators, error counters, byte and packet
// counters, energy registers, etc.).
var counterTokens = []string{
	"count",
	"counter",
	"bytes",
	"pkts",
	"packets",
	"energy",
	"error",
	"errors",
	"wh", // watt-hours, e.g. "energyk_wh"
}

// counterPrefixes mark a metric as a counter when they are the FIRST token,
// e.g. "rx_bytes" or "tx_packets".
var counterPrefixes = []string{
	"rx",
	"tx",
}

// counterSubstrings are compound markers that are matched as substrings rather
// than whole tokens.
var counterSubstrings = []string{
	"acc_core_cycles",
	"limiting_events", // e.g. idrac_telemetry_limiting_events
}

// containsToken reports whether tokens contains s.
func containsToken(tokens []string, s string) bool {
	for _, t := range tokens {
		if t == s {
			return true
		}
	}
	return false
}

// ClassifyMetric evaluates a dynamic iDRAC telemetry metric name and
// categorizes it as a Counter, Timestamp, or Gauge.
//
// The name is expected to be the sanitized metric name produced by
// sanitizeMetricName, but the function lowercases it defensively so it also
// works with raw MetricId values.
func ClassifyMetric(name string) MetricType {
	lowerName := strings.ToLower(name)

	// 1. Check for Timestamps and Epoch markers first.
	// Matches: energy_timestamp, last_day_max_power_time, etc.
	if strings.Contains(lowerName, "timestamp") ||
		strings.HasSuffix(lowerName, "_time") ||
		strings.Contains(lowerName, "_time_") {
		return MetricTypeTimestamp
	}

	tokens := strings.Split(lowerName, "_")

	// 2. A trailing gauge qualifier wins over counter-looking tokens, so
	// "bytes_free", "energy_percent" and "read_error_rate" stay gauges.
	if len(tokens) > 0 && containsToken(gaugeQualifiers, tokens[len(tokens)-1]) {
		return MetricTypeGauge
	}

	// 3. Check for Counters (accumulators, errors, bytes, packets, energy
	// registers).
	for _, t := range tokens {
		if containsToken(counterTokens, t) {
			return MetricTypeCounter
		}
	}

	if len(tokens) > 0 && containsToken(counterPrefixes, tokens[0]) {
		return MetricTypeCounter
	}

	for _, kw := range counterSubstrings {
		if strings.Contains(lowerName, kw) {
			return MetricTypeCounter
		}
	}

	// 4. Default fallback to Gauge for everything else (temperatures,
	// voltages, power draws, fan RPMs, percentages, statuses, and residency
	// clocks).
	return MetricTypeGauge
}

// ValueType maps a MetricType to the prometheus.ValueType used when emitting
// the metric.
//
// Prometheus has no dedicated timestamp value type, so timestamps are exposed
// as gauges. Their value is already normalized to Unix seconds by
// parseMetricValue.
func (t MetricType) ValueType() prometheus.ValueType {
	switch t {
	case MetricTypeCounter:
		return prometheus.CounterValue
	default:
		return prometheus.GaugeValue
	}
}
