// Package client manages the WebSocket connection to Home Assistant and
// implements all entity-control service calls.
package client

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/imaustink/hatui/internal/model"
)

// ─────────────────────────────────────────────── Public event types ──

// StateChangeEvent is sent on the StateChanges channel when an entity changes.
type StateChangeEvent struct {
	EntityID string
	NewState *model.HassEntity
	OldState *model.HassEntity
}

// ─────────────────────────────────────────────── Client ──

// HassClient manages the WebSocket connection and entity state cache.
type HassClient struct {
	// Public entity data (read-only after bootstrap)
	Entities       map[string]*model.HassEntity
	Areas          []model.HassArea
	Devices        []model.HassDevice
	EntityRegistry []model.HassEntityRegistryEntry
	Connected      bool

	// Channels for communicating with the App layer
	StateChanges chan StateChangeEvent
	Disconnected chan struct{}

	// Configurable send function; nil → real WebSocket send.
	// Tests inject a stub here.
	sendFn func(domain, service string, serviceData, target map[string]interface{}) error

	// WebSocket internals
	conn    *websocket.Conn
	mu      sync.Mutex
	msgID   int
	pending map[int]chan wsResult
}

// wsResult carries the server's response for a pending request.
type wsResult struct {
	Success bool
	Result  json.RawMessage
	Error   map[string]interface{}
}

// New returns a new, unconnected HassClient.
func New() *HassClient {
	return &HassClient{
		Entities:     make(map[string]*model.HassEntity),
		StateChanges: make(chan StateChangeEvent, 64),
		Disconnected: make(chan struct{}, 1),
		pending:      make(map[int]chan wsResult),
	}
}

// NewTestable returns a HassClient wired with a custom sendFn instead of a
// real WebSocket connection.  Used only in tests.
func NewTestable(fn func(domain, service string, serviceData, target map[string]interface{}) error) *HassClient {
	c := New()
	c.sendFn = fn
	return c
}

// SetEntity seeds an entity into the client's in-memory store (for tests).
func (c *HassClient) SetEntity(e *model.HassEntity) {
	c.Entities[e.EntityID] = e
}

// ─────────────────────────────────────────────── Connection ──

// NormalizeURL converts http(s) URLs to ws(s) and strips trailing slashes.
func NormalizeURL(rawURL string) (string, error) {
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/websocket"
	return u.String(), nil
}

// Connect dials Home Assistant, performs the auth handshake, bootstraps all
// state/areas/devices/entity registry, and subscribes to state_changed events.
func (c *HassClient) Connect(cfg model.HassConfig) error {
	wsURL, err := NormalizeURL(cfg.URL)
	if err != nil {
		return fmt.Errorf("invalid HA URL: %w", err)
	}

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial: %w", err)
	}
	c.conn = conn

	// Reset channels so reconnect doesn't see stale signals.
	c.Disconnected = make(chan struct{}, 1)
	// Drain any stale state changes from the previous connection.
	for len(c.StateChanges) > 0 {
		<-c.StateChanges
	}

	// Bootstrap read pump reads the initial auth_required + auth_ok, then
	// forwards all subsequent messages to the pending map or events.
	if err := c.authHandshake(cfg.Token); err != nil {
		conn.Close()
		return err
	}
	c.Connected = true

	// Wire up the real send function
	c.sendFn = c.wsCallService

	// Start async read pump
	go c.readPump()

	// Bootstrap data
	if err := c.bootstrap(); err != nil {
		return err
	}

	// Subscribe to state_changed
	return c.subscribeStateChanged()
}

// authHandshake performs the HA WebSocket authentication exchange.
func (c *HassClient) authHandshake(token string) error {
	timeout := time.After(10 * time.Second)

	// Expect auth_required
	var msg map[string]interface{}
	if err := c.conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("auth_required read: %w", err)
	}
	if msg["type"] != "auth_required" {
		return fmt.Errorf("expected auth_required, got %v", msg["type"])
	}

	// Send auth
	authMsg := map[string]interface{}{"type": "auth", "access_token": token}
	if err := c.conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("auth send: %w", err)
	}

	// Expect auth_ok or auth_invalid
	done := make(chan error, 1)
	go func() {
		var resp map[string]interface{}
		if err := c.conn.ReadJSON(&resp); err != nil {
			done <- fmt.Errorf("auth response: %w", err)
			return
		}
		switch resp["type"] {
		case "auth_ok":
			done <- nil
		case "auth_invalid":
			done <- fmt.Errorf("auth_invalid: check your token")
		default:
			done <- fmt.Errorf("unexpected auth response: %v", resp["type"])
		}
	}()

	select {
	case err := <-done:
		return err
	case <-timeout:
		return fmt.Errorf("auth timeout")
	}
}

// bootstrap fetches states, areas, devices, and entity registry.
func (c *HassClient) bootstrap() error {
	// get_states
	raw, err := c.request(map[string]interface{}{"type": "get_states"})
	if err != nil {
		return fmt.Errorf("get_states: %w", err)
	}
	var states []model.HassEntity
	if err := json.Unmarshal(raw, &states); err != nil {
		return fmt.Errorf("parse states: %w", err)
	}
	for i := range states {
		e := states[i]
		c.Entities[e.EntityID] = &e
	}

	// area registry
	raw, err = c.request(map[string]interface{}{"type": "config/area_registry/list"})
	if err != nil {
		return fmt.Errorf("area_registry: %w", err)
	}
	if err := json.Unmarshal(raw, &c.Areas); err != nil {
		return fmt.Errorf("parse areas: %w", err)
	}

	// device registry
	raw, err = c.request(map[string]interface{}{"type": "config/device_registry/list"})
	if err != nil {
		return fmt.Errorf("device_registry: %w", err)
	}
	if err := json.Unmarshal(raw, &c.Devices); err != nil {
		return fmt.Errorf("parse devices: %w", err)
	}

	// entity registry
	raw, err = c.request(map[string]interface{}{"type": "config/entity_registry/list"})
	if err != nil {
		return fmt.Errorf("entity_registry: %w", err)
	}
	if err := json.Unmarshal(raw, &c.EntityRegistry); err != nil {
		return fmt.Errorf("parse entity registry: %w", err)
	}

	return nil
}

// subscribeStateChanged subscribes to Home Assistant state_changed events.
func (c *HassClient) subscribeStateChanged() error {
	_, err := c.request(map[string]interface{}{
		"type":       "subscribe_events",
		"event_type": "state_changed",
	})
	return err
}

// readPump runs in a goroutine and dispatches incoming WebSocket messages.
func (c *HassClient) readPump() {
	defer func() {
		c.Connected = false
		select {
		case c.Disconnected <- struct{}{}:
		default:
		}
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg map[string]json.RawMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		var msgType string
		if err := json.Unmarshal(msg["type"], &msgType); err != nil {
			continue
		}

		switch msgType {
		case "result":
			var id int
			if err := json.Unmarshal(msg["id"], &id); err != nil {
				continue
			}
			var success bool
			json.Unmarshal(msg["success"], &success)

			res := wsResult{Success: success}
			if r, ok := msg["result"]; ok {
				res.Result = r
			}
			c.mu.Lock()
			ch, ok := c.pending[id]
			if ok {
				delete(c.pending, id)
			}
			c.mu.Unlock()
			if ok {
				ch <- res
			}

		case "event":
			var evtWrapper struct {
				Event struct {
					Data struct {
						EntityID string            `json:"entity_id"`
						NewState *model.HassEntity `json:"new_state"`
						OldState *model.HassEntity `json:"old_state"`
					} `json:"data"`
				} `json:"event"`
			}
			if err := json.Unmarshal(raw, &evtWrapper); err != nil {
				continue
			}
			d := evtWrapper.Event.Data
			if d.NewState != nil {
				c.Entities[d.EntityID] = d.NewState
			}
			select {
			case c.StateChanges <- StateChangeEvent{
				EntityID: d.EntityID,
				NewState: d.NewState,
				OldState: d.OldState,
			}:
			default:
			}
		}
	}
}

// request sends a command and waits for its result.
func (c *HassClient) request(msg map[string]interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.msgID++
	id := c.msgID
	msg["id"] = id
	ch := make(chan wsResult, 1)
	c.pending[id] = ch
	err := c.conn.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	res := <-ch
	if !res.Success {
		return nil, fmt.Errorf("HA error: %v", res.Error)
	}
	return res.Result, nil
}

// ─────────────────────────────────────────────── Service calls ──

// wsCallService is the real WebSocket implementation of sendFn.
func (c *HassClient) wsCallService(domain, service string, serviceData, target map[string]interface{}) error {
	msg := map[string]interface{}{
		"type":    "call_service",
		"domain":  domain,
		"service": service,
	}
	if len(serviceData) > 0 {
		msg["service_data"] = serviceData
	}
	if len(target) > 0 {
		msg["target"] = target
	}
	_, err := c.request(msg)
	return err
}

// callService calls a HA service with entity_id in service_data.
func (c *HassClient) callService(domain, service, entityID string, extra map[string]interface{}) error {
	sd := map[string]interface{}{"entity_id": entityID}
	for k, v := range extra {
		sd[k] = v
	}
	return c.sendFn(domain, service, sd, nil)
}

// callServiceTarget calls a HA service with an explicit target map.
func (c *HassClient) callServiceTarget(domain, service string, target, serviceData map[string]interface{}) error {
	return c.sendFn(domain, service, serviceData, target)
}

// ─────────────────────────────────────────────── Entity control ──

// ToggleEntity sends a smart toggle for the given entity.
func (c *HassClient) ToggleEntity(entityID string) error {
	domain := entityDomain(entityID)
	toggleDomains := map[string]bool{
		"light": true, "switch": true, "fan": true, "cover": true,
		"media_player": true, "lock": true, "automation": true, "input_boolean": true,
	}
	if !toggleDomains[domain] {
		return nil
	}
	return c.callService(domain, "toggle", entityID, nil)
}

// ActivateEntity performs a domain-aware activation, returning true if an action was taken.
func (c *HassClient) ActivateEntity(entityID string) (bool, error) {
	domain := entityDomain(entityID)
	var svc string
	switch domain {
	case "light", "switch", "fan", "cover", "media_player", "lock",
		"automation", "input_boolean":
		svc = "toggle"
	case "button", "input_button":
		svc = "press"
	case "scene":
		svc = "turn_on"
	case "script":
		e := c.Entities[entityID]
		if e != nil && e.State == "on" {
			svc = "turn_off"
		} else {
			svc = "turn_on"
		}
	case "vacuum":
		e := c.Entities[entityID]
		if e != nil && e.State == "cleaning" {
			svc = "stop"
		} else {
			svc = "start"
		}
	default:
		return false, nil
	}
	return true, c.callService(domain, svc, entityID, nil)
}

// AdjustBrightness adjusts a light's brightness by delta (0–255).
// Returns true if the service call was sent.
func (c *HassClient) AdjustBrightness(entityID string, delta int) (bool, error) {
	e := c.Entities[entityID]
	if e == nil {
		return false, nil
	}
	if delta < 0 && e.State != "on" {
		return false, nil
	}
	current := 128.0
	if b, ok := e.Attributes["brightness"]; ok {
		if bf, ok2 := toFloat64(b); ok2 {
			current = bf
		}
	}
	newB := clamp(int(current)+delta, 0, 255)
	return true, c.callService("light", "turn_on", entityID, map[string]interface{}{"brightness": newB})
}

// AdjustTemperature adjusts a climate entity's target temperature.
func (c *HassClient) AdjustTemperature(entityID string, delta float64) error {
	e := c.Entities[entityID]
	current := 20.0
	if e != nil {
		if t, ok := e.Attributes["temperature"]; ok {
			if tf, ok2 := toFloat64(t); ok2 {
				current = tf
			}
		}
	}
	// Round to nearest 0.5
	newTemp := math.Round((current+delta)/0.5) * 0.5
	return c.callService("climate", "set_temperature", entityID, map[string]interface{}{"temperature": newTemp})
}

// CycleHvacMode cycles to the next HVAC mode.
func (c *HassClient) CycleHvacMode(entityID string) error {
	e := c.Entities[entityID]
	if e == nil {
		return nil
	}
	modesRaw, ok := e.Attributes["hvac_modes"]
	if !ok {
		return nil
	}
	modes, ok := toStringSlice(modesRaw)
	if !ok || len(modes) <= 1 {
		return nil
	}
	cur := e.State
	idx := -1
	for i, m := range modes {
		if m == cur {
			idx = i
			break
		}
	}
	next := modes[(idx+1)%len(modes)]
	return c.callService("climate", "set_hvac_mode", entityID, map[string]interface{}{"hvac_mode": next})
}

// AdjustFanSpeed steps a fan's percentage by direction (+1/-1).
// Returns the new percentage (or nil when blocked).
func (c *HassClient) AdjustFanSpeed(entityID string, direction int) (*float64, error) {
	e := c.Entities[entityID]
	if e == nil {
		return nil, nil
	}
	if direction < 0 && e.State != "on" {
		return nil, nil
	}
	step := 25.0
	if s, ok := e.Attributes["percentage_step"]; ok {
		if sf, ok2 := toFloat64(s); ok2 && sf > 0 {
			step = sf
		}
	}
	current := 0.0
	if p, ok := e.Attributes["percentage"]; ok {
		if pf, ok2 := toFloat64(p); ok2 {
			current = pf
		}
	}
	newPct := math.Round(current/step)*step + float64(direction)*step
	newPct = math.Min(100, math.Max(0, newPct))

	if newPct == 0 {
		v := 0.0
		return &v, c.callService("fan", "turn_off", entityID, nil)
	}
	return &newPct, c.callService("fan", "set_percentage", entityID, map[string]interface{}{"percentage": newPct})
}

// AdjustVolume adjusts a media_player's volume by delta.
func (c *HassClient) AdjustVolume(entityID string, delta float64) error {
	e := c.Entities[entityID]
	current := 0.5
	if e != nil {
		if v, ok := e.Attributes["volume_level"]; ok {
			if vf, ok2 := toFloat64(v); ok2 {
				current = vf
			}
		}
	}
	newVol := math.Round((current+delta)*100) / 100
	newVol = math.Min(1, math.Max(0, newVol))
	return c.callService("media_player", "volume_set", entityID, map[string]interface{}{"volume_level": newVol})
}

// MediaPlayerCommand sends a command to a media player (e.g. "media_next_track").
func (c *HassClient) MediaPlayerCommand(entityID, command string) error {
	return c.callService("media_player", command, entityID, nil)
}

// VacuumCommand sends a command to a vacuum ("start", "stop", "return_to_base").
func (c *HassClient) VacuumCommand(entityID, command string) error {
	return c.callService("vacuum", command, entityID, nil)
}

// AlarmControl sends an alarm panel command, optionally with a code.
func (c *HassClient) AlarmControl(entityID, action, code string) error {
	extra := map[string]interface{}{}
	if code != "" {
		extra["code"] = code
	}
	return c.callService("alarm_control_panel", action, entityID, extra)
}

// ControlCover sends open_cover, close_cover, or stop_cover.
func (c *HassClient) ControlCover(entityID, action string) error {
	return c.callService("cover", action, entityID, nil)
}

// AdjustNumber steps a number/input_number entity by direction (+1/-1).
func (c *HassClient) AdjustNumber(entityID string, direction int) error {
	e := c.Entities[entityID]
	if e == nil {
		return nil
	}
	current, err := strconv.ParseFloat(e.State, 64)
	if err != nil {
		return nil
	}
	step := 1.0
	if s, ok := e.Attributes["step"]; ok {
		if sf, ok2 := toFloat64(s); ok2 && sf > 0 {
			step = sf
		}
	}
	newVal := current + float64(direction)*step
	if min, ok := e.Attributes["min"]; ok {
		if minF, ok2 := toFloat64(min); ok2 {
			newVal = math.Max(newVal, minF)
		}
	}
	if max, ok := e.Attributes["max"]; ok {
		if maxF, ok2 := toFloat64(max); ok2 {
			newVal = math.Min(newVal, maxF)
		}
	}
	domain := entityDomain(entityID)
	return c.callService(domain, "set_value", entityID, map[string]interface{}{"value": newVal})
}

// CycleSelectOption cycles a select/input_select to the next or previous option.
func (c *HassClient) CycleSelectOption(entityID string, direction int) error {
	e := c.Entities[entityID]
	if e == nil {
		return nil
	}
	optsRaw, ok := e.Attributes["options"]
	if !ok {
		return nil
	}
	opts, ok := toStringSlice(optsRaw)
	if !ok || len(opts) <= 1 {
		return nil
	}
	cur := e.State
	idx := -1
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	next := opts[(idx+direction+len(opts))%len(opts)]
	domain := entityDomain(entityID)
	return c.callService(domain, "select_option", entityID, map[string]interface{}{"option": next})
}

// BulkPower calls turn_on/turn_off (or open_cover/close_cover for covers)
// grouped by domain.
func (c *HassClient) BulkPower(entityIDs []string, action string) error {
	supported := map[string]bool{
		"light": true, "switch": true, "fan": true, "cover": true,
		"media_player": true, "automation": true, "input_boolean": true,
	}
	grouped := make(map[string][]string)
	for _, id := range entityIDs {
		d := entityDomain(id)
		if supported[d] {
			grouped[d] = append(grouped[d], id)
		}
	}
	// Send in a consistent order (sorted) so conformance tests can predict order
	domains := make([]string, 0, len(grouped))
	for d := range grouped {
		domains = append(domains, d)
	}
	// Preserve original ordering: iterate entityIDs to build order
	seen := make(map[string]bool)
	var orderedDomains []string
	for _, id := range entityIDs {
		d := entityDomain(id)
		if supported[d] && !seen[d] {
			orderedDomains = append(orderedDomains, d)
			seen[d] = true
		}
	}
	_ = domains
	for _, domain := range orderedDomains {
		ids := grouped[domain]
		svc := action
		if action == "on" {
			svc = "turn_on"
		} else {
			svc = "turn_off"
		}
		if domain == "cover" {
			if action == "on" {
				svc = "open_cover"
			} else {
				svc = "close_cover"
			}
		}
		sd := map[string]interface{}{"entity_id": ids}
		if err := c.sendFn(domain, svc, sd, nil); err != nil {
			return err
		}
	}
	return nil
}

// RenameDevice sends a device_registry update to rename a device.
func (c *HassClient) RenameDevice(deviceID, name string) error {
	_, err := c.request(map[string]interface{}{
		"type":      "config/device_registry/update",
		"device_id": deviceID,
		"name":      name,
	})
	return err
}

// AssignDeviceArea assigns a device to an area by area_id.
func (c *HassClient) AssignDeviceArea(deviceID, areaID string) error {
	_, err := c.request(map[string]interface{}{
		"type":      "config/device_registry/update",
		"device_id": deviceID,
		"area_id":   areaID,
	})
	return err
}

// ─────────────────────────────────────────────── Helpers ──

func entityDomain(entityID string) string {
	dot := strings.IndexByte(entityID, '.')
	if dot < 0 {
		return entityID
	}
	return entityID[:dot]
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

func toStringSlice(v interface{}) ([]string, bool) {
	raw, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	s := make([]string, 0, len(raw))
	for _, r := range raw {
		str, ok := r.(string)
		if !ok {
			return nil, false
		}
		s = append(s, str)
	}
	return s, true
}
