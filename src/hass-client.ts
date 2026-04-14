import WebSocket from 'ws';
import { HassEntity, HassConfig, HassArea, HassDevice, HassStateChange } from './types';
import { EventEmitter } from 'events';

interface HassMessage {
  type: string;
  id?: number;
  [key: string]: unknown;
}

export class HassClient extends EventEmitter {
  private ws: WebSocket | null = null;
  private msgId = 1;
  private pendingRequests = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
  private subscribeId: number | null = null;
  public entities: Map<string, HassEntity> = new Map();
  public areas: HassArea[] = [];
  public devices: HassDevice[] = [];
  public connected = false;

  constructor(private config: HassConfig) {
    super();
  }

  private nextId(): number {
    return this.msgId++;
  }

  private send(msg: HassMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private request<T>(msg: Omit<HassMessage, 'id'>): Promise<T> {
    return new Promise((resolve, reject) => {
      const id = this.nextId();
      this.pendingRequests.set(id, {
        resolve: resolve as (v: unknown) => void,
        reject,
      });
      this.send({ ...(msg as HassMessage), id });
    });
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const wsUrl = this.config.url
        .replace(/^http/, 'ws')
        .replace(/\/$/, '') + '/api/websocket';

      this.ws = new WebSocket(wsUrl, {
        rejectUnauthorized: false,
      });

      const timeout = setTimeout(() => {
        reject(new Error('Connection timeout'));
        this.ws?.close();
      }, 10000);

      this.ws.on('error', (err) => {
        clearTimeout(timeout);
        reject(err);
      });

      this.ws.on('message', async (raw) => {
        let msg: HassMessage;
        try {
          msg = JSON.parse(raw.toString()) as HassMessage;
        } catch {
          return;
        }

        if (msg.type === 'auth_required') {
          this.send({ type: 'auth', access_token: this.config.token });
          return;
        }

        if (msg.type === 'auth_ok') {
          clearTimeout(timeout);
          this.connected = true;
          try {
            await this.bootstrap();
            resolve();
          } catch (e) {
            reject(e as Error);
          }
          return;
        }

        if (msg.type === 'auth_invalid') {
          clearTimeout(timeout);
          reject(new Error('Invalid Home Assistant token'));
          return;
        }

        if (msg.type === 'result') {
          const id = msg.id as number;
          const handler = this.pendingRequests.get(id);
          if (handler) {
            this.pendingRequests.delete(id);
            if (msg.success as boolean) {
              handler.resolve(msg.result);
            } else {
              handler.reject(new Error(JSON.stringify(msg.error)));
            }
          }
          return;
        }

        if (msg.type === 'event' && msg.id === this.subscribeId) {
          const event = msg.event as { data: HassStateChange };
          const stateChange = event.data;
          if (stateChange.new_state) {
            this.entities.set(stateChange.entity_id, stateChange.new_state);
          } else {
            this.entities.delete(stateChange.entity_id);
          }
          this.emit('state_changed', stateChange);
        }
      });

      this.ws.on('close', () => {
        this.connected = false;
        this.emit('disconnected');
      });
    });
  }

  private async bootstrap(): Promise<void> {
    // Load all states
    const states = await this.request<HassEntity[]>({ type: 'get_states' });
    for (const entity of states) {
      this.entities.set(entity.entity_id, entity);
    }

    // Load areas
    try {
      this.areas = await this.request<HassArea[]>({
        type: 'config/area_registry/list',
      });
    } catch {
      this.areas = [];
    }

    // Load devices
    try {
      this.devices = await this.request<HassDevice[]>({
        type: 'config/device_registry/list',
      });
    } catch {
      this.devices = [];
    }

    // Subscribe to state changes
    this.subscribeId = this.nextId();
    this.send({
      id: this.subscribeId,
      type: 'subscribe_events',
      event_type: 'state_changed',
    });
  }

  async callService(domain: string, service: string, serviceData: Record<string, unknown> = {}): Promise<void> {
    await this.request({
      type: 'call_service',
      domain,
      service,
      service_data: serviceData,
    });
  }

  async toggleEntity(entityId: string): Promise<void> {
    const domain = entityId.split('.')[0];
    const supportedToggle = ['light', 'switch', 'fan', 'cover', 'media_player', 'lock', 'automation', 'input_boolean'];
    if (supportedToggle.includes(domain)) {
      await this.callService(domain, 'toggle', { entity_id: entityId });
    }
  }

  getEntityList(): HassEntity[] {
    return Array.from(this.entities.values());
  }

  disconnect(): void {
    this.ws?.close();
  }
}
