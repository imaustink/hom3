import blessed from 'blessed';
import { HassClient } from './hass-client';
import {
  AppState,
  DeviceType,
  DEVICE_TYPE_SHORTCUTS,
  HassEntity,
} from './types';
import {
  createScreen,
  createHeader,
  createTable,
  createDetailPanel,
  createCommandBar,
  createStatusBar,
  createHelpOverlay,
  createToast,
} from './widgets';
import {
  renderHeader,
  renderCommandBar,
  renderDetail,
  renderEntityRow,
  renderTableHeader,
  renderStatusBar,
  renderHelp,
  filterEntities,
} from './renderer';
import { COLORS } from './theme';

export class App {
  private screen: blessed.Widgets.Screen;
  private header: blessed.Widgets.BoxElement;
  private table: blessed.Widgets.ListElement;
  private detail: blessed.Widgets.BoxElement;
  private commandBar: blessed.Widgets.BoxElement;
  private statusBar: blessed.Widgets.BoxElement;
  private helpOverlay: blessed.Widgets.BoxElement;
  private toast: blessed.Widgets.BoxElement;

  private state: AppState;
  private areaMap: Map<string, string> = new Map(); // entity_id → area name
  private refreshTimer: NodeJS.Timeout | null = null;
  private headerTimer: NodeJS.Timeout | null = null;

  constructor(private client: HassClient) {
    this.state = {
      currentView: 'all',
      filter: '',
      selectedIndex: 0,
      entities: [],
      filteredEntities: [],
      connected: false,
      commandMode: false,
      commandBuffer: '',
      describeMode: false,
      describeEntity: null,
      helpMode: false,
      areas: [],
      devices: [],
      sortField: 'friendly_name',
      sortAsc: true,
      error: null,
      lastRefresh: null,
    };

    this.screen = createScreen();
    this.header = createHeader(this.screen);
    this.table = createTable(this.screen);
    this.detail = createDetailPanel(this.screen);
    this.commandBar = createCommandBar(this.screen);
    this.statusBar = createStatusBar(this.screen);
    this.helpOverlay = createHelpOverlay(this.screen);
    this.toast = createToast(this.screen);

    this.bindKeys();
    this.bindClientEvents();
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Startup
  // ─────────────────────────────────────────────────────────────────────────

  async start(): Promise<void> {
    this.renderSplash();
    this.screen.render();

    try {
      await this.client.connect();
    } catch (err) {
      this.state.error = `Connection failed: ${(err as Error).message}`;
      this.renderAll();
      return;
    }

    this.state.connected = true;
    this.state.areas = this.client.areas;
    this.state.devices = this.client.devices;
    this.buildAreaMap();
    this.refreshEntities();

    // Refresh header clock every second
    this.headerTimer = setInterval(() => {
      this.renderHeaderBar();
      this.screen.render();
    }, 1000);
  }

  private renderSplash(): void {
    this.header.setContent(
      `{center}{bold}{${COLORS.cyan}-fg} HATUI{/} {${COLORS.magenta}-fg}─ Home Assistant TUI{/} {${COLORS.textDim}-fg}Connecting…{/}{/center}`
    );
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Area map: entity_id → area name
  // ─────────────────────────────────────────────────────────────────────────

  private buildAreaMap(): void {
    const areaById = new Map(this.client.areas.map((a) => [a.area_id, a.name]));

    // device_id → area_id from device registry
    const deviceAreaMap = new Map(
      this.client.devices
        .filter((d) => d.area_id)
        .map((d) => [d.id, d.area_id as string])
    );

    // We'd need entity registry for entity→device mapping, but we can
    // approximate using the areas on the entity state if available.
    // For now build a best-effort from attributes.
    for (const entity of this.client.getEntityList()) {
      const areaId = entity.attributes['area_id'] as string | undefined;
      if (areaId && areaById.has(areaId)) {
        this.areaMap.set(entity.entity_id, areaById.get(areaId)!);
      }
    }
  }

  // ─────────────────────────────────────────────────────────────────────────
  // State refresh
  // ─────────────────────────────────────────────────────────────────────────

  private refreshEntities(): void {
    this.state.entities = this.client.getEntityList().sort((a, b) => {
      const nameA = (a.attributes['friendly_name'] as string) ?? a.entity_id;
      const nameB = (b.attributes['friendly_name'] as string) ?? b.entity_id;
      return nameA.localeCompare(nameB);
    });
    this.state.lastRefresh = new Date();
    this.applyFilter();
  }

  private applyFilter(): void {
    const prev = this.state.filteredEntities[this.state.selectedIndex]?.entity_id;
    this.state.filteredEntities = filterEntities(
      this.state.entities,
      this.state.currentView,
      this.state.filter
    );

    // Restore selection
    if (prev) {
      const idx = this.state.filteredEntities.findIndex((e) => e.entity_id === prev);
      this.state.selectedIndex = idx >= 0 ? idx : 0;
    } else {
      this.state.selectedIndex = 0;
    }

    this.renderAll();
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Rendering
  // ─────────────────────────────────────────────────────────────────────────

  private renderAll(): void {
    this.renderHeaderBar();
    this.renderTableView();
    this.renderDetailView();
    this.renderCommandBarView();
    this.renderStatusBarView();
    this.screen.render();
  }

  private renderHeaderBar(): void {
    this.state.connected = this.client.connected;
    const w = (this.screen.width as number) ?? 120;
    this.header.setContent(renderHeader(this.state, w));
  }

  private renderTableView(): void {
    const screenWidth = (this.screen.width as number) ?? 120;
    const tableInnerWidth = Math.floor(screenWidth * 0.7) - 4;
    const items: string[] = [renderTableHeader(tableInnerWidth)];

    for (let i = 0; i < this.state.filteredEntities.length; i++) {
      const entity = this.state.filteredEntities[i];
      const selected = i === this.state.selectedIndex;
      items.push(renderEntityRow(entity, this.areaMap, selected, tableInnerWidth));
    }

    if (this.state.filteredEntities.length === 0) {
      items.push('');
      items.push(`{center}{${COLORS.textDim}-fg}No entities found{/}{/center}`);
    }

    this.table.setItems(items as unknown as string[]);
    // +1 because items[0] is the header row
    const targetIdx = Math.min(this.state.selectedIndex + 1, items.length - 1);
    this.table.select(targetIdx);
  }

  private renderDetailView(): void {
    const screenWidth = (this.screen.width as number) ?? 120;
    const panelInnerWidth = Math.floor(screenWidth * 0.3) - 4;
    const sel = this.state.filteredEntities[this.state.selectedIndex] ?? null;
    this.detail.setContent(renderDetail(sel, panelInnerWidth));
    this.detail.setLabel(
      ` {bold}{${COLORS.purple}-fg}  DESCRIBE{/} `
    );
  }

  private renderCommandBarView(): void {
    const screenWidth = (this.screen.width as number) ?? 120;
    this.commandBar.setContent(renderCommandBar(this.state, screenWidth));
  }

  private renderStatusBarView(): void {
    this.statusBar.setContent(renderStatusBar(this.state));
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Toast notification
  // ─────────────────────────────────────────────────────────────────────────

  private showToast(msg: string, isError = false): void {
    const color = isError ? COLORS.red : COLORS.green;
    this.toast.style.border = { fg: color };
    this.toast.style.fg = color;
    this.toast.setContent(`{center}${msg}{/center}`);
    this.toast.show();
    this.screen.render();
    setTimeout(() => {
      this.toast.hide();
      this.screen.render();
    }, 2500);
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Command execution
  // ─────────────────────────────────────────────────────────────────────────

  private executeCommand(cmd: string): void {
    const trimmed = cmd.trim().toLowerCase();

    if (!trimmed) return;

    if (trimmed === 'q' || trimmed === 'quit' || trimmed === 'exit') {
      this.quit();
      return;
    }

    const view = DEVICE_TYPE_SHORTCUTS[trimmed] as DeviceType | undefined;
    if (view) {
      this.state.currentView = view;
      this.state.filter = '';
      this.state.selectedIndex = 0;
      this.applyFilter();
      this.showToast(`View: ${view}`);
      return;
    }

    this.state.error = `Unknown command: ${trimmed}`;
    this.renderAll();
    setTimeout(() => {
      this.state.error = null;
      this.renderAll();
    }, 3000);
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Key bindings
  // ─────────────────────────────────────────────────────────────────────────

  private bindKeys(): void {
    const screen = this.screen;

    // Global quit
    screen.key(['C-c'], () => this.quit());

    screen.on('keypress', (_ch: string, key: blessed.Widgets.Events.IKeyEventArg) => {
      if (this.state.helpMode) {
        this.state.helpMode = false;
        this.helpOverlay.hide();
        this.renderAll();
        return;
      }

      // ── Command mode ────────────────────────────────────────────────────
      if (this.state.commandMode) {
        if (key.name === 'enter') {
          const cmd = this.state.commandBuffer;
          this.state.commandMode = false;
          this.state.commandBuffer = '';
          this.renderCommandBarView();
          screen.render();
          this.executeCommand(cmd);
        } else if (key.name === 'escape') {
          this.state.commandMode = false;
          this.state.commandBuffer = '';
          this.renderCommandBarView();
          screen.render();
        } else if (key.name === 'backspace') {
          this.state.commandBuffer = this.state.commandBuffer.slice(0, -1);
          this.renderCommandBarView();
          screen.render();
        } else if (_ch && !key.ctrl && !key.meta && key.name !== 'escape') {
          this.state.commandBuffer += _ch;
          this.renderCommandBarView();
          screen.render();
        }
        return;
      }

      // ── Filter mode (/) ─────────────────────────────────────────────────
      if (this.state.filter !== '' || key.name === '/') {
        if (key.name === '/') {
          // entered filter – handled below
        } else if (key.name === 'escape') {
          this.state.filter = '';
          this.applyFilter();
          return;
        } else if (key.name === 'backspace') {
          this.state.filter = this.state.filter.slice(0, -1);
          this.applyFilter();
          return;
        } else if (key.name === 'enter') {
          // commit filter, stop typing
          this.renderCommandBarView();
          screen.render();
          return;
        } else if (this.state.filter !== '' && _ch && !key.ctrl && !key.meta) {
          this.state.filter += _ch;
          this.applyFilter();
          return;
        }
      }

      const { filteredEntities, selectedIndex } = this.state;
      const count = filteredEntities.length;

      switch (key.name ?? _ch) {
        // ── Navigation ──
        case 'up':
        case 'k':
          this.state.selectedIndex = Math.max(0, selectedIndex - 1);
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        case 'down':
        case 'j':
          this.state.selectedIndex = Math.min(count - 1, selectedIndex + 1);
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        case 'g':
        case 'home':
          this.state.selectedIndex = 0;
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        case 'G':
        case 'end':
          this.state.selectedIndex = Math.max(0, count - 1);
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        case 'pageup':
          this.state.selectedIndex = Math.max(0, selectedIndex - 20);
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        case 'pagedown':
          this.state.selectedIndex = Math.min(count - 1, selectedIndex + 20);
          this.renderTableView();
          this.renderDetailView();
          this.renderStatusBarView();
          screen.render();
          break;

        // ── Command mode ──
        case ':':
          this.state.commandMode = true;
          this.state.commandBuffer = '';
          this.renderCommandBarView();
          screen.render();
          break;

        // ── Filter ──
        case '/':
          this.state.filter = '';
          this.renderCommandBarView();
          screen.render();
          break;

        // ── Toggle ──
        case 't': {
          const entity = filteredEntities[selectedIndex];
          if (entity) {
            this.client.toggleEntity(entity.entity_id).then(() => {
              this.showToast(`Toggled: ${entity.entity_id}`);
            }).catch((e: Error) => {
              this.showToast(`Error: ${e.message}`, true);
            });
          }
          break;
        }

        // ── Refresh ──
        case 'r':
          this.refreshEntities();
          this.showToast('Refreshed');
          break;

        // ── Help ──
        case '?':
          this.state.helpMode = true;
          this.helpOverlay.setContent(renderHelp());
          this.helpOverlay.show();
          this.helpOverlay.focus();
          screen.render();
          break;

        // ── Quit ──
        case 'q':
          this.quit();
          break;
      }
    });

    // Mouse selection in table
    this.table.on('select', (_item: blessed.Widgets.BoxElement, idx: number) => {
      // idx 0 is the header row
      if (idx > 0) {
        this.state.selectedIndex = idx - 1;
        this.renderDetailView();
        this.renderStatusBarView();
        this.screen.render();
      }
    });
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Client events
  // ─────────────────────────────────────────────────────────────────────────

  private bindClientEvents(): void {
    this.client.on('state_changed', (change: { entity_id: string; new_state: HassEntity | null }) => {
      // Update entity in local state list
      if (change.new_state) {
        const idx = this.state.entities.findIndex((e) => e.entity_id === change.entity_id);
        if (idx >= 0) {
          this.state.entities[idx] = change.new_state;
        } else {
          this.state.entities.push(change.new_state);
        }
      } else {
        this.state.entities = this.state.entities.filter((e) => e.entity_id !== change.entity_id);
      }
      this.applyFilter();
    });

    this.client.on('disconnected', () => {
      this.state.connected = false;
      this.state.error = 'Disconnected from Home Assistant';
      this.renderAll();
    });
  }

  // ─────────────────────────────────────────────────────────────────────────
  // Quit
  // ─────────────────────────────────────────────────────────────────────────

  private quit(): void {
    if (this.refreshTimer) clearInterval(this.refreshTimer);
    if (this.headerTimer) clearInterval(this.headerTimer);
    this.client.disconnect();
    this.screen.destroy();
    process.exit(0);
  }
}
