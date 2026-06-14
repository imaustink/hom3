package model_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/imaustink/hom3/internal/model"
)

// TestDeviceTypesConformance asserts DEVICE_TYPE_DOMAINS and DEVICE_TYPE_SHORTCUTS
// exactly match the spec/fixtures/device-types.json corpus.
func TestDeviceTypesConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/device-types.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var fixture struct {
		DeviceTypeDomains   map[string][]string `json:"deviceTypeDomains"`
		DeviceTypeShortcuts map[string]string   `json:"deviceTypeShortcuts"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	// Check DeviceTypeDomains
	for key, wantDomains := range fixture.DeviceTypeDomains {
		dt := model.DeviceType(key)
		got, ok := model.DeviceTypeDomains[dt]
		if !ok {
			t.Errorf("DeviceTypeDomains missing key %q", key)
			continue
		}
		if !reflect.DeepEqual(got, wantDomains) {
			t.Errorf("DeviceTypeDomains[%q] = %v; want %v", key, got, wantDomains)
		}
	}
	// Check no extra keys
	for dt := range model.DeviceTypeDomains {
		if _, ok := fixture.DeviceTypeDomains[string(dt)]; !ok {
			t.Errorf("DeviceTypeDomains has unexpected key %q", dt)
		}
	}

	// Check DeviceTypeShortcuts
	for alias, wantTarget := range fixture.DeviceTypeShortcuts {
		got, ok := model.DeviceTypeShortcuts[alias]
		if !ok {
			t.Errorf("DeviceTypeShortcuts missing alias %q", alias)
			continue
		}
		if string(got) != wantTarget {
			t.Errorf("DeviceTypeShortcuts[%q] = %q; want %q", alias, got, wantTarget)
		}
	}
	for alias := range model.DeviceTypeShortcuts {
		if _, ok := fixture.DeviceTypeShortcuts[alias]; !ok {
			t.Errorf("DeviceTypeShortcuts has unexpected alias %q", alias)
		}
	}
}
