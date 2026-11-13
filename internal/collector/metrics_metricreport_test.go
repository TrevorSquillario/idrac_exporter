package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

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

			mc.NewMetricReportEntry(ch, resp.Id, mv.MetricId, ctxPtr, fqPtr, v)

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
		{"Critical", 3},
		{"Unknown", 4},
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
		mc.NewMetricReportEntry(ch, "MetricReport", id, nil, nil, 42)

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
