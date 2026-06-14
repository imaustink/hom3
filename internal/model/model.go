// Package model defines all shared types and domain constants for HOM3.
package model

import "time"

// ─────────────────────────────────────────────────── Device types ──

// DeviceType is the canonical view/filter category.
type DeviceType string

const (
	DeviceTypeAll           DeviceType = "all"
	DeviceTypeLights        DeviceType = "lights"
	DeviceTypeSwitches      DeviceType = "switches"
	DeviceTypeSensors       DeviceType = "sensors"
	DeviceTypeBinarySensors DeviceType = "binary_sensors"
	DeviceTypeClimate       DeviceType = "climate"
	DeviceTypeCovers        DeviceType = "covers"
	DeviceTypeFans          DeviceType = "fans"
	DeviceTypeMediaPlayers  DeviceType = "media_players"
	DeviceTypeAutomations   DeviceType = "automations"
	DeviceTypeScripts       DeviceType = "scripts"
	DeviceTypeScenes        DeviceType = "scenes"
	DeviceTypePersons       DeviceType = "persons"
	DeviceTypeCameras       DeviceType = "cameras"
	DeviceTypeLocks         DeviceType = "locks"
	DeviceTypeVacuums       DeviceType = "vacuums"
	DeviceTypeAlarms        DeviceType = "alarms"
	DeviceTypeWeather       DeviceType = "weather"
	DeviceTypeButtons       DeviceType = "buttons"
	DeviceTypeNumbers       DeviceType = "numbers"
	DeviceTypeSelects       DeviceType = "selects"
	DeviceTypeInputs        DeviceType = "inputs"
)

// DeviceTypeDomains maps each DeviceType to its HA domain(s).
var DeviceTypeDomains = map[DeviceType][]string{
	DeviceTypeAll:           {},
	DeviceTypeLights:        {"light"},
	DeviceTypeSwitches:      {"switch"},
	DeviceTypeSensors:       {"sensor"},
	DeviceTypeBinarySensors: {"binary_sensor"},
	DeviceTypeClimate:       {"climate"},
	DeviceTypeCovers:        {"cover"},
	DeviceTypeFans:          {"fan"},
	DeviceTypeMediaPlayers:  {"media_player"},
	DeviceTypeAutomations:   {"automation"},
	DeviceTypeScripts:       {"script"},
	DeviceTypeScenes:        {"scene"},
	DeviceTypePersons:       {"person", "device_tracker"},
	DeviceTypeCameras:       {"camera"},
	DeviceTypeLocks:         {"lock"},
	DeviceTypeVacuums:       {"vacuum"},
	DeviceTypeAlarms:        {"alarm_control_panel"},
	DeviceTypeWeather:       {"weather"},
	DeviceTypeButtons:       {"button", "input_button"},
	DeviceTypeNumbers:       {"number", "input_number"},
	DeviceTypeSelects:       {"select", "input_select"},
	DeviceTypeInputs:        {"input_boolean", "input_text", "input_datetime"},
}

// ShortcutKeys is the ordered list used for command autocomplete.
// Order must match the device-types fixture for conformance.
var ShortcutKeys = []string{
	"all", "lights", "light", "switches", "switch", "sensors", "sensor",
	"binary_sensors", "bs", "climate", "covers", "cover", "fans", "fan",
	"media_players", "media", "mp", "automations", "auto", "automation",
	"scripts", "script", "scenes", "scene", "persons", "person", "cameras",
	"camera", "locks", "lock", "vacuums", "vacuum", "alarms", "alarm",
	"weather", "buttons", "button", "numbers", "number", "selects", "select",
	"inputs", "input",
}

// DeviceTypeShortcuts maps alias strings to their canonical DeviceType.
var DeviceTypeShortcuts = map[string]DeviceType{
	"all":             DeviceTypeAll,
	"lights":          DeviceTypeLights,
	"light":           DeviceTypeLights,
	"switches":        DeviceTypeSwitches,
	"switch":          DeviceTypeSwitches,
	"sensors":         DeviceTypeSensors,
	"sensor":          DeviceTypeSensors,
	"binary_sensors":  DeviceTypeBinarySensors,
	"bs":              DeviceTypeBinarySensors,
	"climate":         DeviceTypeClimate,
	"covers":          DeviceTypeCovers,
	"cover":           DeviceTypeCovers,
	"fans":            DeviceTypeFans,
	"fan":             DeviceTypeFans,
	"media_players":   DeviceTypeMediaPlayers,
	"media":           DeviceTypeMediaPlayers,
	"mp":              DeviceTypeMediaPlayers,
	"automations":     DeviceTypeAutomations,
	"auto":            DeviceTypeAutomations,
	"automation":      DeviceTypeAutomations,
	"scripts":         DeviceTypeScripts,
	"script":          DeviceTypeScripts,
	"scenes":          DeviceTypeScenes,
	"scene":           DeviceTypeScenes,
	"persons":         DeviceTypePersons,
	"person":          DeviceTypePersons,
	"cameras":         DeviceTypeCameras,
	"camera":          DeviceTypeCameras,
	"locks":           DeviceTypeLocks,
	"lock":            DeviceTypeLocks,
	"vacuums":         DeviceTypeVacuums,
	"vacuum":          DeviceTypeVacuums,
	"alarms":          DeviceTypeAlarms,
	"alarm":           DeviceTypeAlarms,
	"weather":         DeviceTypeWeather,
	"buttons":         DeviceTypeButtons,
	"button":          DeviceTypeButtons,
	"numbers":         DeviceTypeNumbers,
	"number":          DeviceTypeNumbers,
	"selects":         DeviceTypeSelects,
	"select":          DeviceTypeSelects,
	"inputs":          DeviceTypeInputs,
	"input":           DeviceTypeInputs,
}

// ─────────────────────────────────────────────── HA entity types ──

// HassEntity represents a Home Assistant entity state snapshot.
type HassEntity struct {
	EntityID    string                 `json:"entity_id"`
	State       string                 `json:"state"`
	Attributes  map[string]interface{} `json:"attributes"`
	LastChanged string                 `json:"last_changed"`
	LastUpdated string                 `json:"last_updated"`
	Context     HassContext            `json:"context"`
}

// HassContext is the HA context object attached to each entity state.
type HassContext struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parent_id"`
	UserID   *string `json:"user_id"`
}

// HassArea is an HA area registry entry.
type HassArea struct {
	AreaID  string  `json:"area_id"`
	Name    string  `json:"name"`
	Picture *string `json:"picture"`
}

// HassDevice is an HA device registry entry.
type HassDevice struct {
	ID     string  `json:"id"`
	AreaID *string `json:"area_id"`
	Name   string  `json:"name"`
}

// HassEntityRegistryEntry is an HA entity registry entry.
type HassEntityRegistryEntry struct {
	EntityID string  `json:"entity_id"`
	DeviceID *string `json:"device_id"`
	AreaID   *string `json:"area_id"`
}

// HassStateChange is emitted when an entity's state changes.
type HassStateChange struct {
	EntityID string      `json:"entity_id"`
	NewState *HassEntity `json:"new_state"`
	OldState *HassEntity `json:"old_state"`
}

// ─────────────────────────────────────────────────── Config types ──

// HassConfig holds connection details for one Home Assistant instance.
type HassConfig struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

// HassConfigFile supports both legacy single-home and multi-home file formats.
type HassConfigFile struct {
	URL   string       `json:"url"`
	Token string       `json:"token"`
	Homes []HassConfig `json:"homes"`
}

// ─────────────────────────────────────────────────────── AppState ──

// InputMode is the type of active text-input mode.
type InputMode string

const (
	InputModeRename InputMode = "rename"
	InputModeArea   InputMode = "area"
)

// AppState is the single source of truth for all UI state.
type AppState struct {
	CurrentView     DeviceType
	Filter          string
	FilterMode      bool
	AreaFilter      string
	SelectedIndex   int
	Entities        []HassEntity
	FilteredEntities []HassEntity
	Connected       bool
	CommandMode     bool
	CommandBuffer   string
	DetailVisible   bool
	HelpMode        bool
	Areas           []HassArea
	Devices         []HassDevice
	SortField       string
	SortAsc         bool
	Error           *string
	LastRefresh     *time.Time
	AutocompleteSuggestions []string
	AutocompleteIndex       int
	InputMode       *InputMode
	InputBuffer     string
	RecentAreas     []string
	ContextMode     bool
	Homes           []HassConfig
	ActiveHomeIndex int
	ContextSelectedIndex int
}
