# HATUI

> k9s-inspired terminal UI for Home Assistant

```
██╗  ██╗ █████╗ ████████╗██╗   ██╗██╗
██║  ██║██╔══██╗╚══██╔══╝██║   ██║██║
███████║███████║   ██║   ██║   ██║██║
██╔══██║██╔══██║   ██║   ██║   ██║██║
██║  ██║██║  ██║   ██║   ╚██████╔╝██║
╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝
```

Navigate your entire Home Assistant setup from the terminal — just like `k9s` for Kubernetes.

---

## Features

- **Resource-based navigation** — jump to any device type with `:lights`, `:sensors`, `:climate`, etc.
- **Live state streaming** — subscribes to HA WebSocket, updates in real-time
- **Fuzzy filter** — press `/` to filter entities by name, entity_id, or state
- **Toggle** — press `t` to toggle lights, switches, fans, locks, and more
- **Detail panel** — attributes, actions, and timing for the selected entity
- **Cyberpunk/synthwave theme** — cyan, magenta, neon green on dark

---

## Quick Start

```bash
# 1. Clone & install
git clone https://github.com/you/hatui
cd hatui
npm install

# 2. Configure
mkdir -p ~/.config/hatui
cat > ~/.config/hatui/config.json << 'EOF'
{
  "url": "http://homeassistant.local:8123",
  "token": "your_long_lived_access_token"
}
EOF

# 3. Run
npm run dev
# or after building:
npm run build && npm start
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
| `:` | Open command mode |
| `/` | Filter (fuzzy search) |
| `t` | Toggle selected entity |
| `r` | Refresh all states |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

---

## View Commands

Type `:` then any of the following:

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
| `:quit` | Quit |

---

## Project Structure

```
src/
├── index.ts        # Entry point, config loading
├── app.ts          # Main app controller, key bindings, event loop
├── hass-client.ts  # Home Assistant WebSocket client
├── renderer.ts     # All rendering logic (rows, detail, bars)
├── widgets.ts      # blessed widget factories
├── theme.ts        # Colors, icons, state formatters
└── types.ts        # TypeScript types
```

---

## Configuration

HATUI loads config in this order of precedence:

| Source | Example |
|--------|---------|
| CLI flags | `--url http://homeassistant.local:8123 --token <token>` |
| Environment variables | `HASS_URL=...` `HASS_TOKEN=...` |
| Config file | `~/.config/hatui/config.json` |

```json
// ~/.config/hatui/config.json
{
  "url": "http://homeassistant.local:8123",
  "token": "your_long_lived_access_token"
}
```
