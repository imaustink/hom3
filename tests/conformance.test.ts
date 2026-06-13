/**
 * ─────────────────────────────────────────────────────────────────────────────
 * Cross-language conformance suite
 * ─────────────────────────────────────────────────────────────────────────────
 *
 * This suite does NOT test TypeScript-specific behaviour. Instead it loads the
 * language-agnostic JSON specification corpus in `spec/fixtures/` and asserts the
 * current implementation conforms to it.
 *
 * The same JSON fixtures are the contract a migration target (any language /
 * framework) must satisfy. To port HOM3, re-implement a runner like this one in
 * the new language that consumes the identical fixtures. When every fixture
 * passes in both runtimes, the behaviour has been preserved.
 *
 * See spec/README.md for the full porting guide.
 */
import * as fs from 'fs';
import * as path from 'path';

import { COLORS, stateColor, domainIcon, friendlyName, formatState, timeSince } from '../src/theme';
import { DEVICE_TYPE_DOMAINS, DEVICE_TYPE_SHORTCUTS, HassEntity, HassArea, HassConfig } from '../src/types';
import {
  filterEntities,
  computeCommandSuggestions,
  computeAreaSuggestions,
  computeFilterSuggestions,
} from '../src/renderer';
import { parseCliArgs } from '../src/cli';
import { HassClient } from '../src/hass-client';

const FIXTURES = path.join(__dirname, '..', 'spec', 'fixtures');
function load<T = any>(name: string): T {
  return JSON.parse(fs.readFileSync(path.join(FIXTURES, name), 'utf-8')) as T;
}

type Case = Record<string, any>;

/** Build a full HassEntity from a partial spec entity. */
function entityFrom(partial: Record<string, any>): HassEntity {
  return {
    entity_id: partial.entity_id,
    state: partial.state ?? 'on',
    attributes: partial.attributes ?? {},
    last_changed: partial.last_changed ?? '2025-01-01T00:00:00.000Z',
    last_updated: partial.last_updated ?? '2025-01-01T00:00:00.000Z',
    context: partial.context ?? { id: 'ctx', parent_id: null, user_id: null },
  };
}

// ─────────────────────────────────────────────────────────────────────────────
// theme: stateColor
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: stateColor', () => {
  const cases = load('state-color.json').cases as Case[];
  it.each(cases)('state "$input" -> token $expect', ({ input, expect: token }) => {
    expect(stateColor(input)).toBe((COLORS as Record<string, string>)[token]);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// theme: domainIcon
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: domainIcon', () => {
  const cases = load('domain-icon.json').cases as Case[];
  it.each(cases)('$input -> $expect', ({ input, expect: glyph }) => {
    expect(domainIcon(input)).toBe(glyph);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// theme: friendlyName
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: friendlyName', () => {
  const cases = load('friendly-name.json').cases as Case[];
  it.each(cases)('$entity.entity_id', ({ entity, expect: name }) => {
    expect(friendlyName(entity)).toBe(name);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// theme: formatState
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: formatState', () => {
  const cases = load('format-state.json').cases as Case[];
  it.each(cases)('$entity.entity_id "$entity.state"', ({ entity, expect: out }) => {
    expect(formatState(entity)).toBe(out);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// theme: timeSince
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: timeSince', () => {
  const spec = load('time-since.json') as { now: string; cases: Case[] };

  beforeEach(() => {
    jest.useFakeTimers();
    jest.setSystemTime(new Date(spec.now));
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it.each(spec.cases)('$input -> $expect', ({ input, expect: out }) => {
    expect(timeSince(input)).toBe(out);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// renderer: filterEntities
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: filterEntities', () => {
  const spec = load('filter-entities.json') as {
    entities: HassEntity[];
    areaMap: Record<string, string>;
    cases: Case[];
  };
  const areaMap = new Map<string, string>(Object.entries(spec.areaMap));

  it.each(spec.cases)(
    'view=$view filter="$filter" area="$areaFilter"',
    ({ view, filter, areaFilter, expect: expected }) => {
      const result = filterEntities(spec.entities, view, filter, areaMap, areaFilter);
      expect(result.map((e) => e.entity_id)).toEqual(expected);
    }
  );
});

// ─────────────────────────────────────────────────────────────────────────────
// renderer: autocomplete
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: autocomplete', () => {
  const spec = load('autocomplete.json');

  describe('computeCommandSuggestions', () => {
    const section = spec.command as { areas: HassArea[]; homes: HassConfig[]; cases: Case[] };
    it.each(section.cases)('buffer="$buffer"', ({ buffer, expect: expected }) => {
      expect(computeCommandSuggestions(buffer, section.areas, section.homes)).toEqual(expected);
    });
  });

  describe('computeAreaSuggestions', () => {
    const section = spec.area as { areas: HassArea[]; cases: Case[] };
    it.each(section.cases)('buffer="$buffer"', ({ buffer, expect: expected }) => {
      expect(computeAreaSuggestions(buffer, section.areas)).toEqual(expected);
    });
  });

  describe('computeFilterSuggestions', () => {
    const section = spec.filter as { entities: HassEntity[]; cases: Case[] };
    it.each(section.cases)('query="$query"', ({ query, expect: expected }) => {
      expect(computeFilterSuggestions(query, section.entities, new Map(), [])).toEqual(expected);
    });
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// types: device-type maps
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: device-type maps', () => {
  const spec = load('device-types.json');

  it('DEVICE_TYPE_DOMAINS matches the spec exactly', () => {
    expect(DEVICE_TYPE_DOMAINS).toEqual(spec.deviceTypeDomains);
  });

  it('DEVICE_TYPE_SHORTCUTS matches the spec exactly', () => {
    expect(DEVICE_TYPE_SHORTCUTS).toEqual(spec.deviceTypeShortcuts);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// cli: parseCliArgs
// ─────────────────────────────────────────────────────────────────────────────

describe('conformance: parseCliArgs', () => {
  const cases = load('cli-args.json').cases as Case[];
  it.each(cases)('args=$args', ({ args, expect: expected }) => {
    expect(parseCliArgs(args)).toEqual(expected);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// hass-client: service-call computation
// ─────────────────────────────────────────────────────────────────────────────

interface ServiceCall {
  domain: string;
  service: string;
  service_data: Record<string, unknown>;
}

describe('conformance: service-call computation', () => {
  const cases = load('service-calls.json').cases as Case[];

  it.each(cases)('$name', async (testCase) => {
    const client = new HassClient({ url: 'http://localhost:8123', token: 'test' });

    // Seed the entity store when the case provides an entity.
    if (testCase.entity) {
      client.entities.set(testCase.entity.entity_id, entityFrom(testCase.entity));
    }

    const captured: ServiceCall[] = [];
    jest
      .spyOn(client, 'callService')
      .mockImplementation(async (domain: string, service: string, serviceData: Record<string, unknown> = {}) => {
        captured.push({ domain, service, service_data: serviceData });
      });

    const method = (client as unknown as Record<string, (...a: unknown[]) => unknown>)[testCase.method];
    const ret = await method.apply(client, testCase.args);

    if (Array.isArray(testCase.expectCalls)) {
      expect(captured).toEqual(testCase.expectCalls);
    } else if (testCase.expectCall === null) {
      expect(captured).toHaveLength(0);
    } else {
      expect(captured).toHaveLength(1);
      expect(captured[0]).toEqual(testCase.expectCall);
    }

    if (Object.prototype.hasOwnProperty.call(testCase, 'expectReturn')) {
      expect(ret).toEqual(testCase.expectReturn);
    }
  });
});
