package client_test

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/imaustink/hom3/internal/client"
	"github.com/imaustink/hom3/internal/model"
)

// capturedCall records a single service call made through the mock sendFn.
type capturedCall struct {
	Domain      string                 `json:"domain"`
	Service     string                 `json:"service"`
	ServiceData map[string]interface{} `json:"service_data,omitempty"`
}

// newTestClient returns a HassClient with a mock sendFn that captures calls.
func newTestClient() (*client.HassClient, *[]capturedCall) {
	calls := make([]capturedCall, 0)
	c := client.NewTestable(func(domain, service string, sd, _ map[string]interface{}) error {
		calls = append(calls, capturedCall{Domain: domain, Service: service, ServiceData: sd})
		return nil
	})
	return c, &calls
}

// loadFixture parses the service-calls JSON fixture.
func loadFixture(t *testing.T) serviceCallsFixture {
	t.Helper()
	data, err := os.ReadFile("../../spec/fixtures/service-calls.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f serviceCallsFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return f
}

type serviceCallsFixture struct {
	Cases []struct {
		Name          string                 `json:"name"`
		Method        string                 `json:"method"`
		Args          []json.RawMessage      `json:"args"`
		Entity        *model.HassEntity      `json:"entity"`
		ExpectCall    *expectedCall          `json:"expectCall"`
		ExpectCalls   []expectedCall         `json:"expectCalls"`
		ExpectReturn  *json.RawMessage       `json:"expectReturn"`
	} `json:"cases"`
}

type expectedCall struct {
	Domain      string                 `json:"domain"`
	Service     string                 `json:"service"`
	ServiceData map[string]interface{} `json:"service_data"`
}

func TestServiceCallsConformance(t *testing.T) {
	fixture := loadFixture(t)

	for _, tc := range fixture.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			c, calls := newTestClient()

			// Seed entity if provided
			if tc.Entity != nil {
				c.SetEntity(tc.Entity)
			}

			// Dispatch method
			var invokeErr error
			switch tc.Method {
			case "toggleEntity":
				entityID := mustString(tc.Args[0])
				invokeErr = c.ToggleEntity(entityID)

			case "activateEntity":
				entityID := mustString(tc.Args[0])
				_, invokeErr = c.ActivateEntity(entityID)

			case "adjustBrightness":
				entityID := mustString(tc.Args[0])
				delta := mustInt(tc.Args[1])
				_, invokeErr = c.AdjustBrightness(entityID, delta)

			case "adjustTemperature":
				entityID := mustString(tc.Args[0])
				delta := mustFloat64(tc.Args[1])
				invokeErr = c.AdjustTemperature(entityID, delta)

			case "cycleHvacMode":
				entityID := mustString(tc.Args[0])
				invokeErr = c.CycleHvacMode(entityID)

			case "controlCover":
				entityID := mustString(tc.Args[0])
				action := mustString(tc.Args[1])
				invokeErr = c.ControlCover(entityID, action)

			case "adjustFanSpeed":
				entityID := mustString(tc.Args[0])
				dir := mustInt(tc.Args[1])
				_, invokeErr = c.AdjustFanSpeed(entityID, dir)

			case "adjustVolume":
				entityID := mustString(tc.Args[0])
				delta := mustFloat64(tc.Args[1])
				invokeErr = c.AdjustVolume(entityID, delta)

			case "mediaPlayerCommand":
				entityID := mustString(tc.Args[0])
				cmd := mustString(tc.Args[1])
				invokeErr = c.MediaPlayerCommand(entityID, cmd)

			case "vacuumCommand":
				entityID := mustString(tc.Args[0])
				cmd := mustString(tc.Args[1])
				invokeErr = c.VacuumCommand(entityID, cmd)

			case "alarmControl":
				entityID := mustString(tc.Args[0])
				action := mustString(tc.Args[1])
				code := ""
				if len(tc.Args) > 2 {
					code = mustString(tc.Args[2])
				}
				invokeErr = c.AlarmControl(entityID, action, code)

			case "adjustNumber":
				entityID := mustString(tc.Args[0])
				dir := mustInt(tc.Args[1])
				invokeErr = c.AdjustNumber(entityID, dir)

			case "cycleSelectOption":
				entityID := mustString(tc.Args[0])
				dir := mustInt(tc.Args[1])
				invokeErr = c.CycleSelectOption(entityID, dir)

			case "bulkPower":
				var ids []string
				json.Unmarshal(tc.Args[0], &ids)
				action := mustString(tc.Args[1])
				invokeErr = c.BulkPower(ids, action)

			default:
				t.Skipf("unknown method %q", tc.Method)
			}

			if invokeErr != nil {
				t.Fatalf("invoke error: %v", invokeErr)
			}

			// Validate expectCall (single)
			if tc.ExpectCall != nil {
				if len(*calls) == 0 {
					t.Errorf("expected call {%s.%s} but no calls were made", tc.ExpectCall.Domain, tc.ExpectCall.Service)
					return
				}
				got := (*calls)[0]
				assertCall(t, got, *tc.ExpectCall)
			} else if tc.ExpectCalls == nil {
				// expectCall: null → no calls expected
				if len(*calls) != 0 {
					t.Errorf("expected no calls but got %d: %+v", len(*calls), *calls)
				}
			}

			// Validate expectCalls (bulk)
			if tc.ExpectCalls != nil {
				if len(*calls) != len(tc.ExpectCalls) {
					t.Errorf("expected %d calls, got %d: %+v", len(tc.ExpectCalls), len(*calls), *calls)
					return
				}
				for i, want := range tc.ExpectCalls {
					assertCall(t, (*calls)[i], want)
				}
			}
		})
	}
}

func assertCall(t *testing.T, got capturedCall, want expectedCall) {
	t.Helper()
	if got.Domain != want.Domain || got.Service != want.Service {
		t.Errorf("call = {%s.%s}; want {%s.%s}", got.Domain, got.Service, want.Domain, want.Service)
		return
	}
	if !serviceDataEqual(got.ServiceData, want.ServiceData) {
		t.Errorf("service_data mismatch\n  got:  %v\n  want: %v", got.ServiceData, want.ServiceData)
	}
}

// serviceDataEqual compares service_data maps, handling float rounding.
func serviceDataEqual(got, want map[string]interface{}) bool {
	if len(got) != len(want) {
		return false
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		if !valEqual(gv, wv) {
			return false
		}
	}
	return true
}

func valEqual(a, b interface{}) bool {
	// Both nil
	if a == nil && b == nil {
		return true
	}
	// String slice vs string slice
	if sa, ok := toStringSlice(a); ok {
		if sb, ok2 := toStringSlice(b); ok2 {
			return reflect.DeepEqual(sa, sb)
		}
	}
	// Numeric — compare with tolerance
	af, aok := toF64(a)
	bf, bok := toF64(b)
	if aok && bok {
		return math.Abs(af-bf) < 1e-9
	}
	return reflect.DeepEqual(a, b)
}

func toStringSlice(v interface{}) ([]string, bool) {
	switch s := v.(type) {
	case []string:
		return s, true
	case []interface{}:
		out := make([]string, len(s))
		for i, x := range s {
			str, ok := x.(string)
			if !ok {
				return nil, false
			}
			out[i] = str
		}
		return out, true
	}
	return nil, false
}

func toF64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func mustString(raw json.RawMessage) string {
	var s string
	json.Unmarshal(raw, &s)
	return s
}

func mustInt(raw json.RawMessage) int {
	var f float64
	json.Unmarshal(raw, &f)
	return int(f)
}

func mustFloat64(raw json.RawMessage) float64 {
	var f float64
	json.Unmarshal(raw, &f)
	return f
}
