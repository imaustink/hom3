package color_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/imaustink/hom3/internal/color"
	"github.com/imaustink/hom3/internal/model"
)

// ─── state-color ────────────────────────────────────────────────

func TestStateColorConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/state-color.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Cases []struct {
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		got := color.StateColor(tc.Input)
		if got != tc.Expect {
			t.Errorf("StateColor(%q) = %q; want %q", tc.Input, got, tc.Expect)
		}
	}
}

// ─── domain-icon ────────────────────────────────────────────────

func TestDomainIconConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/domain-icon.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Cases []struct {
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		got := color.DomainIcon(tc.Input)
		if got != tc.Expect {
			t.Errorf("DomainIcon(%q) = %q; want %q", tc.Input, got, tc.Expect)
		}
	}
}

// ─── friendly-name ──────────────────────────────────────────────

func TestFriendlyNameConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/friendly-name.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Cases []struct {
			Entity struct {
				EntityID   string                 `json:"entity_id"`
				Attributes map[string]interface{} `json:"attributes"`
			} `json:"entity"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		e := model.HassEntity{
			EntityID:   tc.Entity.EntityID,
			Attributes: tc.Entity.Attributes,
		}
		got := color.FriendlyName(e)
		if got != tc.Expect {
			t.Errorf("FriendlyName(%q) = %q; want %q", tc.Entity.EntityID, got, tc.Expect)
		}
	}
}

// ─── format-state ───────────────────────────────────────────────

func TestFormatStateConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/format-state.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Cases []struct {
			Entity struct {
				EntityID   string                 `json:"entity_id"`
				State      string                 `json:"state"`
				Attributes map[string]interface{} `json:"attributes"`
			} `json:"entity"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		e := model.HassEntity{
			EntityID:   tc.Entity.EntityID,
			State:      tc.Entity.State,
			Attributes: tc.Entity.Attributes,
		}
		got := color.FormatState(e)
		if got != tc.Expect {
			t.Errorf("FormatState(%q state=%q) = %q; want %q",
				tc.Entity.EntityID, tc.Entity.State, got, tc.Expect)
		}
	}
}

// ─── time-since ─────────────────────────────────────────────────

func TestTimeSinceConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/time-since.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Now   string `json:"now"`
		Cases []struct {
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	now, err := time.Parse(time.RFC3339Nano, fixture.Now)
	if err != nil {
		t.Fatalf("parse now: %v", err)
	}
	for _, tc := range fixture.Cases {
		got := color.TimeSinceFrom(tc.Input, now)
		if got != tc.Expect {
			t.Errorf("TimeSinceFrom(%q, now) = %q; want %q", tc.Input, got, tc.Expect)
		}
	}
}
