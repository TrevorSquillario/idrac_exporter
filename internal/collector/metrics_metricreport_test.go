package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mrlhansen/idrac_exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestNewMetricReportEntry_FromSamples(t *testing.T) {
	cases := []struct {
		name          string
		data          string
		expectedValue float64
		expectedCtx   string
		expectedFqdd  string
		expectedType  MetricType
		expectedName  string
	}{
		{
			name:          "AverageConsumedWatts_string",
			data:          `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":"507","Timestamp":"2025-06-23T19:06:14Z"}]}`,
			expectedValue: 507,
			expectedCtx:   "",
			expectedFqdd:  "",
			expectedType:  MetricTypeGauge,
			expectedName:  "idrac_telemetry_average_consumed_watts",
		},
		{
			name:          "AverageConsumedWatts_number",
			data:          `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":507,"Timestamp":"2025-06-23T19:06:14Z"}]}`,
			expectedValue: 507,
			expectedCtx:   "",
			expectedFqdd:  "",
			expectedType:  MetricTypeGauge,
			expectedName:  "idrac_telemetry_average_consumed_watts",
		},
		{
			name:          "TemperatureReading",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"TemperatureReading","MetricProperty":"","MetricValue":"50","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"Embedded NIC 1 Port 1 Partition 1","FQDD":"NIC.Embedded.1-1-1","Label":"Embedded NIC 1 Port 1 Partition 1 TemperatureReading","Source":"hiitempsensors"}},"Timestamp":"2026-04-01T01:45:35.000Z"}]}`,
			expectedValue: 50,
			expectedCtx:   "Embedded NIC 1 Port 1 Partition 1",
			expectedFqdd:  "NIC.Embedded.1-1-1",
			expectedType:  MetricTypeGauge,
			expectedName:  "idrac_telemetry_temperature_reading",
		},
		{
			name:          "AddressParityError_bool_string",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"AddressParityError","MetricProperty":"/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.Socket.A1/MemoryMetrics#HealthData/AlarmTrips/AddressParityError","MetricValue":"false","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"DIMM A1","FQDD":"DIMM.Socket.A1","Label":"DIMM A1 AddressParityError","Source":"memorymetrics"}},"Timestamp":"2026-04-01T01:44:50.000Z"}]}`,
			expectedValue: 0,
			expectedCtx:   "DIMM A1",
			expectedFqdd:  "DIMM.Socket.A1",
			expectedType:  MetricTypeCounter,
			expectedName:  "idrac_telemetry_address_parity_error_total",
		},
		{
			name:          "ReadErrorRate",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"ReadErrorRate","MetricProperty":"","MetricValue":"0","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","FQDD":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","Label":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1 ReadErrorRate","Source":"smart-sas"}},"Timestamp":"2026-04-01T00:52:09.000Z"}]}`,
			expectedValue: 0,
			expectedCtx:   "SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1",
			expectedFqdd:  "SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1",
			expectedType:  MetricTypeGauge,
			expectedName:  "idrac_telemetry_read_error_rate",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var resp MetricReportResponse
			if err := json.Unmarshal([]byte(c.data), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(resp.MetricValues) != 1 {
				t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
			}
			mv := resp.MetricValues[0]

			v, err := parseMetricValue(mv)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			// ensure deterministic metric family name generation
			if config.Config == nil {
				config.SetConfig(config.NewConfig())
			}
			old := config.Config.MetricsPrefix
			config.Config.MetricsPrefix = "idrac"
			defer func() { config.Config.MetricsPrefix = old }()

			ch := make(chan prometheus.Metric, 1)
			var mc Collector

			// build pointers for context/fqdd if present
			var ctxPtr, fqPtr *string
			if mv.Oem != nil && mv.Oem.Dell != nil {
				if mv.Oem.Dell.ContextID != "" {
					ctxPtr = &mv.Oem.Dell.ContextID
				} else if mv.MetricProperty != nil && *mv.MetricProperty == "" {
					tmp := ""
					ctxPtr = &tmp
				}
				if mv.Oem.Dell.FQDD != "" {
					fqPtr = &mv.Oem.Dell.FQDD
				}
			}

			if err := mc.NewMetricReportEntry(ch, resp.Id, mv.MetricId, ctxPtr, fqPtr, v); err != nil {
				t.Fatalf("emit error: %v", err)
			}

			select {
			case m := <-ch:
				dtoMetric := &dto.Metric{}
				if err := m.Write(dtoMetric); err != nil {
					t.Fatalf("metric write: %v", err)
				}

				// The emitted value type must match the classification.
				switch c.expectedType {
				case MetricTypeCounter:
					if dtoMetric.GetCounter() == nil {
						t.Fatalf("expected counter metric, got %s", dtoMetric)
					}
					if dtoMetric.GetCounter().GetValue() != c.expectedValue {
						t.Fatalf("expected value %v, got %v", c.expectedValue, dtoMetric.GetCounter().GetValue())
					}
				default:
					if dtoMetric.GetGauge() == nil {
						t.Fatalf("expected gauge metric, got %s", dtoMetric)
					}
					if dtoMetric.GetGauge().GetValue() != c.expectedValue {
						t.Fatalf("expected value %v, got %v", c.expectedValue, dtoMetric.GetGauge().GetValue())
					}
				}

				// The metric family name must reflect the classification
				// (counters carry a "_total" suffix).
				if desc := m.Desc().String(); !strings.Contains(desc, `fqName: "`+c.expectedName+`"`) {
					t.Fatalf("expected metric name %q, got %s", c.expectedName, desc)
				}

				labels := dtoMetric.GetLabel()
				if len(labels) != 3 {
					t.Fatalf("expected 3 labels, got %d", len(labels))
				}
				labmap := make(map[string]string, len(labels))
				for _, lp := range labels {
					labmap[lp.GetName()] = lp.GetValue()
				}
				if labmap["metric_report_id"] != resp.Id {
					t.Fatalf("metric_report_id label mismatch: %s", labmap["metric_report_id"])
				}
				if labmap["context"] != c.expectedCtx {
					t.Fatalf("context label mismatch: expected %q got %q", c.expectedCtx, labmap["context"])
				}
				if labmap["fqdd"] != c.expectedFqdd {
					t.Fatalf("fqdd label mismatch: expected %q got %q", c.expectedFqdd, labmap["fqdd"])
				}
			default:
				t.Fatalf("no metric emitted for case %s", c.name)
			}
		})
	}
}

func TestParseMetricValue_StateStrings(t *testing.T) {
	cases := []struct {
		in       interface{}
		expected float64
	}{
		{"Down", 0},
		{"down", 0},
		{"Up", 1},
		{"UP", 1},
		{"Operational", 1},
		{"operational", 1},
		{"Non-operational", 0},
		{"Non-Operational", 0},
		{"NonOperational", 0},
		{"Non operational", 0},
		{"OK", 0},
		{"Degraded", 1},
		{"Critical", 2},
		{"Unknown", 10},
		{"true", 1},
		{"false", 0},
		{"507", 507},
		{507.0, 507},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%v", c.in), func(t *testing.T) {
			mv := MetricReportValue{MetricId: "Test", MetricValue: c.in}
			got, err := parseMetricValue(mv)
			if err != nil {
				t.Fatalf("parse error for %v: %v", c.in, err)
			}
			if got != c.expected {
				t.Fatalf("expected %v for %v, got %v", c.expected, c.in, got)
			}
		})
	}
}

func TestSanitizeMetricName(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		{"TemperatureReading", "temperature_reading"},
		{"RPMReading", "rpm_reading"},
		{"AverageConsumedWatts", "average_consumed_watts"},
		{"SMARTData:Disk.Bay.0", "smart_data_disk_bay_0"},
		{"", "metric"},
		{"123abc", "m_123abc"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := sanitizeMetricName(c.in); got != c.expected {
				t.Fatalf("expected %q, got %q", c.expected, got)
			}
		})
	}
}

func TestMetricHelpKind(t *testing.T) {
	if got := metricHelpKind("TemperatureReading"); got != "ThermalSensor" {
		t.Fatalf("expected ThermalSensor, got %s", got)
	}
	if got := metricHelpKind("RPMReading"); got != "Sensor" {
		t.Fatalf("expected Sensor, got %s", got)
	}
}

// TestMetricReportHelpDeterministic verifies that MetricIds which sanitize to
// the same family name also produce the same help string. A mismatch would
// make registry.Gather() fail with an inconsistent-help error.
func TestMetricReportHelpDeterministic(t *testing.T) {
	if config.Config == nil {
		config.SetConfig(config.NewConfig())
	}
	old := config.Config.MetricsPrefix
	config.Config.MetricsPrefix = "idrac"
	defer func() { config.Config.MetricsPrefix = old }()

	variants := []string{"TemperatureReading", "temperature_reading", "TEMPERATURE_READING"}

	var firstDesc string
	for _, id := range variants {
		ch := make(chan prometheus.Metric, 1)
		var mc Collector
		if err := mc.NewMetricReportEntry(ch, "MetricReport", id, nil, nil, 42); err != nil {
			t.Fatalf("emit error for %q: %v", id, err)
		}

		m := <-ch
		desc := m.Desc().String()
		if firstDesc == "" {
			firstDesc = desc
			continue
		}
		if desc != firstDesc {
			t.Fatalf("help/desc mismatch for %q:\nfirst: %s\ngot:   %s", id, firstDesc, desc)
		}
	}
}

func TestClassifyMetric(t *testing.T) {
	cases := []struct {
		in       string
		expected MetricType
	}{
		// Timestamps are checked before everything else.
		{"energy_timestamp", MetricTypeTimestamp},
		{"last_day_max_power_time", MetricTypeTimestamp},
		{"last_reset_time_seconds", MetricTypeTimestamp},
		{"power_timestamp_seconds", MetricTypeTimestamp},

		// Counters: whole-token matches.
		{"error_count", MetricTypeCounter},
		{"crc_error_count", MetricTypeCounter},
		{"current_pending_sector_count", MetricTypeCounter},
		{"power_cycle_count", MetricTypeCounter},
		{"media_write_count", MetricTypeCounter},
		{"program_fail_count", MetricTypeCounter},
		{"reallocated_block_count", MetricTypeCounter},
		{"rx_bytes", MetricTypeCounter},
		{"tx_bytes", MetricTypeCounter},
		{"rx_broadcast", MetricTypeCounter},
		{"tx_unicast", MetricTypeCounter},
		{"lan_unicast_pkt_rx_count", MetricTypeCounter},
		{"lan_unicast_pkt_tx_count", MetricTypeCounter},
		{"cpu_pkg_energy", MetricTypeCounter},
		{"dram_pkg_energy", MetricTypeCounter},
		{"address_parity_error", MetricTypeCounter},
		{"correctable_ecc_error", MetricTypeCounter},
		{"limiting_events", MetricTypeCounter},
		{"acc_core_cycles", MetricTypeCounter},
		{"acc_core_cycles_total", MetricTypeCounter},

		// Previously missed bare forms, now covered by token matching.
		{"energy_consumption", MetricTypeCounter},
		{"bytes_read", MetricTypeCounter},
		{"packets_received", MetricTypeCounter},
		{"energyk_wh", MetricTypeCounter},

		// Trailing gauge qualifiers win over counter-looking tokens.
		{"bytes_free", MetricTypeGauge},
		{"bytes_per_second", MetricTypeGauge},
		{"memory_available", MetricTypeGauge},
		{"energy_percent", MetricTypeGauge},
		{"read_error_rate", MetricTypeGauge},
		{"power_consumed_watts", MetricTypeGauge},

		// Gauges (fallback).
		{"temperature_reading", MetricTypeGauge},
		{"fan_rpm", MetricTypeGauge},
		{"average_consumed_watts", MetricTypeGauge},
		{"percent", MetricTypeGauge},
		{"", MetricTypeGauge},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := ClassifyMetric(c.in); got != c.expected {
				t.Fatalf("expected %q for %q, got %q", c.expected, c.in, got)
			}
		})
	}
}

func TestMetricTypeValueType(t *testing.T) {
	cases := []struct {
		in       MetricType
		expected prometheus.ValueType
	}{
		{MetricTypeCounter, prometheus.CounterValue},
		{MetricTypeGauge, prometheus.GaugeValue},
		{MetricTypeTimestamp, prometheus.GaugeValue},
	}

	for _, c := range cases {
		t.Run(string(c.in), func(t *testing.T) {
			if got := c.in.ValueType(); got != c.expected {
				t.Fatalf("expected %v for %q, got %v", c.expected, c.in, got)
			}
		})
	}
}

func TestHealthValue(t *testing.T) {
	cases := []struct {
		in       string
		expected int
		ok       bool
	}{
		{"", -1, false},
		{"OK", 0, true},
		{"ok", 0, true},
		{"GoodInUse", 0, true},
		{"Warning", 1, true},
		{"Degraded", 1, true},
		{"Critical", 2, true},
		{"Unknown", 10, true},
		{"Bogus", 10, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := healthValue(c.in)
			if got != c.expected || ok != c.ok {
				t.Fatalf("expected (%d,%v) for %q, got (%d,%v)", c.expected, c.ok, c.in, got, ok)
			}
		})
	}
}

// TestMetricReportSeriesKeyDistinguishesParts verifies that the series key
// cannot be forged by embedding the separator inside a label value.
func TestMetricReportSeriesKeyDistinguishesParts(t *testing.T) {
	a := "a"
	b := "b"
	ab := "a|b"
	empty := ""

	// ("a", "b") must not collide with ("a|b", "").
	k1 := metricReportSeriesKey("report", "Metric", &a, &b)
	k2 := metricReportSeriesKey("report", "Metric", &ab, &empty)
	if k1 == k2 {
		t.Fatalf("series keys collided: %q", k1)
	}

	// nil and empty must normalize to the same identity.
	k3 := metricReportSeriesKey("report", "Metric", nil, nil)
	k4 := metricReportSeriesKey("report", "Metric", &empty, &empty)
	if k3 != k4 {
		t.Fatalf("nil and empty context should share identity: %q vs %q", k3, k4)
	}

	// The report ID must participate in the identity.
	k5 := metricReportSeriesKey("other", "Metric", nil, nil)
	if k3 == k5 {
		t.Fatal("metric_report_id must be part of the series identity")
	}
}

func TestPreferMetricReportEntry(t *testing.T) {
	older := metricReportEntry{ts: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), hasTS: true}
	newer := metricReportEntry{ts: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), hasTS: true}
	noTS := metricReportEntry{}

	cases := []struct {
		name      string
		existing  metricReportEntry
		candidate metricReportEntry
		expected  bool
	}{
		{"newer beats older", older, newer, true},
		{"older loses to newer", newer, older, false},
		{"tie keeps first", older, older, false},
		{"valid beats invalid", noTS, older, true},
		{"invalid loses to valid", older, noTS, false},
		{"invalid vs invalid keeps first", noTS, noTS, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := preferMetricReportEntry(c.existing, c.candidate); got != c.expected {
				t.Fatalf("expected %v, got %v", c.expected, got)
			}
		})
	}
}

func TestParseMetricTimestamp(t *testing.T) {
	if _, ok := parseMetricTimestamp(""); ok {
		t.Fatal("empty timestamp should be invalid")
	}
	if _, ok := parseMetricTimestamp("not-a-timestamp"); ok {
		t.Fatal("garbage timestamp should be invalid")
	}
	ts, ok := parseMetricTimestamp("2026-04-01T01:45:35.000Z")
	if !ok {
		t.Fatal("expected valid timestamp")
	}
	if ts.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

// TestParseMetricValue_AnchoredStatusMatching verifies that status matching is
// anchored to the leading word, so unrelated values are not misclassified.
func TestParseMetricValue_AnchoredStatusMatching(t *testing.T) {
	// These previously matched via substring/prefix checks and were silently
	// mapped to 1 ("set"/"on") or 0 ("ok"/"off").
	for _, in := range []string{"offset", "reset", "unset", "broken", "onset"} {
		t.Run(in, func(t *testing.T) {
			if _, err := parseMetricValue(MetricReportValue{MetricValue: in}); err == nil {
				t.Fatalf("expected error for %q, got a numeric mapping", in)
			}
		})
	}

	// Suffixed forms must still resolve via their leading word.
	cases := []struct {
		in       string
		expected float64
	}{
		{"OK (some info)", 0},
		{"Degraded: reason", 1},
		{"Critical - fan failure", 2},
		{"Unknown; no data", 10},
		{"Online", 1},
		{"Offline", 0},
		{"Enabled", 1},
		{"Disabled", 0},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseMetricValue(MetricReportValue{MetricValue: c.in})
			if err != nil {
				t.Fatalf("parse error for %q: %v", c.in, err)
			}
			if got != c.expected {
				t.Fatalf("expected %v for %q, got %v", c.expected, c.in, got)
			}
		})
	}
}

// telemetryTestCollector emits telemetry metrics through the same claim guard
// used by RefreshMetricReport, so tests can exercise real registry gathering.
type telemetryTestCollector struct {
	mc      *Collector
	entries []struct {
		reportID string
		metricId string
		ctx      *string
		fqdd     *string
		value    float64
	}
}

func (c *telemetryTestCollector) Describe(ch chan<- *prometheus.Desc) {}

func (c *telemetryTestCollector) Collect(ch chan<- prometheus.Metric) {
	for _, e := range c.entries {
		key := metricReportSeriesKey(e.reportID, e.metricId, e.ctx, e.fqdd)
		if !c.mc.claimMetricReportSeries(key) {
			continue
		}
		if err := c.mc.NewMetricReportEntry(ch, e.reportID, e.metricId, e.ctx, e.fqdd, e.value); err != nil {
			panic(err)
		}
	}
}

// TestDuplicateEmittedIdentityGathers verifies that two entries resolving to
// the same emitted series do not make registry.Gather() fail.
func TestDuplicateEmittedIdentityGathers(t *testing.T) {
	if config.Config == nil {
		config.SetConfig(config.NewConfig())
	}
	old := config.Config.MetricsPrefix
	config.Config.MetricsPrefix = "idrac"
	defer func() { config.Config.MetricsPrefix = old }()

	ctx := "CPU.Socket.1"
	fqdd := "CPU.Socket.1"

	mc := &Collector{emitted: make(map[string]struct{})}
	tc := &telemetryTestCollector{mc: mc}
	tc.entries = append(tc.entries,
		struct {
			reportID string
			metricId string
			ctx      *string
			fqdd     *string
			value    float64
		}{"ThermalSensor", "TemperatureReading", &ctx, &fqdd, 50},
		struct {
			reportID string
			metricId string
			ctx      *string
			fqdd     *string
			value    float64
		}{"ThermalSensor", "temperature_reading", &ctx, &fqdd, 51},
	)

	reg := prometheus.NewRegistry()
	if err := reg.Register(tc); err != nil {
		t.Fatalf("register: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather must not fail on duplicate identity: %v", err)
	}
	if len(families) != 1 {
		t.Fatalf("expected 1 metric family, got %d", len(families))
	}
	if n := len(families[0].GetMetric()); n != 1 {
		t.Fatalf("expected 1 series, got %d", n)
	}
}

// TestClaimMetricReportSeries verifies the guard semantics and that resetting
// allows the same series to be claimed again on the next scrape.
func TestClaimMetricReportSeries(t *testing.T) {
	mc := &Collector{emitted: make(map[string]struct{})}

	if !mc.claimMetricReportSeries("k") {
		t.Fatal("first claim should succeed")
	}
	if mc.claimMetricReportSeries("k") {
		t.Fatal("second claim of the same key should fail")
	}
	if !mc.claimMetricReportSeries("other") {
		t.Fatal("claim of a different key should succeed")
	}

	mc.resetMetricReportSeries()
	if !mc.claimMetricReportSeries("k") {
		t.Fatal("claim after reset should succeed")
	}
}
