// Package render provides pure functions for filtering entities and computing
// autocomplete suggestions.  No I/O; safe to call from tests.
package render

import (
	"fmt"
	"sort"
	"strings"

	clr "github.com/imaustink/hom3/internal/color"
	"github.com/imaustink/hom3/internal/model"
)

// ─────────────────────────────────────────────── Entity filtering ──

// FilterEntities filters entities by view (device-type), free-text filter,
// and optional area filter.  Input order is preserved.
func FilterEntities(
	entities []model.HassEntity,
	view model.DeviceType,
	filter string,
	areaMap map[string]string,
	areaFilter string,
) []model.HassEntity {
	domains := model.DeviceTypeDomains[view]
	domainSet := make(map[string]bool, len(domains))
	for _, d := range domains {
		domainSet[d] = true
	}

	filterLow := strings.ToLower(filter)
	areaFilterLow := strings.ToLower(areaFilter)

	var result []model.HassEntity
	seenName := make(map[string]bool)
	for _, e := range entities {
		// Domain filter (skip when view == "all")
		if len(domains) > 0 {
			if !domainSet[entityDomain(e.EntityID)] {
				continue
			}
		}

		// Area filter — case-insensitive exact match on the resolved area name
		if areaFilterLow != "" {
			area := strings.ToLower(areaMap[e.EntityID])
			if area != areaFilterLow {
				continue
			}
		}

		// Text filter — case-insensitive substring match on entity_id, name, or state
		if filterLow != "" {
			name := strings.ToLower(clr.FriendlyName(e))
			id := strings.ToLower(e.EntityID)
			state := strings.ToLower(e.State)
			if !strings.Contains(id, filterLow) &&
				!strings.Contains(name, filterLow) &&
				!strings.Contains(state, filterLow) {
				continue
			}
		}

		// Deduplicate by friendly name — prefer the entity whose domain matches
		// the view (e.g. light over switch when in lights view), otherwise first seen wins.
		friendlyName := clr.FriendlyName(e)
		if seenName[friendlyName] {
			continue
		}
		seenName[friendlyName] = true

		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(clr.FriendlyName(result[i])) < strings.ToLower(clr.FriendlyName(result[j]))
	})
	return result
}

// ─────────────────────────────────────────────── Autocomplete ──

// contextCommands are prefixes that trigger home-name completion after a space.
// reservedKeywords are single-token commands that should never show area autocomplete.
var contextCommands = map[string]bool{
	"home": true, "homes": true, "ctx": true, "context": true,
}

var reservedKeywords = map[string]bool{
	"home": true, "homes": true, "ctx": true, "context": true,
}

// ComputeCommandSuggestions returns autocomplete suggestions for command-mode input.
//
//   - Before a space: area names first, then shortcut keywords; prefix matches first.
//   - After a space: if the prefix is an exact area name, suggest device-type shortcuts.
//     Otherwise, treat the whole buffer as an area prefix and keep suggesting area names.
func ComputeCommandSuggestions(buffer string, areas []model.HassArea, homes []model.HassConfig) []string {
	areaNames := make([]string, len(areas))
	areaSet := make(map[string]bool, len(areas))
	for i, a := range areas {
		areaNames[i] = a.Name
		areaSet[strings.ToLower(a.Name)] = true
	}
	sort.Strings(areaNames)

	spaceIdx := strings.IndexByte(buffer, ' ')

	if spaceIdx < 0 {
		// Single-token case — suppress suggestions for reserved keywords
		if reservedKeywords[strings.ToLower(buffer)] {
			return []string{}
		}
		bufLow := strings.ToLower(buffer)

		var prefixMatches, subMatches []string
		seen := make(map[string]bool)
		for _, name := range areaNames {
			nl := strings.ToLower(name)
			if strings.HasPrefix(nl, bufLow) && !seen[name] {
				prefixMatches = append(prefixMatches, name)
				seen[name] = true
			}
		}
		for _, name := range areaNames {
			nl := strings.ToLower(name)
			if !seen[name] && strings.Contains(nl, bufLow) {
				subMatches = append(subMatches, name)
				seen[name] = true
			}
		}
		// Shortcut keywords after area matches
		for _, key := range model.ShortcutKeys {
			kl := strings.ToLower(key)
			if !seen[key] && (strings.HasPrefix(kl, bufLow) || strings.Contains(kl, bufLow)) {
				subMatches = append(subMatches, key)
				seen[key] = true
			}
		}

		return sliceN(append(prefixMatches, subMatches...), 6)
	}

	// Two-part case
	prefix := buffer[:spaceIdx]
	query := strings.ToLower(buffer[spaceIdx+1:])
	prefixLow := strings.ToLower(prefix)

	if contextCommands[prefixLow] {
		names := make([]string, len(homes))
		for i, h := range homes {
			names[i] = h.Name
		}
		return filterAndPrefix(prefix, query, names, false)
	}

	// Only switch to type-completion if the prefix is an exact area name.
	// Otherwise the user is still mid-typing an area name (e.g. "back p" for "Back Porch").
	if areaSet[prefixLow] {
		return filterAndPrefix(prefix, query, model.ShortcutKeys, true)
	}

	// Still completing the area name — suggest area names matching the full buffer.
	bufLow := strings.ToLower(buffer)
	var prefixMatches, subMatches []string
	seen := make(map[string]bool)
	for _, name := range areaNames {
		nl := strings.ToLower(name)
		if strings.HasPrefix(nl, bufLow) && !seen[name] {
			prefixMatches = append(prefixMatches, name)
			seen[name] = true
		}
	}
	for _, name := range areaNames {
		nl := strings.ToLower(name)
		if !seen[name] && strings.Contains(nl, bufLow) {
			subMatches = append(subMatches, name)
			seen[name] = true
		}
	}
	return sliceN(append(prefixMatches, subMatches...), 6)
}

// ComputeFilterSuggestions returns suggestions for filter (/search) mode.
// Order: friendly-name prefix matches → friendly-name substring matches → entity_id
// substring matches. De-duplicated by friendly name; sliced to 6; empty query returns [].
func ComputeFilterSuggestions(query string, entities []model.HassEntity, areaMap map[string]string) []string {
	_ = areaMap // kept for signature compatibility
	if query == "" {
		return []string{}
	}
	qLow := strings.ToLower(query)

	var prefixFN, subFN []string
	seen := make(map[string]bool)

	for _, e := range entities {
		name := clr.FriendlyName(e)
		nameLow := strings.ToLower(name)
		if seen[name] {
			continue
		}
		if strings.HasPrefix(nameLow, qLow) {
			prefixFN = append(prefixFN, name)
			seen[name] = true
		} else if strings.Contains(nameLow, qLow) {
			subFN = append(subFN, name)
			seen[name] = true
		}
	}

	// entity_id substring matches (not already added via friendly name)
	var subID []string
	for _, e := range entities {
		name := clr.FriendlyName(e)
		if seen[name] {
			continue
		}
		if strings.Contains(strings.ToLower(e.EntityID), qLow) {
			subID = append(subID, name)
			seen[name] = true
		}
	}

	all := append(prefixFN, append(subFN, subID...)...)
	return sliceN(all, 6)
}

// ComputeAreaSuggestions returns suggestions for area-assignment input mode.
// Empty buffer returns first 8 area names in source order.
// Non-empty: prefix matches first (source order), then substring matches (source order); sliced to 8.
func ComputeAreaSuggestions(buffer string, areas []model.HassArea) []string {
	if buffer == "" {
		names := make([]string, 0, len(areas))
		for _, a := range areas {
			names = append(names, a.Name)
		}
		return sliceN(names, 8)
	}

	bufLow := strings.ToLower(buffer)
	var prefix, sub []string
	seen := make(map[string]bool)
	for _, a := range areas {
		n := a.Name
		nl := strings.ToLower(n)
		if strings.HasPrefix(nl, bufLow) {
			prefix = append(prefix, n)
			seen[n] = true
		}
	}
	for _, a := range areas {
		n := a.Name
		if seen[n] {
			continue
		}
		nl := strings.ToLower(n)
		if strings.Contains(nl, bufLow) {
			sub = append(sub, n)
		}
	}
	return sliceN(append(prefix, sub...), 8)
}

// ─────────────────────────────────────────────── Column widths ──

// Cols holds responsive column width values for the entity table.
type Cols struct {
	Name  int
	State int
	Area  int
	Age   int
}

// ComputeCols returns table column widths for the given inner width.
func ComputeCols(innerWidth int) Cols {
	switch {
	case innerWidth >= 80:
		return Cols{Name: 30, State: 22, Area: 16, Age: 5}
	case innerWidth >= 65:
		return Cols{Name: 24, State: 16, Area: 12, Age: 4}
	case innerWidth >= 50:
		return Cols{Name: 20, State: 12, Area: 0, Age: 0}
	default:
		return Cols{Name: 16, State: 10, Area: 0, Age: 0}
	}
}

// ─────────────────────────────────────────────── Tag builders ──

// Tag wraps text in a tview color tag: [hex]text[-].
func Tag(hex, text string) string {
	return fmt.Sprintf("[%s]%s[-]", hex, text)
}

// Bold wraps text in a tview bold tag.
func Bold(text string) string {
	return fmt.Sprintf("[::b]%s[::-]", text)
}

// Pad right-pads or truncates s to exactly width runes.
func Pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// ─────────────────────────────────────────────── Internal helpers ──

func entityDomain(entityID string) string {
	dot := strings.IndexByte(entityID, '.')
	if dot < 0 {
		return entityID
	}
	return entityID[:dot]
}

func sliceN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// filterAndPrefix filters candidates by query and prefixes each with "prefix ".
// sorted: if true candidates are assumed already in desired order (caller sorts).
func filterAndPrefix(prefix, queryLow string, candidates []string, _ bool) []string {
	if queryLow == "" {
		result := make([]string, len(candidates))
		for i, c := range candidates {
			result[i] = prefix + " " + c
		}
		return sliceN(result, 6)
	}

	var prefixM, subM []string
	for _, c := range candidates {
		cl := strings.ToLower(c)
		if strings.HasPrefix(cl, queryLow) {
			prefixM = append(prefixM, c)
		} else if strings.Contains(cl, queryLow) {
			subM = append(subM, c)
		}
	}
	all := append(prefixM, subM...)
	result := make([]string, len(all))
	for i, c := range all {
		result[i] = prefix + " " + c
	}
	return sliceN(result, 6)
}
