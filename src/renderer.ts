import { HassEntity, DeviceType, DEVICE_TYPE_DOMAINS, AppState } from './types';
import {
  COLORS,
  domainIcon,
  friendlyName,
  formatState,
  stateColor,
  timeSince,
} from './theme';

// ─────────────────────────────────────────────────────────────────────────────
// Logo / header art
// ─────────────────────────────────────────────────────────────────────────────

export const LOGO_ART = [
  '{bold}{#00e5ff-fg}██╗  ██╗ █████╗ ████████╗██╗   ██╗██╗{/}',
  '{bold}{#00e5ff-fg}██║  ██║██╔══██╗╚══██╔══╝██║   ██║██║{/}',
  '{bold}{#ff00ff-fg}███████║███████║   ██║   ██║   ██║██║{/}',
  '{bold}{#ff00ff-fg}██╔══██║██╔══██║   ██║   ██║   ██║██║{/}',
  '{bold}{#aa44ff-fg}██║  ██║██║  ██║   ██║   ╚██████╔╝██║{/}',
  '{bold}{#aa44ff-fg}╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝{/}',
].join('\n');

// ─────────────────────────────────────────────────────────────────────────────
// Header render
// ─────────────────────────────────────────────────────────────────────────────

export function renderHeader(state: AppState, width: number): string {
  const connIcon = state.connected
    ? `{${COLORS.green}-fg}◉ CONNECTED{/}`
    : `{${COLORS.red}-fg}◉ OFFLINE{/}`;

  const viewLabel = `{bold}{${COLORS.cyan}-fg}${state.currentView.toUpperCase()}{/}`;
  const countLabel = `{${COLORS.textSecondary}-fg}${state.filteredEntities.length} entities{/}`;
  const time = new Date().toLocaleTimeString('en-US', { hour12: false });
  const timeLabel = `{${COLORS.textDim}-fg}${time}{/}`;

  const title = `{bold}{${COLORS.magenta}-fg} HA{/}{bold}{${COLORS.cyan}-fg}TUI{/}`;
  const sep = `{${COLORS.border}-fg}│{/}`;

  // On narrow terminals skip the entity count and spacer arithmetic.
  if (width < 80) {
    return ` ${title}  ${sep}  ${viewLabel}  ${sep}  ${connIcon}  ${timeLabel} `;
  }

  const spacer = ' '.repeat(Math.max(0, Math.floor(width / 2) - 20));
  const right = `${countLabel}  ${connIcon}  ${timeLabel}`;

  return ` ${title}  ${sep}  ${viewLabel}${spacer}${right} `;
}

// ─────────────────────────────────────────────────────────────────────────────
// Entity table row
// ─────────────────────────────────────────────────────────────────────────────

const COL_ICON = 2;

// Compute column widths that fit within the table's inner pixel budget.
// innerWidth = floor(screenWidth * 0.70) − 4  (border + padding)
function computeCols(innerWidth: number): { name: number; state: number; area: number; age: number } {
  if (innerWidth >= 80) return { name: 30, state: 22, area: 16, age: 5 };
  if (innerWidth >= 65) return { name: 24, state: 16, area: 12, age: 4 };
  if (innerWidth >= 50) return { name: 20, state: 12, area:  0, age: 0 };
  if (innerWidth >= 36) return { name: 16, state:  8, area:  0, age: 0 };
  return { name: Math.max(8, innerWidth - 14), state: 6, area: 0, age: 0 };
}

export function renderTableHeader(tableInnerWidth: number): string {
  const cols = computeCols(tableInnerWidth);
  let row = ` ${pad('', COL_ICON)}  ${pad('NAME', cols.name)}  ${pad('STATE', cols.state)}`;
  if (cols.area > 0) row += `  ${pad('AREA', cols.area)}`;
  if (cols.age  > 0) row += `  ${pad('AGE',  cols.age)}`;
  return `{bold}{${COLORS.textDim}-fg}${row}{/}`;
}

export function renderEntityRow(
  entity: HassEntity,
  areaMap: Map<string, string>,
  selected: boolean,
  tableInnerWidth: number
): string {
  const cols = computeCols(tableInnerWidth);
  const icon     = domainIcon(entity.entity_id);
  const name     = truncate(friendlyName(entity), cols.name);
  const stateStr = truncate(formatState(entity), cols.state);
  const stateCol = stateColor(entity.state);
  const nameColor  = selected ? COLORS.cyan : COLORS.textPrimary;
  const iconColor  = domainColorForEntity(entity.entity_id);

  let row =
    ` {${iconColor}-fg}${icon}{/}` +
    `  {${nameColor}-fg}${pad(name, cols.name)}{/}` +
    `  {${stateCol}-fg}${pad(stateStr, cols.state)}{/}`;

  if (cols.area > 0) {
    const area = truncate(areaMap.get(entity.entity_id) ?? '—', cols.area);
    row += `  {${COLORS.textSecondary}-fg}${pad(area, cols.area)}{/}`;
  }
  if (cols.age > 0) {
    row += `  {${COLORS.textDim}-fg}${pad(timeSince(entity.last_changed), cols.age)}{/}`;
  }
  return row;
}

function domainColorForEntity(entityId: string): string {
  const domain = entityId.split('.')[0];
  const domainColors: Record<string, string> = {
    light:              COLORS.yellow,
    switch:             COLORS.teal,
    sensor:             COLORS.blue,
    binary_sensor:      COLORS.purple,
    climate:            COLORS.orange,
    cover:              COLORS.cyan,
    fan:                COLORS.teal,
    media_player:       COLORS.magenta,
    automation:         COLORS.green,
    script:             COLORS.green,
    scene:              COLORS.purple,
    person:             COLORS.cyan,
    device_tracker:     COLORS.cyan,
    camera:             COLORS.yellow,
    lock:               COLORS.orange,
    vacuum:             COLORS.teal,
    alarm_control_panel:COLORS.red,
    weather:            COLORS.blue,
    button:             COLORS.teal,
    input_boolean:      COLORS.teal,
    number:             COLORS.blue,
    input_number:       COLORS.blue,
    select:             COLORS.purple,
    input_select:       COLORS.purple,
  };
  return domainColors[domain] ?? COLORS.textSecondary;
}

// ─────────────────────────────────────────────────────────────────────────────
// Detail panel render
// ─────────────────────────────────────────────────────────────────────────────

export function renderDetail(entity: HassEntity | null, panelInnerWidth = 30): string {
  if (!entity) {
    return `\n\n{center}{${COLORS.textDim}-fg}Select an entity{/}{/center}`;
  }

  const domain = entity.entity_id.split('.')[0];
  const icon = domainIcon(entity.entity_id);
  const iconColor = domainColorForEntity(entity.entity_id);
  const stateCol = stateColor(entity.state);

  // Fit label/value columns to the available panel width.
  const w      = Math.max(16, panelInnerWidth);
  const keyW   = Math.min(14, Math.max(6, Math.floor(w * 0.42)));
  const valW   = Math.max(6, w - keyW - 1);
  const hrLen  = Math.min(26, w);

  const lines: string[] = [
    `{bold}{${iconColor}-fg}${icon} ${truncate(friendlyName(entity), w - 2)}{/}`,
    `{${COLORS.border}-fg}${'─'.repeat(hrLen)}{/}`,
    `{${COLORS.textSecondary}-fg}${truncate('entity', keyW).padEnd(keyW)} {/}{${COLORS.textPrimary}-fg}${truncate(entity.entity_id, valW)}{/}`,
    `{${COLORS.textSecondary}-fg}${truncate('domain', keyW).padEnd(keyW)} {/}{${COLORS.cyan}-fg}${truncate(domain, valW)}{/}`,
    `{${COLORS.textSecondary}-fg}${truncate('state', keyW).padEnd(keyW)}  {/}{${stateCol}-fg}${truncate(formatState(entity), valW)}{/}`,
    `{${COLORS.textSecondary}-fg}${truncate('changed', keyW).padEnd(keyW)} {/}{${COLORS.textDim}-fg}${truncate(timeSince(entity.last_changed) + ' ago', valW)}{/}`,
    `{${COLORS.textSecondary}-fg}${truncate('updated', keyW).padEnd(keyW)} {/}{${COLORS.textDim}-fg}${truncate(timeSince(entity.last_updated) + ' ago', valW)}{/}`,
    '',
    `{bold}{${COLORS.textSecondary}-fg}ATTRIBUTES{/}`,
    `{${COLORS.border}-fg}${'─'.repeat(hrLen)}{/}`,
  ];

  const attrs = entity.attributes;
  const skip = new Set(['friendly_name', 'icon', 'entity_picture']);
  for (const [key, val] of Object.entries(attrs)) {
    if (skip.has(key)) continue;
    const valStr = typeof val === 'object' ? JSON.stringify(val) : String(val);
    lines.push(`{${COLORS.textSecondary}-fg}${truncate(key, keyW).padEnd(keyW)} {/}{${COLORS.textPrimary}-fg}${truncate(valStr, valW)}{/}`);
  }

  lines.push('');
  lines.push(`{bold}{${COLORS.textSecondary}-fg}ACTIONS{/}`);
  lines.push(`{${COLORS.border}-fg}${'─'.repeat(26)}{/}`);

  const toggleable = ['light', 'switch', 'fan', 'cover', 'media_player', 'lock', 'automation', 'input_boolean'];
  if (toggleable.includes(domain)) {
    lines.push(`{${COLORS.green}-fg}[t]{/} Toggle`);
  }
  lines.push(`{${COLORS.yellow}-fg}[y]{/} Copy entity_id`);
  lines.push(`{${COLORS.cyan}-fg}[?]{/} Help`);

  return lines.join('\n');
}

// ─────────────────────────────────────────────────────────────────────────────
// Command bar
// ─────────────────────────────────────────────────────────────────────────────

export function renderCommandBar(state: AppState, termWidth = 120): string {
  if (state.commandMode) {
    const buf = state.commandBuffer;
    return `{bold}{${COLORS.cyan}-fg}:{/}{${COLORS.textPrimary}-fg}${buf}{/}{${COLORS.cyan}-fg}█{/}`;
  }
  if (state.filter) {
    return `{${COLORS.yellow}-fg}/${/}{${COLORS.textPrimary}-fg}${state.filter}{/}{${COLORS.yellow}-fg}│{/} {${COLORS.textDim}-fg}ESC to clear{/}`;
  }
  // Shorten the hint string on narrow terminals so it doesn't wrap.
  if (termWidth < 80) {
    return `{${COLORS.textDim}-fg}/:filter  :view  t:toggle  d:describe  ?:help  q:quit{/}`;
  }
  return (
    `{${COLORS.textDim}-fg}/:filter  ` +
    `:view  ` +
    `t:toggle  ` +
    `l:logs  ` +
    `d:describe  ` +
    `r:refresh  ` +
    `?:help  ` +
    `q:quit{/}`
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Status bar
// ─────────────────────────────────────────────────────────────────────────────

export function renderStatusBar(state: AppState): string {
  // Always show connection errors, even before any entities load.
  if (state.error) {
    return `{${COLORS.red}-fg}⚠  ${state.error}{/}`;
  }

  const sel = state.filteredEntities[state.selectedIndex];
  if (!sel) return '';

  const domain = sel.entity_id.split('.')[0];
  const stateCol = stateColor(sel.state);

  return (
    `{${COLORS.textDim}-fg}${domain} {/}` +
    `{${COLORS.textSecondary}-fg}» {/}` +
    `{${COLORS.textPrimary}-fg}${friendlyName(sel)} {/}` +
    `{${COLORS.textDim}-fg}│ {/}` +
    `{${stateCol}-fg}${sel.state}{/}`
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Help content
// ─────────────────────────────────────────────────────────────────────────────

export function renderHelp(): string {
  const h = (s: string) => `{bold}{${COLORS.magenta}-fg}${s}{/}`;
  const k = (s: string) => `{bold}{${COLORS.cyan}-fg}${s}{/}`;
  const d = (s: string) => `{${COLORS.textSecondary}-fg}${s}{/}`;
  const dim = (s: string) => `{${COLORS.textDim}-fg}${s}{/}`;

  return [
    `{center}{bold}{${COLORS.magenta}-fg}  HATUI – Home Assistant TUI  {/}{/center}`,
    `{center}{${COLORS.textDim}-fg}k9s-inspired terminal UI{/}{/center}`,
    '',
    h('── NAVIGATION ──────────────────'),
    ` ${k('↑ / k')}      ${d('Move up')}`,
    ` ${k('↓ / j')}      ${d('Move down')}`,
    ` ${k('g / Home')}   ${d('Jump to top')}`,
    ` ${k('G / End')}    ${d('Jump to bottom')}`,
    ` ${k('PgUp/PgDn')}  ${d('Page up/down')}`,
    '',
    h('── VIEWS (:command) ────────────'),
    ` ${k(':all')}        ${d('All entities')}`,
    ` ${k(':lights')}     ${d('Lights')}`,
    ` ${k(':switches')}   ${d('Switches')}`,
    ` ${k(':sensors')}    ${d('Sensors')}`,
    ` ${k(':climate')}    ${d('Climate')}`,
    ` ${k(':covers')}     ${d('Covers')}`,
    ` ${k(':fans')}       ${d('Fans')}`,
    ` ${k(':media')}      ${d('Media players')}`,
    ` ${k(':automations')} ${d('Automations')}`,
    ` ${k(':locks')}      ${d('Locks')}`,
    ` ${k(':cameras')}    ${d('Cameras')}`,
    ` ${k(':vacuums')}    ${d('Vacuums')}`,
    '',
    h('── ACTIONS ─────────────────────'),
    ` ${k('t')}          ${d('Toggle entity')}`,
    ` ${k('/')}          ${d('Filter entities (fuzzy)')}`,
    ` ${k('d')}          ${d('Describe / inspect entity')}`,
    ` ${k('r')}          ${d('Refresh states')}`,
    ` ${k('y')}          ${d('Copy entity_id to clipboard')}`,
    ` ${k('?')}          ${d('Toggle this help')}`,
    ` ${k('q / ctrl+c')} ${d('Quit')}`,
    '',
    dim('Press ? or ESC to close'),
  ].join('\n');
}

// ─────────────────────────────────────────────────────────────────────────────
// Entity filtering
// ─────────────────────────────────────────────────────────────────────────────

export function filterEntities(
  entities: HassEntity[],
  view: DeviceType,
  filter: string
): HassEntity[] {
  let result = entities;

  if (view !== 'all') {
    const domains = DEVICE_TYPE_DOMAINS[view];
    result = result.filter((e) => domains.includes(e.entity_id.split('.')[0]));
  }

  if (filter) {
    const q = filter.toLowerCase();
    result = result.filter(
      (e) =>
        e.entity_id.toLowerCase().includes(q) ||
        friendlyName(e).toLowerCase().includes(q) ||
        e.state.toLowerCase().includes(q)
    );
  }

  return result;
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function pad(str: string, len: number): string {
  if (str.length >= len) return str.substring(0, len);
  return str + ' '.repeat(len - str.length);
}

function truncate(str: string, len: number): string {
  if (str.length <= len) return str;
  return str.substring(0, len - 1) + '…';
}
