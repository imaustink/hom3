// Package view contains the App — the top-level tview orchestrator that owns
// all UI state, key bindings, mode transitions, and render pipelines.
package view

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/imaustink/hatui/internal/client"
	clr "github.com/imaustink/hatui/internal/color"
	"github.com/imaustink/hatui/internal/model"
	"github.com/imaustink/hatui/internal/render"
	"github.com/imaustink/hatui/internal/ui"
)

// ─────────────────────────────────────────────── App ──

// App is the root TUI orchestrator.
type App struct {
	// tview primitives
	tapp     *tview.Application
	pages    *tview.Pages
	layout   *tview.Flex
	header   *tview.TextView
	table    *tview.Table
	detail   *tview.TextView
	cmdBar   *tview.TextView
	sttBar   *tview.TextView
	helpView *tview.TextView
	toast    *tview.TextView
	acList   *tview.List

	// data
	state   model.AppState
	areaMap map[string]string

	// client / config
	client *client.HassClient
	homes  []model.HassConfig
}

// NewApp constructs a fully wired App.
func NewApp(c *client.HassClient, homes []model.HassConfig, activeIdx int) *App {
	a := &App{
		client:  c,
		homes:   homes,
		areaMap: make(map[string]string),
		state: model.AppState{
			CurrentView:     model.DeviceTypeAll,
			Homes:           homes,
			ActiveHomeIndex: activeIdx,
			SortAsc:         true,
			DetailVisible:   true,
			Connected:       false,
		},
	}

	// Build widgets
	a.tapp = tview.NewApplication()
	a.header = ui.NewHeader()
	a.table = ui.NewEntityTable()
	a.detail = ui.NewDetailPanel()
	a.sttBar = ui.NewStatusBar()
	a.cmdBar = ui.NewCommandBar()
	a.helpView = ui.NewHelpOverlay()
	a.toast = ui.NewToast()
	a.acList = ui.NewAutocompleteList()

	// Pages
	a.pages = tview.NewPages()
	a.layout = ui.BuildMainLayout(a.header, a.detail, a.sttBar, a.cmdBar, a.table)
	a.pages.AddPage("main", a.layout, true, true)
	a.pages.AddPage("help", centeredBox(a.helpView, 70, 40), true, false)

	a.tapp.SetRoot(a.pages, true)

	// Wire key handling
	a.bindKeys()
	a.bindTableSelect()

	return a
}

// Start connects to Home Assistant and runs the TUI event loop.
func (a *App) Start() error {
	// Render splash synchronously — tapp.Run() will do the first draw.
	a.renderSplash()

	// Connect with auto-reconnect in background.
	go a.connectLoop()

	return a.tapp.Run()
}

// connectLoop connects to HA and listens for events, reconnecting on disconnect
// with exponential backoff (1s → 2s → 4s … capped at 30s).
func (a *App) connectLoop() {
	if len(a.homes) == 0 {
		return
	}
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		cfg := a.homes[a.state.ActiveHomeIndex]
		if err := a.client.Connect(cfg); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.state.Connected = false
				a.showToast(fmt.Sprintf("Connection failed, retrying in %s…", backoff), true)
				a.renderStatusBar()
			})
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Connected successfully — reset backoff and refresh state.
		backoff = time.Second
		a.tapp.QueueUpdateDraw(func() {
			a.state.Connected = true
			a.state.Entities = clientEntities(a.client)
			a.buildAreaMap()
			a.applyFilter()
			a.renderAll()
		})

		// Listen for state changes until disconnect.
		for {
			select {
			case evt := <-a.client.StateChanges:
				a.tapp.QueueUpdateDraw(func() {
					a.syncEntity(evt)
					a.applyFilter()
					a.renderAll()
				})
			case <-a.client.Disconnected:
				a.tapp.QueueUpdateDraw(func() {
					a.state.Connected = false
					a.renderStatusBar()
					a.showToast(fmt.Sprintf("Disconnected — reconnecting in %s…", backoff), true)
				})
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				// Break inner loop to reconnect.
				goto reconnect
			}
		}
	reconnect:
	}
}

// ─────────────────────────────────────────────── Key bindings ──

func (a *App) bindKeys() {
	a.tapp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Always-available: Ctrl-C
		if event.Key() == tcell.KeyCtrlC {
			a.tapp.Stop()
			return nil
		}

		// Escape: dismiss overlays / exit modes
		if event.Key() == tcell.KeyEscape {
			a.handleEscape()
			return nil
		}

		// Help overlay absorbs all input except Escape and ?
		if a.state.HelpMode {
			if event.Rune() == '?' {
				a.toggleHelp()
			}
			return nil
		}

		// Context (home-switcher) mode
		if a.state.ContextMode {
			a.handleContextKey(event)
			return nil
		}

		// Command mode
		if a.state.CommandMode {
			a.handleCommandKey(event)
			return nil
		}

		// Filter mode
		if a.state.FilterMode {
			a.handleFilterKey(event)
			return nil
		}

		// Input mode (rename / area)
		if a.state.InputMode != nil {
			a.handleInputKey(event)
			return nil
		}

		// Normal mode
		a.handleNormalKey(event)
		return nil
	})
}

func (a *App) handleEscape() {
	if a.state.HelpMode {
		a.toggleHelp()
		return
	}
	if a.state.ContextMode {
		a.state.ContextMode = false
		a.renderAll()
		return
	}
	if a.state.CommandMode {
		a.state.CommandMode = false
		a.state.CommandBuffer = ""
		a.state.AutocompleteSuggestions = nil
		a.renderAll()
		return
	}
	if a.state.FilterMode {
		a.state.FilterMode = false
		a.state.Filter = ""
		a.applyFilter()
		a.renderAll()
		return
	}
	if a.state.InputMode != nil {
		a.state.InputMode = nil
		a.state.InputBuffer = ""
		a.renderAll()
	}
}

func (a *App) handleNormalKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyDown:
		a.moveSelection(1)
		return
	case tcell.KeyUp:
		a.moveSelection(-1)
		return
	case tcell.KeyHome:
		a.setSelection(0)
		return
	case tcell.KeyEnd:
		a.setSelection(len(a.state.FilteredEntities) - 1)
		return
	case tcell.KeyPgDn:
		a.moveSelection(20)
		return
	case tcell.KeyPgUp:
		a.moveSelection(-20)
		return
	case tcell.KeyEnter:
		a.activateSelected()
		return
	}

	switch event.Rune() {
	case 'j':
		a.moveSelection(1)
	case 'k':
		a.moveSelection(-1)
	case 'g':
		a.setSelection(0)
	case 'G':
		a.setSelection(len(a.state.FilteredEntities) - 1)
	case 't':
		a.toggleSelected()
	case 'r':
		a.refresh()
	case 'd':
		a.state.DetailVisible = !a.state.DetailVisible
		a.renderAll()
	case '?':
		a.toggleHelp()
	case 'q', 'Q':
		a.tapp.Stop()
	case ':':
		a.state.CommandMode = true
		a.state.CommandBuffer = ""
		a.updateAutocomplete()
		a.renderAll()
	case '/':
		a.state.FilterMode = true
		a.state.Filter = ""
		a.updateAutocomplete()
		a.renderAll()
	case 'C':
		a.openContextSwitcher()
	case '+', '=': // = is unshifted + on most keyboards
		a.handleAdjust(1)
	case '-', '_':
		a.handleAdjust(-1)
	case 'O':
		a.bulkPowerFiltered("off")
	case 'I':
		a.bulkPowerFiltered("on")
	case ' ':
		a.handleSpace()
	case 'm':
		a.handleCycleMode()
	case '[':
		a.mediaCommand("media_previous_track")
	case ']':
		a.mediaCommand("media_next_track")
	case 'o':
		a.coverCommand("open_cover")
	case 'c':
		a.coverCommand("close_cover")
	case 's':
		a.coverCommand("stop_cover")
	case '1', '2', '3', '4', '5':
		a.activateRecentArea(int(event.Rune() - '0'))
	}
}

func (a *App) handleCommandKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		a.executeCommand(a.state.CommandBuffer)
		a.state.CommandMode = false
		a.state.CommandBuffer = ""
		a.state.AutocompleteSuggestions = nil
		a.renderAll()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(a.state.CommandBuffer) > 0 {
			r := []rune(a.state.CommandBuffer)
			a.state.CommandBuffer = string(r[:len(r)-1])
		}
		a.updateAutocomplete()
		a.renderAll()
	case tcell.KeyTab:
		a.navigateAutocomplete(1)
	case tcell.KeyBacktab:
		a.navigateAutocomplete(-1)
	default:
		if event.Rune() != 0 {
			a.state.CommandBuffer += string(event.Rune())
			a.updateAutocomplete()
			a.renderAll()
		}
	}
}

func (a *App) handleFilterKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		if a.state.AutocompleteIndex >= 0 && a.state.AutocompleteIndex < len(a.state.AutocompleteSuggestions) {
			a.state.Filter = a.state.AutocompleteSuggestions[a.state.AutocompleteIndex]
		}
		a.state.FilterMode = false
		a.state.AutocompleteSuggestions = nil
		a.applyFilter()
		a.renderAll()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(a.state.Filter) > 0 {
			r := []rune(a.state.Filter)
			a.state.Filter = string(r[:len(r)-1])
		}
		a.updateAutocomplete()
		a.applyFilter()
		a.renderAll()
	case tcell.KeyTab:
		a.navigateAutocomplete(1)
	default:
		if event.Rune() != 0 {
			a.state.Filter += string(event.Rune())
			a.updateAutocomplete()
			a.applyFilter()
			a.renderAll()
		}
	}
}

func (a *App) handleInputKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		a.commitInput()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(a.state.InputBuffer) > 0 {
			r := []rune(a.state.InputBuffer)
			a.state.InputBuffer = string(r[:len(r)-1])
		}
		a.renderAll()
	default:
		if event.Rune() != 0 {
			a.state.InputBuffer += string(event.Rune())
			a.renderAll()
		}
	}
}

func (a *App) handleContextKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		a.switchHome(a.state.ContextSelectedIndex)
		a.state.ContextMode = false
		a.renderAll()
	case tcell.KeyDown:
		if a.state.ContextSelectedIndex < len(a.homes)-1 {
			a.state.ContextSelectedIndex++
			a.renderAll()
		}
	case tcell.KeyUp:
		if a.state.ContextSelectedIndex > 0 {
			a.state.ContextSelectedIndex--
			a.renderAll()
		}
	}
}

// ─────────────────────────────────────────────── Table selection ──

func (a *App) bindTableSelect() {
	a.table.SetSelectedFunc(func(row, col int) {
		// Enter on an entity row → activate
		if row > 0 {
			a.state.SelectedIndex = row - 1
			a.activateSelected()
		}
	})
	a.table.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 {
			a.state.SelectedIndex = row - 1
			a.renderDetailView()
			a.renderCommandBar()
		}
	})
}

// ─────────────────────────────────────────────── Actions ──

func (a *App) moveSelection(delta int) {
	n := len(a.state.FilteredEntities)
	if n == 0 {
		return
	}
	idx := a.state.SelectedIndex + delta
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	a.setSelection(idx)
}

func (a *App) setSelection(idx int) {
	a.state.SelectedIndex = idx
	if idx >= 0 && idx < len(a.state.FilteredEntities) {
		// +1 for header row
		a.table.Select(idx+1, 0)
	}
	a.renderDetailView()
	a.renderCommandBar()
}

func (a *App) toggleSelected() {
	e := a.selectedEntity()
	if e == nil {
		return
	}
	go func() {
		if err := a.client.ToggleEntity(e.EntityID); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Toggle failed: "+err.Error(), true)
			})
		}
	}()
}

func (a *App) activateSelected() {
	e := a.selectedEntity()
	if e == nil {
		return
	}
	go func() {
		if _, err := a.client.ActivateEntity(e.EntityID); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Activate failed: "+err.Error(), true)
			})
		}
	}()
}

func (a *App) handleAdjust(direction int) {
	e := a.selectedEntity()
	if e == nil {
		return
	}
	domain := entityDomain(e.EntityID)
	go func() {
		var err error
		switch domain {
		case "light":
			_, err = a.client.AdjustBrightness(e.EntityID, direction*26)
		case "climate":
			err = a.client.AdjustTemperature(e.EntityID, float64(direction)*0.5)
		case "fan":
			_, err = a.client.AdjustFanSpeed(e.EntityID, direction)
		case "media_player":
			err = a.client.AdjustVolume(e.EntityID, float64(direction)*0.1)
		case "number", "input_number":
			err = a.client.AdjustNumber(e.EntityID, direction)
		case "select", "input_select":
			err = a.client.CycleSelectOption(e.EntityID, direction)
		}
		if err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Adjust failed: "+err.Error(), true)
			})
		}
	}()
}

func (a *App) mediaCommand(cmd string) {
	e := a.selectedEntity()
	if e == nil || entityDomain(e.EntityID) != "media_player" {
		return
	}
	go func() {
		if err := a.client.MediaPlayerCommand(e.EntityID, cmd); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast(err.Error(), true)
			})
		}
	}()
}

func (a *App) handleSpace() {
	e := a.selectedEntity()
	if e == nil {
		return
	}
	switch entityDomain(e.EntityID) {
	case "media_player":
		a.mediaCommand("media_play_pause")
	case "vacuum":
		// Start if docked/idle, return_to_base if cleaning
		action := "start"
		if e.State == "cleaning" {
			action = "return_to_base"
		}
		go func() {
			if err := a.client.VacuumCommand(e.EntityID, action); err != nil {
				a.tapp.QueueUpdateDraw(func() {
					a.showToast(err.Error(), true)
				})
			}
		}()
	default:
		a.toggleSelected()
	}
}

func (a *App) handleCycleMode() {
	e := a.selectedEntity()
	if e == nil {
		return
	}
	switch entityDomain(e.EntityID) {
	case "climate":
		go func() {
			if err := a.client.CycleHvacMode(e.EntityID); err != nil {
				a.tapp.QueueUpdateDraw(func() {
					a.showToast(err.Error(), true)
				})
			}
		}()
	case "select", "input_select":
		// Treat m as cycle-next for selects too
		go func() {
			if err := a.client.CycleSelectOption(e.EntityID, 1); err != nil {
				a.tapp.QueueUpdateDraw(func() {
					a.showToast(err.Error(), true)
				})
			}
		}()
	}
}

func (a *App) coverCommand(action string) {
	e := a.selectedEntity()
	if e == nil || entityDomain(e.EntityID) != "cover" {
		return
	}
	go func() {
		if err := a.client.ControlCover(e.EntityID, action); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast(err.Error(), true)
			})
		}
	}()
}

func (a *App) activateRecentArea(n int) {
	if n <= 0 || n > len(a.state.RecentAreas) {
		return
	}
	areaName := a.state.RecentAreas[n-1]
	a.state.AreaFilter = areaName
	a.applyFilter()
	a.renderAll()
}

func (a *App) toggleHelp() {
	a.state.HelpMode = !a.state.HelpMode
	if a.state.HelpMode {
		a.helpView.SetText(renderHelp())
		a.pages.ShowPage("help")
	} else {
		a.pages.HidePage("help")
	}
}

func (a *App) openContextSwitcher() {
	a.state.ContextMode = true
	a.state.ContextSelectedIndex = a.state.ActiveHomeIndex
	a.renderAll()
}

func (a *App) switchHome(idx int) {
	if idx < 0 || idx >= len(a.homes) {
		return
	}
	a.state.ActiveHomeIndex = idx
	a.state.Entities = nil
	a.state.FilteredEntities = nil
	a.state.Connected = false
	a.renderAll()

	go func() {
		cfg := a.homes[idx]
		if err := a.client.Connect(cfg); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Connect failed: "+err.Error(), true)
			})
			return
		}
		a.tapp.QueueUpdateDraw(func() {
			a.state.Connected = true
			a.state.Entities = clientEntities(a.client)
			a.buildAreaMap()
			a.applyFilter()
			a.renderAll()
		})
	}()
}

func (a *App) commitInput() {
	e := a.selectedEntity()
	if e == nil || a.state.InputMode == nil {
		a.state.InputMode = nil
		a.state.InputBuffer = ""
		a.renderAll()
		return
	}
	mode := *a.state.InputMode
	buf := a.state.InputBuffer
	a.state.InputMode = nil
	a.state.InputBuffer = ""

	go func() {
		deviceID := a.getDeviceIDForEntity(e.EntityID)
		var err error
		switch mode {
		case model.InputModeRename:
			if deviceID != "" {
				err = a.client.RenameDevice(deviceID, buf)
			}
		case model.InputModeArea:
			areaID := a.resolveAreaID(buf)
			if deviceID != "" && areaID != "" {
				err = a.client.AssignDeviceArea(deviceID, areaID)
			}
		}
		if err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Failed: "+err.Error(), true)
			})
		}
	}()
	a.renderAll()
}

func (a *App) bulkPowerFiltered(action string) {
	ids := make([]string, len(a.state.FilteredEntities))
	for i, e := range a.state.FilteredEntities {
		ids[i] = e.EntityID
	}
	if len(ids) == 0 {
		a.showToast("No entities in current view", true)
		return
	}
	go func() {
		if err := a.client.BulkPower(ids, action); err != nil {
			a.tapp.QueueUpdateDraw(func() {
				a.showToast("Bulk "+action+" failed: "+err.Error(), true)
			})
		}
	}()
}

func (a *App) executeCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	// Context / bulk commands (single keyword)
	switch strings.ToLower(strings.SplitN(cmd, " ", 2)[0]) {
	case "home", "homes", "ctx", "context":
		a.openContextSwitcher()
		return
	case "q", "quit":
		a.tapp.Stop()
		return
	case "on":
		a.bulkPowerFiltered("on")
		return
	case "off":
		a.bulkPowerFiltered("off")
		return
	}

	// Backward-compat: if the first word is a known device-type shortcut, treat as :type [area]
	firstWord := strings.SplitN(cmd, " ", 2)[0]
	if dt, ok := model.DeviceTypeShortcuts[firstWord]; ok {
		rest := strings.TrimSpace(cmd[len(firstWord):])
		a.state.CurrentView = dt
		a.state.AreaFilter = rest
		if rest != "" {
			a.addRecentArea(rest)
		}
		a.applyFilter()
		return
	}

	// Primary: area-first syntax.
	// Find the longest prefix of cmd that exactly matches a known area name,
	// then treat the remainder as an optional device-type shortcut.
	areaMatch, typeToken := a.resolveAreaAndType(cmd)
	if areaMatch != "" {
		a.state.AreaFilter = areaMatch
		a.addRecentArea(areaMatch)
		if typeToken != "" {
			if dt, ok := model.DeviceTypeShortcuts[typeToken]; ok {
				a.state.CurrentView = dt
			}
		}
		a.applyFilter()
		return
	}

	// Fallback: treat the whole command as a free-form area filter
	a.state.AreaFilter = cmd
	a.addRecentArea(cmd)
	a.applyFilter()
}

// resolveAreaAndType finds the longest prefix of cmd that is an exact
// (case-insensitive) area name and returns (areaName, remainder).
func (a *App) resolveAreaAndType(cmd string) (string, string) {
	cmdLow := strings.ToLower(cmd)
	best := ""
	for _, ar := range a.client.Areas {
		nl := strings.ToLower(ar.Name)
		if strings.HasPrefix(cmdLow, nl) {
			rest := strings.TrimSpace(cmd[len(nl):])
			// Must be end of string or followed by a space
			if len(nl) == len(cmd) || cmd[len(nl)] == ' ' {
				if len(nl) > len(best) {
					best = ar.Name
				}
			}
			_ = rest
		}
	}
	if best == "" {
		return "", ""
	}
	remainder := strings.TrimSpace(cmd[len(best):])
	return best, remainder
}

func (a *App) refresh() {
	go func() {
		entities := clientEntities(a.client)
		a.tapp.QueueUpdateDraw(func() {
			a.state.Entities = entities
			now := time.Now()
			a.state.LastRefresh = &now
			a.applyFilter()
			a.renderAll()
		})
	}()
}

// ─────────────────────────────────────────────── Autocomplete ──

func (a *App) updateAutocomplete() {
	if a.state.CommandMode {
		a.state.AutocompleteSuggestions = render.ComputeCommandSuggestions(
			a.state.CommandBuffer, a.state.Areas, a.homes)
	} else if a.state.FilterMode {
		a.state.AutocompleteSuggestions = render.ComputeFilterSuggestions(
			a.state.Filter, a.state.Entities, a.areaMap)
	} else if a.state.InputMode != nil && *a.state.InputMode == model.InputModeArea {
		a.state.AutocompleteSuggestions = render.ComputeAreaSuggestions(
			a.state.InputBuffer, a.state.Areas)
	} else {
		a.state.AutocompleteSuggestions = nil
	}
	a.state.AutocompleteIndex = -1
}

func (a *App) navigateAutocomplete(delta int) {
	n := len(a.state.AutocompleteSuggestions)
	if n == 0 {
		return
	}
	a.state.AutocompleteIndex = (a.state.AutocompleteIndex + delta + n) % n
	suggestion := a.state.AutocompleteSuggestions[a.state.AutocompleteIndex]
	if a.state.CommandMode {
		a.state.CommandBuffer = suggestion
	} else if a.state.FilterMode {
		a.state.Filter = suggestion
		a.applyFilter()
	}
	a.renderAll()
}

// ─────────────────────────────────────────────── State helpers ──

func (a *App) applyFilter() {
	a.state.FilteredEntities = render.FilterEntities(
		a.state.Entities,
		a.state.CurrentView,
		a.state.Filter,
		a.areaMap,
		a.state.AreaFilter,
	)
	if a.state.SelectedIndex >= len(a.state.FilteredEntities) {
		a.state.SelectedIndex = max(0, len(a.state.FilteredEntities)-1)
	}
}

func (a *App) syncEntity(evt client.StateChangeEvent) {
	if evt.NewState == nil {
		// Entity removed
		for i, e := range a.state.Entities {
			if e.EntityID == evt.EntityID {
				a.state.Entities = append(a.state.Entities[:i], a.state.Entities[i+1:]...)
				break
			}
		}
		return
	}
	for i, e := range a.state.Entities {
		if e.EntityID == evt.EntityID {
			a.state.Entities[i] = *evt.NewState
			return
		}
	}
	a.state.Entities = append(a.state.Entities, *evt.NewState)
}

func (a *App) buildAreaMap() {
	areaName := make(map[string]string)
	for _, ar := range a.client.Areas {
		areaName[ar.AreaID] = ar.Name
	}
	a.state.Areas = a.client.Areas

	deviceArea := make(map[string]string)
	for _, d := range a.client.Devices {
		if d.AreaID != nil {
			deviceArea[d.ID] = areaName[*d.AreaID]
		}
	}

	for _, reg := range a.client.EntityRegistry {
		if reg.AreaID != nil {
			a.areaMap[reg.EntityID] = areaName[*reg.AreaID]
		} else if reg.DeviceID != nil {
			a.areaMap[reg.EntityID] = deviceArea[*reg.DeviceID]
		}
	}
}

func (a *App) selectedEntity() *model.HassEntity {
	if a.state.SelectedIndex < 0 || a.state.SelectedIndex >= len(a.state.FilteredEntities) {
		return nil
	}
	e := a.state.FilteredEntities[a.state.SelectedIndex]
	return &e
}

func (a *App) getDeviceIDForEntity(entityID string) string {
	for _, reg := range a.client.EntityRegistry {
		if reg.EntityID == entityID && reg.DeviceID != nil {
			return *reg.DeviceID
		}
	}
	return ""
}

func (a *App) resolveAreaID(areaName string) string {
	nameLow := strings.ToLower(areaName)
	for _, a2 := range a.client.Areas {
		if strings.ToLower(a2.Name) == nameLow {
			return a2.AreaID
		}
	}
	return ""
}

func (a *App) addRecentArea(name string) {
	for _, r := range a.state.RecentAreas {
		if r == name {
			return
		}
	}
	a.state.RecentAreas = append([]string{name}, a.state.RecentAreas...)
	if len(a.state.RecentAreas) > 5 {
		a.state.RecentAreas = a.state.RecentAreas[:5]
	}
}

// ─────────────────────────────────────────────── Rendering ──

func (a *App) renderAll() {
	a.renderHeader()
	a.renderTableView()
	a.renderDetailView()
	a.renderCommandBar()
	a.renderStatusBar()
}

func (a *App) renderSplash() {
	splash := fmt.Sprintf(
		"[%s]HOM3[-]\n[%s]Connecting to Home Assistant…[-]",
		clr.COLORS.Cyan, clr.COLORS.TextSecondary,
	)
	a.header.SetText(splash)
}

func (a *App) renderHeader() {
	// Line 1: title + connection + home name + entity count
	connStr := fmt.Sprintf("[%s]● CONNECTED[-]", clr.COLORS.Green)
	if !a.state.Connected {
		connStr = fmt.Sprintf("[%s]○ DISCONNECTED[-]", clr.COLORS.Red)
	}
	homeName := ""
	if a.state.ActiveHomeIndex < len(a.homes) {
		homeName = a.homes[a.state.ActiveHomeIndex].Name
	}
	line1 := fmt.Sprintf("[%s][::b]HOM3[-][-]  %s  [%s]%s[-]  [%s]%d entities[-]",
		clr.COLORS.Cyan, connStr,
		clr.COLORS.Yellow, homeName,
		clr.COLORS.TextSecondary, len(a.state.FilteredEntities),
	)

	// Line 2: recent areas
	line2 := ""
	for i, area := range a.state.RecentAreas {
		if i >= 5 {
			break
		}
		line2 += fmt.Sprintf("[%s]%d[%s]:%s[-]  ", clr.COLORS.Cyan, i+1, clr.COLORS.TextSecondary, area)
	}

	// Line 3: view shortcuts
	views := []string{"all", "lights", "switches", "sensors", "climate", "media_players", "automations"}
	var parts []string
	for _, v := range views {
		col := clr.COLORS.TextDim
		if string(a.state.CurrentView) == v {
			col = clr.COLORS.Cyan
		}
		parts = append(parts, fmt.Sprintf("[%s]:%s[-]", col, v))
	}
	line3 := strings.Join(parts, "  ")

	// Line 4: area filter indicator
	line4 := ""
	if a.state.AreaFilter != "" {
		line4 = fmt.Sprintf("[%s]area: %s[-]", clr.COLORS.Yellow, a.state.AreaFilter)
	}

	a.header.SetText(line1 + "\n" + line2 + "\n" + line3 + "\n" + line4)
}

func (a *App) renderTableView() {
	a.table.Clear()

	// Header row
	headers := []string{"", "NAME", "STATE", "AREA", "AGE"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan))).
			SetAttributes(tcell.AttrBold).
			SetExpansion(colExpansion(col)).
			SetMaxWidth(colMax(col))
		a.table.SetCell(0, col, cell)
	}

	for row, e := range a.state.FilteredEntities {
		icon := clr.DomainIcon(e.EntityID)
		name := clr.FriendlyName(e)
		state := clr.FormatState(e)
		area := a.areaMap[e.EntityID]
		age := clr.TimeSince(e.LastChanged)

		stateToken := clr.StateColor(e.State)
		stateHex := clr.TokenColor(stateToken)

		a.table.SetCell(row+1, 0, tview.NewTableCell(icon).SetMaxWidth(2))
		a.table.SetCell(row+1, 1, tview.NewTableCell(name).SetExpansion(3).SetMaxWidth(30))
		a.table.SetCell(row+1, 2,
			tview.NewTableCell(state).
				SetTextColor(tcell.NewHexColor(hexToInt(stateHex))).
				SetExpansion(2).SetMaxWidth(22))
		a.table.SetCell(row+1, 3, tview.NewTableCell(area).SetExpansion(1).SetMaxWidth(16))
		a.table.SetCell(row+1, 4,
			tview.NewTableCell(age).
				SetTextColor(tcell.NewHexColor(hexToInt(clr.COLORS.TextDim))).
				SetMaxWidth(5))
	}

	// Re-apply selection
	if a.state.SelectedIndex >= 0 && a.state.SelectedIndex < len(a.state.FilteredEntities) {
		a.table.Select(a.state.SelectedIndex+1, 0)
	}
}

func (a *App) renderDetailView() {
	e := a.selectedEntity()
	if e == nil || !a.state.DetailVisible {
		a.detail.SetText("")
		return
	}

	lines := []string{}
	cyan := clr.COLORS.Cyan
	dim := clr.COLORS.TextDim
	primary := clr.COLORS.TextPrimary

	icon := clr.DomainIcon(e.EntityID)
	name := clr.FriendlyName(*e)
	lines = append(lines, fmt.Sprintf("[%s][::b]%s %s[-][-]", cyan, icon, name))
	lines = append(lines, "")

	kv := func(k, v string) string {
		return fmt.Sprintf("[%s]%-14s[-] [%s]%s[-]", dim, k, primary, v)
	}
	stateStr := clr.FormatState(*e)
	stateToken := clr.StateColor(e.State)
	stateHex := clr.TokenColor(stateToken)
	lines = append(lines, fmt.Sprintf("[%s]%-14s[-] [%s]%s[-]", dim, "state", stateHex, stateStr))
	lines = append(lines, kv("entity_id", e.EntityID))
	lines = append(lines, kv("domain", entityDomain(e.EntityID)))
	lines = append(lines, kv("area", a.areaMap[e.EntityID]))
	lines = append(lines, kv("changed", e.LastChanged))
	lines = append(lines, kv("updated", e.LastUpdated))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("[%s]─── attributes ───[-]", dim))

	// Sort attribute keys
	keys := make([]string, 0, len(e.Attributes))
	for k := range e.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fmt.Sprintf("%v", e.Attributes[k])
		if len(v) > 40 {
			v = v[:37] + "..."
		}
		lines = append(lines, kv(k, v))
	}

	a.detail.SetText(strings.Join(lines, "\n"))
}

func (a *App) domainHint() string {
	cy := clr.COLORS.Cyan
	dim := clr.COLORS.TextDim
	key := func(k, desc string) string {
		return fmt.Sprintf("[%s]%s[-][%s]%s[-]", cy, k, dim, desc)
	}
	base := fmt.Sprintf("%s  %s  %s  %s  %s",
		key("?", " help"), key(":", " cmd"), key("/", " filter"), key("t", " toggle"), key("q", " quit"))

	e := a.selectedEntity()
	if e == nil {
		return base
	}
	switch entityDomain(e.EntityID) {
	case "light":
		return base + fmt.Sprintf("  %s  %s", key("+/-", " brightness"), key("Enter", " activate"))
	case "climate":
		return base + fmt.Sprintf("  %s  %s", key("+/-", " temp"), key("m", " hvac mode"))
	case "cover":
		return base + fmt.Sprintf("  %s  %s  %s", key("o", " open"), key("c", " close"), key("s", " stop"))
	case "media_player":
		return base + fmt.Sprintf("  %s  %s  %s  %s", key("Space", " play/pause"), key("+/-", " vol"), key("[/]", " track"), key("Enter", " activate"))
	case "fan":
		return base + fmt.Sprintf("  %s", key("+/-", " speed"))
	case "vacuum":
		return base + fmt.Sprintf("  %s  %s", key("Space", " start/dock"), key("Enter", " activate"))
	case "select", "input_select":
		return base + fmt.Sprintf("  %s  %s", key("+/-", " option"), key("m", " next"))
	case "number", "input_number":
		return base + fmt.Sprintf("  %s", key("+/-", " value"))
	case "scene", "script", "button", "input_button":
		return base + fmt.Sprintf("  %s", key("Enter", " activate"))
	default:
		return base
	}
}

func (a *App) renderCommandBar() {
	var line1 string
	switch {
	case a.state.ContextMode:
		line1 = fmt.Sprintf("[%s]Context:[%s] use ↑↓ + Enter to switch home, Esc to cancel[-][-]",
			clr.COLORS.Magenta, clr.COLORS.TextSecondary)
	case a.state.CommandMode:
		line1 = fmt.Sprintf("[%s]:[%s]%s[-][-]",
			clr.COLORS.Cyan, clr.COLORS.TextPrimary, a.state.CommandBuffer)
	case a.state.FilterMode:
		line1 = fmt.Sprintf("[%s]/[%s]%s[-][-]",
			clr.COLORS.Cyan, clr.COLORS.TextPrimary, a.state.Filter)
	case a.state.InputMode != nil:
		label := "rename"
		if *a.state.InputMode == model.InputModeArea {
			label = "area"
		}
		line1 = fmt.Sprintf("[%s]%s>[%s]%s[-][-]",
			clr.COLORS.Yellow, label, clr.COLORS.TextPrimary, a.state.InputBuffer)
	default:
		line1 = a.domainHint()
	}

	// Build suggestion line
	var line2 string
	if len(a.state.AutocompleteSuggestions) > 0 {
		parts := make([]string, len(a.state.AutocompleteSuggestions))
		for i, s := range a.state.AutocompleteSuggestions {
			if i == a.state.AutocompleteIndex {
				parts[i] = fmt.Sprintf("[%s::b]%s[-:-:-]", clr.COLORS.Cyan, s)
			} else {
				parts[i] = fmt.Sprintf("[%s]%s[-]", clr.COLORS.TextDim, s)
			}
		}
		line2 = "  " + strings.Join(parts, "  ")
	}

	a.cmdBar.SetText(line1 + "\n" + line2)
}

func (a *App) renderStatusBar() {
	conn := "LIVE"
	connColor := clr.COLORS.Green
	if !a.state.Connected {
		conn = "OFFLINE"
		connColor = clr.COLORS.Red
	}
	e := a.selectedEntity()
	entityInfo := ""
	if e != nil {
		entityInfo = fmt.Sprintf("  [%s]%s[-]", clr.COLORS.TextSecondary, e.EntityID)
	}
	text := fmt.Sprintf("[%s]%s[-]%s  [%s]%d/%d[-]",
		connColor, conn, entityInfo,
		clr.COLORS.TextDim, a.state.SelectedIndex+1, len(a.state.FilteredEntities))
	a.sttBar.SetText(text)
}

// ─────────────────────────────────────────────── Toast ──

func (a *App) showToast(msg string, isError bool) {
	col := clr.COLORS.Green
	if isError {
		col = clr.COLORS.Red
	}
	a.toast.SetText(fmt.Sprintf("[%s]%s[-]", col, msg))
	a.pages.AddPage("toast", a.toast, true, true)

	go func() {
		time.Sleep(3 * time.Second)
		a.tapp.QueueUpdateDraw(func() {
			a.pages.RemovePage("toast")
		})
	}()
}

// ─────────────────────────────────────────────── Help content ──

func renderHelp() string {
	c := clr.COLORS.Cyan
	return fmt.Sprintf(`[%s][::b]HOM3 Key Bindings[-][-]

[%s]Navigation[-]
  j / ↓       Move down
  k / ↑       Move up
  g / Home    Jump to top
  G / End     Jump to bottom
  PgUp/PgDn   Scroll 20 rows

[%s]Actions[-]
  Enter       Activate entity (scenes, scripts, buttons)
  t           Toggle entity on/off
  Space       Play/pause (media), start/dock (vacuum), toggle (others)
  O / I       Bulk turn off / on all visible entities
  d           Toggle detail panel
  r           Refresh
  C           Switch home (context)
  q           Quit

[%s]Device Controls[-]
  + / =       Adjust brightness/temp/speed/volume up
  - / _       Adjust brightness/temp/speed/volume down
  m           Cycle HVAC mode (climate) or next option (select)
  o / c / s   Cover open / close / stop
  [ / ]       Media previous / next track
  1-5         Jump to recent area

[%s]Modes[-]
  :           Command mode
  /           Filter mode (fuzzy search)
  ?           Toggle this help
  Esc         Exit current mode

[%s]Commands[:] (area-first)[-]
  :bedroom          All devices in bedroom
  :bedroom lights   Lights in bedroom
  :lights           All lights (no area filter)
  :on / :off        Bulk turn on/off visible entities
  :home / :ctx      Switch home
  :all :sensors :climate :covers :fans
  :media :auto :locks :vacuums :persons

[%s]? to close[-]`,
		c, c, c, c, c, c, c)
}

// ─────────────────────────────────────────────── Helpers ──

func clientEntities(c *client.HassClient) []model.HassEntity {
	es := make([]model.HassEntity, 0, len(c.Entities))
	for _, e := range c.Entities {
		es = append(es, *e)
	}
	return es
}

func entityDomain(entityID string) string {
	dot := strings.IndexByte(entityID, '.')
	if dot < 0 {
		return entityID
	}
	return entityID[:dot]
}

func hexToInt(hex string) int32 {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 0
	}
	var r, g, b int32
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return (r << 16) | (g << 8) | b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// centeredBox wraps a primitive in a centered Flex overlay.
func centeredBox(p tview.Primitive, w, h int) *tview.Flex {
	return tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(p, h, 1, true).
			AddItem(tview.NewBox(), 0, 1, false),
			w, 1, true).
		AddItem(tview.NewBox(), 0, 1, false)
}

// colExpansion returns the tview table column expansion factor.
func colExpansion(col int) int {
	switch col {
	case 1:
		return 3
	case 2:
		return 2
	case 3:
		return 1
	}
	return 0
}

// colMax returns the tview table column max width.
func colMax(col int) int {
	switch col {
	case 0:
		return 2
	case 1:
		return 30
	case 2:
		return 22
	case 3:
		return 16
	case 4:
		return 5
	}
	return 0
}
