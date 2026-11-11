package collector

import (
	"encoding/json"
	"fmt"
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
		expectedLabel string
	}{
		{
			name:          "AverageConsumedWatts_string",
			data:          `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":"507","Timestamp":"2025-06-23T19:06:14Z"}]}`,
			expectedValue: 507,
			expectedCtx:   "",
			expectedFqdd:  "",
			expectedLabel: "",
		},
		{
			name:          "AverageConsumedWatts_number",
			data:          `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":507,"Timestamp":"2025-06-23T19:06:14Z"}]}`,
			expectedValue: 507,
			expectedCtx:   "",
			expectedFqdd:  "",
			expectedLabel: "",
		},
		{
			name:          "TemperatureReading",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"TemperatureReading","MetricProperty":"","MetricValue":"50","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"Embedded NIC 1 Port 1 Partition 1","FQDD":"NIC.Embedded.1-1-1","Label":"Embedded NIC 1 Port 1 Partition 1 TemperatureReading","Source":"hiitempsensors"}},"Timestamp":"2026-04-01T01:45:35.000Z"}]}`,
			expectedValue: 50,
			expectedCtx:   "Embedded NIC 1 Port 1 Partition 1",
			expectedFqdd:  "NIC.Embedded.1-1-1",
			expectedLabel: "Embedded NIC 1 Port 1 Partition 1 TemperatureReading",
		},
		{
			name:          "AddressParityError_bool_string",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"AddressParityError","MetricProperty":"/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.Socket.A1/MemoryMetrics#HealthData/AlarmTrips/AddressParityError","MetricValue":"false","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"DIMM A1","FQDD":"DIMM.Socket.A1","Label":"DIMM A1 AddressParityError","Source":"memorymetrics"}},"Timestamp":"2026-04-01T01:44:50.000Z"}]}`,
			expectedValue: 0,
			expectedCtx:   "DIMM A1",
			expectedFqdd:  "DIMM.Socket.A1",
			expectedLabel: "DIMM A1 AddressParityError",
		},
		{
			name:          "ReadErrorRate",
			data:          `{"Id":"MetricReport","MetricValues":[{"MetricId":"ReadErrorRate","MetricProperty":"","MetricValue":"0","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","FQDD":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","Label":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1 ReadErrorRate","Source":"smart-sas"}},"Timestamp":"2026-04-01T00:52:09.000Z"}]}`,
			expectedValue: 0,
			expectedCtx:   "SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1",
			expectedFqdd:  "SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1",
			expectedLabel: "SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1 ReadErrorRate",
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
			config.Config.MetricsPrefix = ""
			defer func() { config.Config.MetricsPrefix = old }()

			ch := make(chan prometheus.Metric, 1)
			var mc Collector

			// build pointers for context/fqdd/label if present
			var ctxPtr, fqPtr, labPtr *string
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
				if mv.Oem.Dell.Label != "" {
					labPtr = &mv.Oem.Dell.Label
				}
			}

			mc.NewMetricReportEntry(ch, resp.Id, mv.MetricId, ctxPtr, fqPtr, labPtr, v)

			select {
			case m := <-ch:
				dtoMetric := &dto.Metric{}
				if err := m.Write(dtoMetric); err != nil {
					t.Fatalf("metric write: %v", err)
				}
				if dtoMetric.GetGauge() == nil {
					t.Fatalf("expected gauge metric")
				}
				if dtoMetric.GetGauge().GetValue() != c.expectedValue {
					t.Fatalf("expected value %v, got %v", c.expectedValue, dtoMetric.GetGauge().GetValue())
				}
				labels := dtoMetric.GetLabel()
				if len(labels) != 4 {
					t.Fatalf("expected 4 labels, got %d", len(labels))
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
				if labmap["label"] != c.expectedLabel {
					t.Fatalf("label label mismatch: expected %q got %q", c.expectedLabel, labmap["label"])
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
