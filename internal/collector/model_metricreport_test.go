package collector

import (
	"encoding/json"
	"testing"
)

func TestParseMetricValue_String(t *testing.T) {
	data := `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":"507","Timestamp":"2025-06-23T19:06:14Z"}]}`
	var resp MetricReportResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}
	v, err := parseMetricValue(resp.MetricValues[0])
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 507 {
		t.Fatalf("expected 507, got %v", v)
	}
}

func TestParseMetricValue_Number(t *testing.T) {
	data := `{"Id":"PowerMetrics","MetricValues":[{"MetricId":"AverageConsumedWatts","MetricValue":507,"Timestamp":"2025-06-23T19:06:14Z"}]}`
	var resp MetricReportResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}
	v, err := parseMetricValue(resp.MetricValues[0])
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 507 {
		t.Fatalf("expected 507, got %v", v)
	}
}

func TestParseMetricValue_TemperatureReading(t *testing.T) {
	data := `{"Id":"MetricReport","MetricValues":[{"MetricId":"TemperatureReading","MetricProperty":"","MetricValue":"50","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"Embedded NIC 1 Port 1 Partition 1","FQDD":"NIC.Embedded.1-1-1","Label":"Embedded NIC 1 Port 1 Partition 1 TemperatureReading","Source":"hiitempsensors"}},"Timestamp":"2026-04-01T01:45:35.000Z"}]}`
	var resp MetricReportResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}
	mv := resp.MetricValues[0]
	if mv.MetricId != "TemperatureReading" {
		t.Fatalf("unexpected MetricId: %s", mv.MetricId)
	}
	if mv.Oem == nil || mv.Oem.Dell == nil || mv.Oem.Dell.ContextID != "Embedded NIC 1 Port 1 Partition 1" {
		t.Fatalf("unexpected or missing Dell ContextID: %#v", mv.Oem)
	}
	if mv.MetricProperty == nil || *mv.MetricProperty != "" {
		t.Fatalf("expected MetricProperty pointer to exist and be empty string")
	}
	v, err := parseMetricValue(mv)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 50 {
		t.Fatalf("expected 50, got %v", v)
	}
}

func TestParseMetricValue_AddressParityError(t *testing.T) {
	data := `{"Id":"MetricReport","MetricValues":[{"MetricId":"AddressParityError","MetricProperty":"/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.Socket.A1/MemoryMetrics#HealthData/AlarmTrips/AddressParityError","MetricValue":"false","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"DIMM A1","FQDD":"DIMM.Socket.A1","Label":"DIMM A1 AddressParityError","Source":"memorymetrics"}},"Timestamp":"2026-04-01T01:44:50.000Z"}]}`
	var resp MetricReportResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}
	mv := resp.MetricValues[0]
	if mv.MetricId != "AddressParityError" {
		t.Fatalf("unexpected MetricId: %s", mv.MetricId)
	}
	if mv.Oem == nil || mv.Oem.Dell == nil || mv.Oem.Dell.ContextID != "DIMM A1" {
		t.Fatalf("unexpected or missing Dell ContextID: %#v", mv.Oem)
	}
	if mv.MetricProperty == nil || *mv.MetricProperty == "" {
		t.Fatalf("expected MetricProperty pointer to exist and be non-empty")
	}
	// boolean-like strings are normalized to 0/1
	v, err := parseMetricValue(mv)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0 for 'false', got %v", v)
	}
}

func TestParseMetricValue_ReadErrorRate(t *testing.T) {
	data := `{"Id":"MetricReport","MetricValues":[{"MetricId":"ReadErrorRate","MetricProperty":"","MetricValue":"0","Oem":{"Dell":{"@odata.type":"#DellMetricReportValue.v1_0_1.DellMetricReportValue","ContextID":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","FQDD":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1","Label":"SMARTData:Disk.Bay.0:Enclosure.Internal.0-1:RAID.SL.1-1 ReadErrorRate","Source":"smart-sas"}},"Timestamp":"2026-04-01T00:52:09.000Z"}]}`
	var resp MetricReportResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.MetricValues) != 1 {
		t.Fatalf("expected 1 metric value, got %d", len(resp.MetricValues))
	}
	mv := resp.MetricValues[0]
	if mv.MetricId != "ReadErrorRate" {
		t.Fatalf("unexpected MetricId: %s", mv.MetricId)
	}
	if mv.Oem == nil || mv.Oem.Dell == nil || mv.Oem.Dell.Source != "smart-sas" {
		t.Fatalf("unexpected or missing Dell Source: %#v", mv.Oem)
	}
	if mv.MetricProperty == nil || *mv.MetricProperty != "" {
		t.Fatalf("expected MetricProperty pointer to exist and be empty string")
	}
	v, err := parseMetricValue(mv)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0, got %v", v)
	}
}

func TestParseMetricValue_StatusStrings(t *testing.T) {
	cases := []struct {
		value    string
		expected float64
	}{
		{"OK", 0},
		{"ok", 0},
		{"Degraded", 1},
		{"Critical", 3},
		{"Unknown", 4},
		{"true", 1},
		{"false", 0},
		{"on", 1},
		{"off", 0},
		{"released", 0},
		{"set", 1},
		{" 42 ", 42},
	}
	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			v, err := parseMetricValue(MetricReportValue{MetricValue: c.value})
			if err != nil {
				t.Fatalf("parse error for %q: %v", c.value, err)
			}
			if v != c.expected {
				t.Fatalf("expected %v for %q, got %v", c.expected, c.value, v)
			}
		})
	}
}

func TestParseMetricValue_Bool(t *testing.T) {
	v, err := parseMetricValue(MetricReportValue{MetricValue: true})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected 1, got %v", v)
	}

	v, err = parseMetricValue(MetricReportValue{MetricValue: false})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected 0, got %v", v)
	}
}

func TestParseMetricValue_Errors(t *testing.T) {
	if _, err := parseMetricValue(MetricReportValue{MetricValue: nil}); err == nil {
		t.Fatal("expected error for nil metric value")
	}
	if _, err := parseMetricValue(MetricReportValue{MetricValue: ""}); err == nil {
		t.Fatal("expected error for empty string metric value")
	}
	if _, err := parseMetricValue(MetricReportValue{MetricValue: "not-a-number"}); err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}
