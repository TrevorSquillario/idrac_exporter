package collector

import (
	"encoding/json"
	"testing"
)

func TestEnvironmentMetrics_DellAverageFanPWM(t *testing.T) {
	data := `{
		"@odata.context": "/redfish/v1/$metadata#EnvironmentMetrics.EnvironmentMetrics",
		"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/EnvironmentMetrics",
		"@odata.type": "#EnvironmentMetrics.v1_3_2.EnvironmentMetrics",
		"Name": "EnvironmentMetrics",
		"Id": "EnvironmentMetrics",
		"Description": "Represents EnvironmentMetrics of the Chassis",
		"PowerWatts": {
			"DataSourceUri": "/redfish/v1/Chassis/System.Embedded.1/Sensors/SystemBoardPwrConsumption",
			"Reading": 200.0
		},
		"Oem": {
			"Dell": {
				"AverageFanPWM": 17.0,
				"@odata.type": "#DellEnvironmentMetrics.v1_1_0.DellEnvironmentMetrics"
			}
		}
	}`

	var resp EnvironmentMetrics
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.OdataId != "/redfish/v1/Chassis/System.Embedded.1/EnvironmentMetrics" {
		t.Fatalf("unexpected OdataId: %q", resp.OdataId)
	}

	if resp.Oem.Dell == nil {
		t.Fatalf("expected Dell OEM extension to be present")
	}

	v, ok := asFloat64(resp.Oem.Dell.AverageFanPWM)
	if !ok {
		t.Fatalf("expected AverageFanPWM to be coercible to float64")
	}
	if v != 17 {
		t.Fatalf("expected 17, got %v", v)
	}
}

func TestEnvironmentMetrics_NonDell(t *testing.T) {
	// A non-Dell BMC exposes the standard resource without the Dell OEM
	// extension. The Dell pointer must stay nil so the metric is skipped.
	data := `{
		"@odata.id": "/redfish/v1/Chassis/1/EnvironmentMetrics",
		"Id": "EnvironmentMetrics",
		"PowerWatts": {"Reading": 200.0}
	}`

	var resp EnvironmentMetrics
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Oem.Dell != nil {
		t.Fatalf("expected Dell OEM extension to be absent, got %#v", resp.Oem.Dell)
	}
}

func TestEnvironmentMetrics_AverageFanPWMAsString(t *testing.T) {
	// Some firmware revisions serialize numeric readings as JSON strings.
	data := `{"Oem":{"Dell":{"AverageFanPWM":"17"}}}`

	var resp EnvironmentMetrics
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Oem.Dell == nil {
		t.Fatalf("expected Dell OEM extension to be present")
	}

	v, ok := asFloat64(resp.Oem.Dell.AverageFanPWM)
	if !ok {
		t.Fatalf("expected string AverageFanPWM to be coercible to float64")
	}
	if v != 17 {
		t.Fatalf("expected 17, got %v", v)
	}
}

func TestEnvironmentMetrics_AverageFanPWMAbsent(t *testing.T) {
	// The Dell extension may exist without the AverageFanPWM property. This
	// must be distinguishable from a legitimate reading of zero.
	data := `{"Oem":{"Dell":{"@odata.type":"#DellEnvironmentMetrics.v1_1_0.DellEnvironmentMetrics"}}}`

	var resp EnvironmentMetrics
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Oem.Dell == nil {
		t.Fatalf("expected Dell OEM extension to be present")
	}

	if _, ok := asFloat64(resp.Oem.Dell.AverageFanPWM); ok {
		t.Fatalf("expected absent AverageFanPWM to be non-coercible")
	}
}

func TestEnvironmentMetrics_AverageFanPWMZero(t *testing.T) {
	// Zero is a valid PWM reading and must be coercible.
	data := `{"Oem":{"Dell":{"AverageFanPWM":0}}}`

	var resp EnvironmentMetrics
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	v, ok := asFloat64(resp.Oem.Dell.AverageFanPWM)
	if !ok {
		t.Fatalf("expected zero AverageFanPWM to be coercible")
	}
	if v != 0 {
		t.Fatalf("expected 0, got %v", v)
	}
}
