#!/usr/bin/env node
import * as path from 'path';
import * as fs from 'fs';
import { HassClient } from './hass-client';
import { App } from './app';
import { HassConfig } from './types';
import { parseCliArgs, runCli } from './cli';

// ─────────────────────────────────────────────────────────────────────────────
// Load config: CLI flags → env vars → ~/.config/hatui/config.json
// ─────────────────────────────────────────────────────────────────────────────

const CONFIG_PATH = path.resolve(process.env['HOME'] ?? '~', '.config', 'hatui', 'config.json');

function loadJsonConfig(): Partial<HassConfig> {
  if (!fs.existsSync(CONFIG_PATH)) return {};
  try {
    return JSON.parse(fs.readFileSync(CONFIG_PATH, 'utf-8')) as Partial<HassConfig>;
  } catch {
    console.error(`  ✖ Failed to parse config file: ${CONFIG_PATH}`);
    return {};
  }
}

function getConfig(): HassConfig {
  const args = process.argv.slice(2);
  let url = '';
  let token = '';

  // 1. CLI flags
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--url' && args[i + 1]) url = args[++i];
    if (args[i] === '--token' && args[i + 1]) token = args[++i];
  }
  // 2. Environment variables
  if (!url) url = process.env['HASS_URL'] ?? process.env['HA_URL'] ?? '';
  if (!token) token = process.env['HASS_TOKEN'] ?? process.env['HA_TOKEN'] ?? '';

  // 3. ~/.config/hatui/config.json
  if (!url || !token) {
    const cfg = loadJsonConfig();
    if (cfg.url && !url) url = cfg.url;
    if (cfg.token && !token) token = cfg.token;
  }

  if (!url) {
    console.error('\n  ✖ Missing HASS_URL\n');
    printUsage();
    process.exit(1);
  }

  if (!token) {
    console.error('\n  ✖ Missing HASS_TOKEN\n');
    printUsage();
    process.exit(1);
  }

  // Normalize URL
  if (!url.startsWith('http')) url = `http://${url}`;
  url = url.replace(/\/$/, '');

  return { url, token };
}

function printUsage(): void {
  console.log(`
  ${'\x1b[96m'}HATUI${'\x1b[0m'} – k9s-inspired Home Assistant TUI

  ${'\x1b[2m'}Usage (interactive TUI):${'\x1b[0m'}
    hatui [--url <url>] [--token <token>]

  ${'\x1b[2m'}Usage (one-off commands):${'\x1b[0m'}
    hatui get entities  [--domain <type>] [--search <text>] [--area <name>] [-o table|wide|json]
    hatui get entity    <entity_id>       [-o table|json]
    hatui get areas     [-o table|json]
    hatui get devices   [-o table|json]
    hatui toggle        <entity_id>
    hatui toggle        --area <name> [--domain <domain>]
    hatui turn-on       <entity_id>
    hatui turn-on       --area <name> [--domain <domain>]
    hatui turn-off      <entity_id>
    hatui turn-off      --area <name> [--domain <domain>]
    hatui call          <domain> <service> [--entity <id>] [--data <json>]

  ${'\x1b[2m'}Area examples:${'\x1b[0m'}
    hatui turn-off --area "Living Room"           # turn off all lights
    hatui turn-off --area kitchen --domain switch  # turn off switches
    hatui turn-on  --area bedroom                 # partial name match

  ${'\x1b[2m'}Domain shortcuts for --domain:${'\x1b[0m'}
    lights, switches, sensors, bs (binary_sensors), climate, covers,
    fans, media (media_players), auto (automations), locks, cameras …

  ${'\x1b[2m'}Configuration (in order of precedence):${'\x1b[0m'}
    1. CLI flags:   --url http://homeassistant.local:8123 --token <token>
    2. Environment: HASS_URL=...  HASS_TOKEN=...
    3. Config file: ~/.config/hatui/config.json

  ${'\x1b[2m'}Config file format:${'\x1b[0m'}
    {
      "url": "http://homeassistant.local:8123",
      "token": "your_long_lived_access_token"
    }

  ${'\x1b[2m'}Get a token from:${'\x1b[0m'}
    Home Assistant → Profile → Long-Lived Access Tokens
`);
}

async function main(): Promise<void> {
  if (process.argv.includes('--help') || process.argv.includes('-h')) {
    printUsage();
    process.exit(0);
  }

  const config = getConfig();
  const client = new HassClient(config);

  // Strip connection flags from argv to get the remaining subcommands
  const rawArgs = process.argv.slice(2);
  const stripped: string[] = [];
  for (let i = 0; i < rawArgs.length; i++) {
    if ((rawArgs[i] === '--url' || rawArgs[i] === '--token') && rawArgs[i + 1]) {
      i++; // skip value
    } else {
      stripped.push(rawArgs[i]);
    }
  }

  const cliArgs = parseCliArgs(stripped);

  if (cliArgs) {
    // Non-interactive one-off command (kubectl-style)
    await runCli(client, cliArgs);
  } else {
    // Interactive TUI mode
    const app = new App(client);
    await app.start();
  }
}

main().catch((err) => {
  console.error('\n  ✖ Fatal error:', (err as Error).message);
  process.exit(1);
});
