# hom3

k9s-inspired terminal UI for Home Assistant

![HOM3 Screenshot](img/app.png)

Navigate your entire Home Assistant setup from the terminal — just like `k9s` for Kubernetes.

---

## Features

- **Resource-based navigation** — jump to any device type with `:lights`, `:sensors`, `:climate`, etc.
- **Live state streaming** — subscribes to HA WebSocket, updates in real-time
- **Area filtering** — filter by area with `:lights kitchen` or use the area selector in the header
- **Fuzzy filter** — press `/` to filter entities by name, entity_id, or state
- **Toggle & control** — `t` toggles lights/switches/fans/locks; `+`/`-` adjusts brightness, temperature, volume
- **Detail panel** — attributes, domain-specific controls, and timing for the selected entity
- **Multi-home context switching** — manage multiple HA instances and switch between them with `C` or `:homes`
- **Cyberpunk/synthwave theme** — cyan, magenta, neon green on dark

---

## Quick Start

### Install pre-built binary (recommended)

Download the latest binary for your platform from the [Releases](https://github.com/imaustink/hom3/releases) page, then move it onto your `PATH`:

```bash
# Example for macOS arm64
curl -L https://github.com/imaustink/hom3/releases/latest/download/hom3-darwin-arm64 \
  -o /tmp/hom3 && sudo mv /tmp/hom3 /usr/local/bin/hom3 && sudo chmod +x /usr/local/bin/hom3
```

### Install with Go

Requires Go 1.23+:

```bash
go install github.com/imaustink/hom3@latest
```

The binary is installed as `hom3` in `$GOPATH/bin`.

### Build from source

```bash
git clone https://github.com/imaustink/hom3
cd hom3
make build          # produces dist/hom3
sudo mv dist/hom3 /usr/local/bin/hom3
```

### Configure

```bash
mkdir -p ~/.config/hom3
cat > ~/.config/hom3/config.json << 'EOF'
{
  "homes": [
    {
      "name": "Home",
      "url": "http://homeassistant.local:8123",
      "token": "your_long_lived_access_token"
    }
  ]
}
EOF
```

Get a token: **HA → Profile → Long-Lived Access Tokens**

---

## Keybindings

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `g` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `PgUp` / `PgDn` | Page up/down |
| `Enter` | Activate selected entity (scenes, scripts, buttons, vacuums) |
| `t` | Toggle selected entity on/off |
| `Space` | Play/pause (media player), start/dock (vacuum), toggle (others) |
| `+` / `=` | Adjust brightness, temperature, volume, fan speed up |
| `-` / `_` | Adjust brightness, temperature, volume, fan speed down |
| `m` | Cycle HVAC mode (climate) or next option (select/input_select) |
| `o` / `c` / `s` | Cover: open / close / stop |
| `[` / `]` | Media: previous / next track |
| `O` | Bulk turn **off** all currently visible entities |
| `I` | Bulk turn **on** all currently visible entities |
| `d` | Toggle detail panel |
| `n` | Rename selected entity |
| `a` | Set area for selected entity |
| `r` | Refresh all states |
| `1`–`5` | Jump to recent area |
| `C` | Open context switcher (multi-home) |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

---

## View Commands

Type `:` to enter command mode. Commands are **area-first**: type an area name, optionally followed by a device type.

### Area + type examples

| Command | Effect |
|---------|--------|
| `:bedroom` | All devices in bedroom |
| `:bedroom lights` | Lights in bedroom |
| `:back porch` | All devices in back porch |
| `:back porch switches` | Switches in back porch |

### Device-type shortcuts (no area filter)

| Command | View |
|---------|------|
| `:all` | All entities |
| `:lights` | Lights |
| `:switches` | Switches |
| `:sensors` | Sensors |
| `:binary_sensors` / `:bs` | Binary sensors |
| `:climate` | Climate |
| `:covers` | Covers |
| `:fans` | Fans |
| `:media` / `:media_players` | Media players |
| `:automations` / `:auto` | Automations |
| `:scripts` | Scripts |
| `:scenes` | Scenes |
| `:locks` | Locks |
| `:cameras` | Cameras |
| `:vacuums` | Vacuums |
| `:alarms` | Alarm panels |
| `:weather` | Weather |
| `:buttons` | Buttons |
| `:numbers` | Number inputs |
| `:selects` | Select inputs |
| `:inputs` | Input helpers |

### Bulk & utility commands

| Command | Effect |
|---------|--------|
| `:on` | Turn on all currently visible entities |
| `:off` | Turn off all currently visible entities |
| `:homes` / `:ctx` | Context switcher (multi-home) |
| `:quit` | Quit |

---

## Multi-Home Context Switching

HOM3 supports multiple Home Assistant instances, similar to how k9s handles multiple Kubernetes clusters.

**Open the context switcher** with `C` or `:homes` — the main table switches to a `contexts` view listing all configured homes. Navigate with `j`/`k`, press `Enter` to connect, `Esc` to cancel.

The active home name is shown as a badge in the header (top-right of the title bar).

---

## Configuration

HOM3 loads config in this order of precedence:

| Source | Variables / Flags |
|--------|-------------------|
| CLI flags | `--url`, `--token`, `--name` |
| Environment variables | `HASS_URL`, `HASS_TOKEN`, `HASS_NAME` |
| Config file | `~/.config/hom3/config.json` |

### Single home

```json
{
  "url": "http://homeassistant.local:8123",
  "token": "your_long_lived_access_token"
}
```

### Multiple homes

```json
{
  "homes": [
    {
      "name": "Home",
      "url": "http://homeassistant.local:8123",
      "token": "your_long_lived_access_token"
    },
    {
      "name": "Cabin",
      "url": "http://192.168.1.100:8123",
      "token": "another_token"
    }
  ]
}
```

---

## Project Structure

```
main.go             # Entry point
cmd/
├── root.go         # Root cobra command, TUI launch
├── get.go          # `hom3 get` subcommand
└── control.go      # `hom3 control` subcommand
internal/
├── client/         # Home Assistant WebSocket client
├── config/         # Config loading (flags → env → file)
├── model/          # Entity, area, device types
├── color/          # Theme colours and state formatters
├── render/         # Pure rendering functions
├── ui/             # tview widget factories
└── view/           # App controller, key bindings, event loop
```

