package render_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/imaustink/hom3/internal/model"
	"github.com/imaustink/hom3/internal/render"
)

// ─── filter-entities ────────────────────────────────────────────

func TestFilterEntitiesConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/filter-entities.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Entities []model.HassEntity      `json:"entities"`
		AreaMap  map[string]string        `json:"areaMap"`
		Cases    []struct {
			View       string   `json:"view"`
			Filter     string   `json:"filter"`
			AreaFilter string   `json:"areaFilter"`
			Expect     []string `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, tc := range fixture.Cases {
		got := render.FilterEntities(
			fixture.Entities,
			model.DeviceType(tc.View),
			tc.Filter,
			fixture.AreaMap,
			tc.AreaFilter,
		)
		ids := make([]string, len(got))
		for i, e := range got {
			ids[i] = e.EntityID
		}
		want := tc.Expect
		if want == nil {
			want = []string{}
		}
		if len(ids) == 0 && len(want) == 0 {
			continue
		}
		if !slicesEqual(ids, want) {
			t.Errorf("FilterEntities(view=%q filter=%q area=%q) = %v; want %v",
				tc.View, tc.Filter, tc.AreaFilter, ids, want)
		}
	}
}

// ─── autocomplete ───────────────────────────────────────────────

func TestComputeCommandSuggestionsConformance(t *testing.T) {
	data, err := os.ReadFile("../../spec/fixtures/autocomplete.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Command struct {
			Areas []model.HassArea    `json:"areas"`
			Homes []model.HassConfig  `json:"homes"`
			Cases []struct {
				Buffer string   `json:"buffer"`
				Expect []string `json:"expect"`
			} `json:"cases"`
		} `json:"command"`
		Area struct {
			Areas []model.HassArea `json:"areas"`
			Cases []struct {
				Buffer string   `json:"buffer"`
				Expect []string `json:"expect"`
			} `json:"cases"`
		} `json:"area"`
		Filter struct {
			Entities []model.HassEntity `json:"entities"`
			Cases    []struct {
				Query  string   `json:"query"`
				Expect []string `json:"expect"`
			} `json:"cases"`
		} `json:"filter"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	// Command suggestions
	for _, tc := range fixture.Command.Cases {
		got := render.ComputeCommandSuggestions(tc.Buffer, fixture.Command.Areas, fixture.Command.Homes)
		want := tc.Expect
		if !slicesEqual(got, want) {
			t.Errorf("ComputeCommandSuggestions(%q) = %v; want %v", tc.Buffer, got, want)
		}
	}

	// Area suggestions
	for _, tc := range fixture.Area.Cases {
		got := render.ComputeAreaSuggestions(tc.Buffer, fixture.Area.Areas)
		want := tc.Expect
		if !slicesEqual(got, want) {
			t.Errorf("ComputeAreaSuggestions(%q) = %v; want %v", tc.Buffer, got, want)
		}
	}

	// Filter suggestions
	for _, tc := range fixture.Filter.Cases {
		got := render.ComputeFilterSuggestions(tc.Query, fixture.Filter.Entities, nil)
		want := tc.Expect
		if !slicesEqual(got, want) {
			t.Errorf("ComputeFilterSuggestions(%q) = %v; want %v", tc.Query, got, want)
		}
	}
}

// slicesEqual compares two string slices, treating nil and [] as equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
