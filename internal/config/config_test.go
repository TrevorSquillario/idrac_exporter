package config

import (
	"reflect"
	"testing"
)

func TestValidateTelemetryReports(t *testing.T) {
	cases := []struct {
		name     string
		in       []string
		expected []string
	}{
		{
			name:     "nil stays nil",
			in:       nil,
			expected: nil,
		},
		{
			name:     "duplicates collapsed preserving order",
			in:       []string{"ThermalSensor", "ThermalSensor", "StorageDiskSMARTData"},
			expected: []string{"ThermalSensor", "StorageDiskSMARTData"},
		},
		{
			name:     "entries trimmed",
			in:       []string{" ThermalSensor ", "StorageDiskSMARTData"},
			expected: []string{"ThermalSensor", "StorageDiskSMARTData"},
		},
		{
			name:     "empty entries dropped",
			in:       []string{"", "ThermalSensor", "   ", ""},
			expected: []string{"ThermalSensor"},
		},
		{
			name:     "trimmed duplicates collapsed",
			in:       []string{"ThermalSensor", " ThermalSensor "},
			expected: []string{"ThermalSensor"},
		},
		{
			name:     "all empty yields empty slice",
			in:       []string{"", "  "},
			expected: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.TelemetryReports = c.in

			if err := cfg.Validate(); err != nil {
				t.Fatalf("validate: %v", err)
			}

			if !reflect.DeepEqual(cfg.TelemetryReports, c.expected) {
				t.Fatalf("expected %#v, got %#v", c.expected, cfg.TelemetryReports)
			}
		})
	}
}

func TestValidateDefaults(t *testing.T) {
	cfg := NewConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if cfg.MetricsPrefix != "idrac" {
		t.Fatalf("expected default metrics prefix, got %q", cfg.MetricsPrefix)
	}
	if cfg.Port != 9348 {
		t.Fatalf("expected default port, got %d", cfg.Port)
	}
}
