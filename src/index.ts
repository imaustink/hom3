#!/usr/bin/env node
import * as path from 'path';
import * as fs from 'fs';
import { HassClient } from './hass-client';
import { App } from './app';
import { HassConfig } from './types';

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

  ${'\x1b[2m'}Usage:${'\x1b[0m'}
    hatui [--url <url>] [--token <token>]

  ${'\x1b[2m'}Configuration (in order of precedence):${'\x1b[0m'}
    1. CLI flags:           --url http://homeassistant.local:8123 --token <your_token>
    2. Environment:         HASS_URL=...  HASS_TOKEN=...
    3. Config file:         ~/.config/hatui/config.json

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
  const app = new App(client);

  await app.start();
}

main().catch((err) => {
  console.error('\n  ✖ Fatal error:', (err as Error).message);
  process.exit(1);
});
