# HOM3 Conformance Specification

This directory holds a **language- and framework-agnostic test corpus** that
captures the portable business logic of HOM3 as plain JSON. Its purpose is to
make migrating HOM3 to another language or framework safe and mechanical: re-implement
the logic, point a thin runner at these same fixtures, and you have a behavioral
guarantee that the port matches the original.

The fixtures are validated against the current TypeScript implementation by
[`tests/conformance.test.ts`](../tests/conformance.test.ts). That runner is also the
reference template for what a runner in the target language must do.

```
spec/
├── README.md            ← this file
└── fixtures/
    ├── state-color.json     state → semantic palette token
    ├── domain-icon.json     entity_id → glyph
    ├── friendly-name.json   entity → display name
    ├── format-state.json    entity → formatted state string
    ├── time-since.json      ISO timestamp → coarse age (with fixed "now")
    ├── filter-entities.json view + filter + area → matching entity ids
    ├── autocomplete.json    command / area / filter suggestion engines
    ├── device-types.json    device-type ↔ HA-domain maps + alias table
    ├── cli-args.json        argv → parsed command struct (or null = TUI mode)
    └── service-calls.json   entity + action → resulting HA service call(s)
```

## What is and isn't covered

These fixtures intentionally cover the **deterministic, presentation-independent
core** — the parts that must behave identically regardless of UI toolkit:

- State/value formatting and iconography (`theme`)
- Entity filtering and autocomplete logic (`renderer`)
- Device-type ↔ domain mapping and command aliases (`types`)
- CLI argument parsing (`cli`)
- Domain-aware service-call computation: toggle/activate dispatch, brightness,
  temperature, fan speed, volume, number, select cycling, HVAC cycling, alarm,
  cover, vacuum, media, and bulk power grouping (`hass-client`)

They deliberately **do not** assert exact `blessed` tag markup, widget geometry,
key-event plumbing, or WebSocket framing. Those are framework-specific concerns a
migration will re-design. The contracts above are what must be preserved.

## Fixture conventions

Each file is a JSON object with a top-level `description` and a `cases` array (or
named sub-sections, each with their own `cases`). Every case is an input plus an
`expect` value. Where a function reads ambient context (areas, homes, entities,
the entity store, or a reference clock), that context is provided alongside the
cases so the fixture is fully self-contained.

Notable encodings:

- **`state-color.json`** expresses the result as a *semantic palette token*
  (`stateOn`, `stateOff`, `stateUnavail`, `stateUnknown`, `textPrimary`) rather
  than a hex value, so each implementation maps tokens to its own theme.
- **`time-since.json`** pins a `now` timestamp; the runner must freeze the clock
  to that value before evaluating cases.
- **`service-calls.json`** cases name a `method` and `args`; the expected result
  is the service call(s) produced — `expectCall` (single, or `null` for no-op),
  `expectCalls` (ordered list, for bulk ops), and optional `expectReturn`.

## Porting workflow

1. Re-implement the logic module-by-module in the target language.
2. Write a small runner that, for each fixture file, loads the JSON, invokes the
   corresponding function/method with the case inputs and provided context, and
   asserts deep equality against `expect`.
3. Run it. Every case must pass. Treat any divergence as a port bug, not a
   fixture bug — the TypeScript reference already conforms (CI proves it).
4. When you intentionally change behavior, update the fixture **and** the
   TypeScript reference together so both runtimes stay in lockstep.

### Reference runner

The canonical runner is [`tests/conformance.test.ts`](../tests/conformance.test.ts).
Run just the conformance suite with:

```bash
npx jest tests/conformance.test.ts
```

Each fixture maps to one function under test:

| Fixture | Function (source) |
|---|---|
| `state-color.json` | `stateColor` (`src/theme.ts`) |
| `domain-icon.json` | `domainIcon` (`src/theme.ts`) |
| `friendly-name.json` | `friendlyName` (`src/theme.ts`) |
| `format-state.json` | `formatState` (`src/theme.ts`) |
| `time-since.json` | `timeSince` (`src/theme.ts`) |
| `filter-entities.json` | `filterEntities` (`src/renderer.ts`) |
| `autocomplete.json` | `computeCommandSuggestions` / `computeAreaSuggestions` / `computeFilterSuggestions` (`src/renderer.ts`) |
| `device-types.json` | `DEVICE_TYPE_DOMAINS` / `DEVICE_TYPE_SHORTCUTS` (`src/types.ts`) |
| `cli-args.json` | `parseCliArgs` (`src/cli.ts`) |
| `service-calls.json` | `HassClient` control methods (`src/hass-client.ts`) |
