import { parseCliArgs, CliArgs } from '../src/cli';

// ─────────────────────────────────────────────────────────────────────────────
// parseCliArgs
// ─────────────────────────────────────────────────────────────────────────────

describe('parseCliArgs', () => {
  // ── TUI mode detection ──────────────────────────────────────────────────

  it('returns null when no positional arguments are given (TUI mode)', () => {
    expect(parseCliArgs([])).toBeNull();
  });

  it('returns null for only flag arguments', () => {
    expect(parseCliArgs(['--domain', 'light'])).toBeNull();
    expect(parseCliArgs(['-o', 'json'])).toBeNull();
  });

  // ── Basic subcommand parsing ────────────────────────────────────────────

  it('parses a single positional as subcommand', () => {
    const result = parseCliArgs(['get']);
    expect(result).not.toBeNull();
    expect(result!.subcommand).toBe('get');
    expect(result!.resource).toBeUndefined();
    expect(result!.target).toBeUndefined();
  });

  it('parses subcommand + resource', () => {
    const result = parseCliArgs(['get', 'entities']);
    expect(result!.subcommand).toBe('get');
    expect(result!.resource).toBe('entities');
  });

  it('parses subcommand + resource + target', () => {
    const result = parseCliArgs(['get', 'entity', 'light.bedroom']);
    expect(result!.subcommand).toBe('get');
    expect(result!.resource).toBe('entity');
    expect(result!.target).toBe('light.bedroom');
  });

  it('parses "toggle" subcommand', () => {
    const result = parseCliArgs(['toggle', 'light.bedroom']);
    expect(result!.subcommand).toBe('toggle');
    expect(result!.resource).toBe('light.bedroom');
  });

  it('parses "turn-on" and "turn-off" subcommands', () => {
    expect(parseCliArgs(['turn-on', 'switch.kitchen'])!.subcommand).toBe('turn-on');
    expect(parseCliArgs(['turn-off', 'switch.kitchen'])!.subcommand).toBe('turn-off');
  });

  it('parses "call" subcommand with domain and service', () => {
    const result = parseCliArgs(['call', 'light', 'turn_on']);
    expect(result!.subcommand).toBe('call');
    expect(result!.resource).toBe('light');
    expect(result!.target).toBe('turn_on');
  });

  // ── Named flags ─────────────────────────────────────────────────────────

  it('parses --domain flag', () => {
    const result = parseCliArgs(['toggle', '--domain', 'light']);
    expect(result!.domain).toBe('light');
  });

  it('parses -d as shorthand for --domain', () => {
    const result = parseCliArgs(['toggle', '-d', 'switch']);
    expect(result!.domain).toBe('switch');
  });

  it('parses --search flag', () => {
    const result = parseCliArgs(['get', 'entities', '--search', 'bedroom']);
    expect(result!.search).toBe('bedroom');
  });

  it('parses -s as shorthand for --search', () => {
    const result = parseCliArgs(['get', 'entities', '-s', 'kitchen']);
    expect(result!.search).toBe('kitchen');
  });

  it('parses --area flag', () => {
    const result = parseCliArgs(['get', 'entities', '--area', 'Living Room']);
    expect(result!.area).toBe('Living Room');
  });

  it('parses -A as shorthand for --area', () => {
    const result = parseCliArgs(['toggle', '-A', 'Kitchen']);
    expect(result!.area).toBe('Kitchen');
  });

  it('parses --entity flag', () => {
    const result = parseCliArgs(['call', 'light', 'turn_on', '--entity', 'light.bedroom']);
    expect(result!.entity).toBe('light.bedroom');
  });

  it('parses -e as shorthand for --entity', () => {
    const result = parseCliArgs(['call', 'light', 'turn_on', '-e', 'light.x']);
    expect(result!.entity).toBe('light.x');
  });

  it('parses --data flag', () => {
    const result = parseCliArgs(['call', 'light', 'turn_on', '--data', '{"brightness":128}']);
    expect(result!.data).toBe('{"brightness":128}');
  });

  // ── Output format ────────────────────────────────────────────────────────

  it('defaults output to "table"', () => {
    const result = parseCliArgs(['get', 'entities']);
    expect(result!.output).toBe('table');
  });

  it('parses -o json', () => {
    const result = parseCliArgs(['get', 'entities', '-o', 'json']);
    expect(result!.output).toBe('json');
  });

  it('parses --output wide', () => {
    const result = parseCliArgs(['get', 'entities', '--output', 'wide']);
    expect(result!.output).toBe('wide');
  });

  it('parses --output table', () => {
    const result = parseCliArgs(['get', 'entities', '--output', 'table']);
    expect(result!.output).toBe('table');
  });

  it('ignores unknown output format and keeps default "table"', () => {
    const result = parseCliArgs(['get', 'entities', '-o', 'yaml']);
    expect(result!.output).toBe('table');
  });

  // ── Combined flags ───────────────────────────────────────────────────────

  it('parses a combination of flags and positional args', () => {
    const result = parseCliArgs([
      'get', 'entities',
      '--domain', 'light',
      '--area', 'Living Room',
      '-o', 'json',
    ]);
    expect(result!.subcommand).toBe('get');
    expect(result!.resource).toBe('entities');
    expect(result!.domain).toBe('light');
    expect(result!.area).toBe('Living Room');
    expect(result!.output).toBe('json');
  });

  it('flags between positionals are parsed correctly', () => {
    const result = parseCliArgs(['get', '-o', 'json', 'areas']);
    expect(result!.subcommand).toBe('get');
    expect(result!.resource).toBe('areas');
    expect(result!.output).toBe('json');
  });

  // ── Edge cases ──────────────────────────────────────────────────────────

  it('ignores flags that start with - that are not recognised', () => {
    const result = parseCliArgs(['get', 'entities', '--unknown-flag']);
    expect(result!.subcommand).toBe('get');
    // unknown flags are skipped; no crash
  });

  it('handles flag at end of argv without a value gracefully', () => {
    // --domain at end of args with no following value
    const result = parseCliArgs(['get', 'entities', '--domain']);
    expect(result!.domain).toBeUndefined();
  });
});
