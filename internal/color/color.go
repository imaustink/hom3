// Package color defines the HOM3 color palette, domain icons, and pure
// entity-formatting functions.  No side effects; safe to call from tests.
package color

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/imaustink/hom3/internal/model"
)

// ─────────────────────────────────────────────── Palette tokens ──

// Semantic palette token names (language-agnostic; used by conformance tests).
const (
	TokenStateOn      = "stateOn"
	TokenStateOff     = "stateOff"
	TokenStateUnavail = "stateUnavail"
	TokenStateUnknown = "stateUnknown"
	TokenTextPrimary  = "textPrimary"
)

// Palette holds tview-compatible hex color strings.
type Palette struct {
	BG            string
	BGAlt         string
	BGPanel       string
	BGHighlight   string
	BGSelected    string
	BGHeader      string
	Cyan          string
	Magenta       string
	Green         string
	Yellow        string
	Orange        string
	Red           string
	Blue          string
	Purple        string
	Teal          string
	CyanDim       string
	GreenDim      string
	YellowDim     string
	TextPrimary   string
	TextSecondary string
	TextDim       string
	TextHeader    string
	Border        string
	BorderActive  string
	BorderFocus   string
	StateOn       string
	StateOff      string
	StateUnavail  string
	StateUnknown  string
}

// COLORS is the HOM3 cyberpunk/synthwave palette.
var COLORS = Palette{
	BG:            "#0d1117",
	BGAlt:         "#161b22",
	BGPanel:       "#1c2128",
	BGHighlight:   "#30363d",
	BGSelected:    "#1f6feb",
	BGHeader:      "#010409",
	Cyan:          "#39d0d8",
	Magenta:       "#f92672",
	Green:         "#56d364",
	Yellow:        "#e3b341",
	Orange:        "#f0883e",
	Red:           "#ff7b72",
	Blue:          "#79c0ff",
	Purple:        "#bc8cff",
	Teal:          "#56d4dd",
	CyanDim:       "#1b6f6f",
	GreenDim:      "#2d6a30",
	YellowDim:     "#7a5a15",
	TextPrimary:   "#e6edf3",
	TextSecondary: "#8b949e",
	TextDim:       "#484f58",
	TextHeader:    "#f0f6fc",
	Border:        "#30363d",
	BorderActive:  "#39d0d8",
	BorderFocus:   "#f92672",
	StateOn:       "#56d364",
	StateOff:      "#484f58",
	StateUnavail:  "#f0883e",
	StateUnknown:  "#e3b341",
}

// TokenColor maps a palette token name to its hex color string.
func TokenColor(token string) string {
	switch token {
	case TokenStateOn:
		return COLORS.StateOn
	case TokenStateOff:
		return COLORS.StateOff
	case TokenStateUnavail:
		return COLORS.StateUnavail
	case TokenStateUnknown:
		return COLORS.StateUnknown
	default:
		return COLORS.TextPrimary
	}
}

// ─────────────────────────────────────────────────── Domain icons ──

// DomainIcons maps HA domain names to single Unicode glyphs.
var DomainIcons = map[string]string{
	"light":               "◆",
	"switch":              "⏻",
	"sensor":              "◉",
	"binary_sensor":       "◎",
	"climate":             "⊕",
	"cover":               "▫",
	"fan":                 "✦",
	"media_player":        "♫",
	"automation":          "⚙",
	"script":              "▶",
	"scene":               "✦",
	"person":              "◈",
	"device_tracker":      "◈",
	"camera":              "⊡",
	"lock":                "⊠",
	"vacuum":              "◌",
	"alarm_control_panel": "⊗",
	"weather":             "☁",
	"button":              "▣",
	"input_button":        "▣",
	"number":              "#",
	"input_number":        "#",
	"select":              "≡",
	"input_select":        "≡",
	"input_boolean":       "⏻",
	"input_text":          "T",
	"input_datetime":      "⊙",
	"sun":                 "☀",
	"zone":                "⬡",
	"update":              "↑",
	"calendar":            "▦",
	"timer":               "⊘",
	"counter":             "+",
	"group":               "⊞",
}

// ─────────────────────────────────────────────────── Pure formatters ──

// StateColor maps an HA state string to a palette token name.
func StateColor(state string) string {
	switch state {
	case "on", "open", "unlocked", "home", "playing", "active",
		"armed_away", "armed_home", "cleaning":
		return TokenStateOn
	case "off", "closed", "locked", "not_home", "paused", "idle",
		"disarmed", "docked", "standby", "stopped":
		return TokenStateOff
	case "unavailable":
		return TokenStateUnavail
	case "unknown":
		return TokenStateUnknown
	default:
		return TokenTextPrimary
	}
}

// DomainIcon returns the display glyph for the given entity_id.
func DomainIcon(entityID string) string {
	dot := strings.IndexByte(entityID, '.')
	if dot < 0 {
		return "○"
	}
	domain := entityID[:dot]
	if icon, ok := DomainIcons[domain]; ok {
		return icon
	}
	return "○"
}

// FriendlyName resolves the display name for an entity.
// Returns the friendly_name attribute when present, otherwise entity_id.
func FriendlyName(entity model.HassEntity) string {
	if fn, ok := entity.Attributes["friendly_name"]; ok {
		return fmt.Sprintf("%v", fn)
	}
	return entity.EntityID
}

// FormatState formats entity state for display in a domain-aware way.
func FormatState(entity model.HassEntity) string {
	domain := entityDomain(entity.EntityID)

	switch domain {
	case "light":
		if entity.State == "on" {
			if b, ok := entity.Attributes["brightness"]; ok {
				if bv, ok2 := toFloat64(b); ok2 {
					pct := int(math.Round(bv / 255.0 * 100))
					return fmt.Sprintf("on (%d%%)", pct)
				}
			}
		}
		return entity.State

	case "climate":
		if cur, ok := entity.Attributes["current_temperature"]; ok {
			if curF, ok2 := toFloat64(cur); ok2 {
				target := "?"
				if t, ok3 := entity.Attributes["temperature"]; ok3 {
					if tf, ok4 := toFloat64(t); ok4 {
						target = formatNum(tf)
					}
				}
				return fmt.Sprintf("%s %s°→%s°", entity.State, formatNum(curF), target)
			}
		}
		return entity.State

	case "sensor", "number", "input_number":
		if u, ok := entity.Attributes["unit_of_measurement"]; ok {
			return fmt.Sprintf("%s %v", entity.State, u)
		}
		return entity.State

	case "media_player":
		if entity.State == "playing" {
			if title, ok := entity.Attributes["media_title"]; ok && title != nil {
				t := fmt.Sprintf("%v", title)
				runes := []rune(t)
				if len(runes) > 20 {
					t = string(runes[:20])
				}
				return "♫ " + t
			}
			return "playing"
		}
		return entity.State
	}

	return entity.State
}

// TimeSince formats a coarse human-readable age from an ISO timestamp to now.
func TimeSince(isoDate string) string {
	return TimeSinceFrom(isoDate, time.Now())
}

// TimeSinceFrom formats a duration relative to a reference time (useful for testing).
func TimeSinceFrom(isoDate string, now time.Time) string {
	t, err := time.Parse(time.RFC3339Nano, isoDate)
	if err != nil {
		// Try without nanoseconds
		t, err = time.Parse(time.RFC3339, isoDate)
		if err != nil {
			return "?"
		}
	}
	d := now.Sub(t)
	if d < 0 {
		return "0s"
	}
	secs := int(d.Seconds())
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh", secs/3600)
	default:
		return fmt.Sprintf("%dd", secs/86400)
	}
}

// DomainColorHex returns the hex accent color for a given entity domain.
func DomainColorHex(domain string) string {
	switch domain {
	case "light":
		return COLORS.Yellow
	case "switch", "input_boolean":
		return COLORS.Teal
	case "sensor":
		return COLORS.Blue
	case "binary_sensor":
		return COLORS.Blue
	case "climate":
		return COLORS.Orange
	case "cover":
		return COLORS.Cyan
	case "lock":
		return COLORS.Orange
	case "automation":
		return COLORS.Green
	case "script", "scene":
		return COLORS.Purple
	case "media_player":
		return COLORS.Magenta
	case "fan":
		return COLORS.Teal
	case "vacuum":
		return COLORS.Cyan
	case "alarm_control_panel":
		return COLORS.Red
	default:
		return COLORS.TextSecondary
	}
}

// ─────────────────────────────────────────────────── Helpers ──

func entityDomain(entityID string) string {
	dot := strings.IndexByte(entityID, '.')
	if dot < 0 {
		return entityID
	}
	return entityID[:dot]
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// formatNum formats a float, omitting the decimal for integers.
func formatNum(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%g", f)
}
