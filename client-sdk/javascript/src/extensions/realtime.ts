import type { LayrClient } from '../core/client';

export type RealtimeEvent = 'INSERT' | 'UPDATE' | 'DELETE' | '*';

export interface CDCMessage<T = Record<string, unknown>> {
  channel?: string;
  topic?: string;
  event: 'INSERT' | 'UPDATE' | 'DELETE';
  schema: string;
  table: string;
  commit_timestamp: string;
  record: T;
  old_record?: T;
  truncated?: boolean;
}

export type EventCallback<T = Record<string, unknown>> = (payload: CDCMessage<T>) => void;

export interface SubscribeOptions {
  schema?: string;
  filter?: string;
  channel?: string;
}

export class RealtimeChannel<T = Record<string, unknown>> {
  readonly channel: string;
  readonly schema: string;
  readonly table: string;
  private filter?: string;
  private client: RealtimeManager;
  private insertCallbacks: EventCallback<T>[] = [];
  private updateCallbacks: EventCallback<T>[] = [];
  private deleteCallbacks: EventCallback<T>[] = [];
  private allCallbacks: EventCallback<T>[] = [];

  constructor(client: RealtimeManager, table: string, options: SubscribeOptions = {}) {
    this.client = client;
    this.table = table;
    this.schema = options.schema || 'public';
    this.channel = options.channel || `${this.schema}:${this.table}`;
    this.filter = options.filter;
  }

  onInsert(callback: EventCallback<T>): this {
    this.insertCallbacks.push(callback);
    return this;
  }

  onUpdate(callback: EventCallback<T>): this {
    this.updateCallbacks.push(callback);
    return this;
  }

  onDelete(callback: EventCallback<T>): this {
    this.deleteCallbacks.push(callback);
    return this;
  }

  onAll(callback: EventCallback<T>): this {
    this.allCallbacks.push(callback);
    return this;
  }

  on(event: RealtimeEvent, callback: EventCallback<T>): this {
    if (event === 'INSERT') return this.onInsert(callback);
    if (event === 'UPDATE') return this.onUpdate(callback);
    if (event === 'DELETE') return this.onDelete(callback);
    return this.onAll(callback);
  }

  subscribe(): this {
    this.client.ensureConnected();
    return this;
  }

  _dispatch(message: CDCMessage<unknown>) {
    if (message.event === 'INSERT') {
      for (const cb of this.insertCallbacks) cb(message as CDCMessage<T>);
    } else if (message.event === 'UPDATE') {
      for (const cb of this.updateCallbacks) cb(message as CDCMessage<T>);
    } else if (message.event === 'DELETE') {
      for (const cb of this.deleteCallbacks) cb(message as CDCMessage<T>);
    }
    for (const cb of this.allCallbacks) cb(message as CDCMessage<T>);
  }

  getSubscribePayload(): Record<string, unknown> {
    return {
      action: 'subscribe',
      channel: this.channel,
      topic: this.channel,
      schema: this.schema,
      table: this.table,
      event: '*',
      filter: this.filter || '',
    };
  }

  unsubscribe(): void {
    this.client.unsubscribe(this.channel);
  }
}

export interface SubscribedChannel {
  getSubscribePayload(): Record<string, unknown>;
  _dispatch(message: CDCMessage<unknown>): void;
}

export class RealtimeManager {
  private layrClient: LayrClient;
  private ws?: WebSocket;
  private channels = new Map<string, SubscribedChannel>();
  private pingInterval?: ReturnType<typeof setInterval>;
  private reconnectTimer?: ReturnType<typeof setTimeout>;
  private isExplicitClose = false;

  constructor(layrClient: LayrClient) {
    this.layrClient = layrClient;
  }

  channel<T = Record<string, unknown>>(table: string, options: SubscribeOptions = {}): RealtimeChannel<T> {
    return this.subscribe<T>(table, options);
  }

  subscribe<T = Record<string, unknown>>(table: string, options: SubscribeOptions = {}): RealtimeChannel<T> {
    const parts = table.split('.');
    let schema = options.schema;
    let actualTable = table;
    if (parts.length > 1 && parts[0] && parts[1]) {
      schema = parts[0];
      actualTable = parts[1];
    }
    const channel = new RealtimeChannel<T>(this, actualTable, { ...options, schema });
    this.channels.set(channel.channel, channel);

    this.ensureConnected();
    if (this.ws && this.ws.readyState === 1 /* OPEN */) {
      this.ws.send(JSON.stringify(channel.getSubscribePayload()));
    }

    return channel;
  }

  unsubscribe(channelName: string): void {
    if (this.channels.has(channelName)) {
      this.channels.delete(channelName);
      if (this.ws && this.ws.readyState === 1) {
        this.ws.send(JSON.stringify({ action: 'unsubscribe', channel: channelName }));
      }
    }
  }

  ping(): void {
    if (this.ws && this.ws.readyState === 1) {
      this.ws.send(JSON.stringify({ event: 'ping', action: 'ping' }));
    }
  }

  ensureConnected(): void {
    if (this.ws && (this.ws.readyState === 0 || this.ws.readyState === 1)) {
      return;
    }
    this.isExplicitClose = false;
    const cleanToken = this.layrClient.sessionToken?.replace(/^Bearer\s+/i, '').trim();
    let wsURL = this.layrClient.endpoint.replace(/^http/i, 'ws') + '/v1/realtime';
    if (cleanToken) {
      wsURL += `?token=${encodeURIComponent(cleanToken)}`;
    }

    const WebSocketClass = (globalThis as unknown as { WebSocket?: typeof WebSocket }).WebSocket;
    if (!WebSocketClass) {
      return;
    }

    try {
      this.ws = new WebSocketClass(wsURL);
    } catch {
      return;
    }

    this.ws.onopen = () => {
      for (const channel of this.channels.values()) {
        this.ws?.send(JSON.stringify(channel.getSubscribePayload()));
      }
      this.startHeartbeat();
    };

    this.ws.onmessage = (event: MessageEvent<unknown>) => {
      try {
        const raw = typeof event.data === 'string' ? event.data : String(event.data);
        const data = JSON.parse(raw) as CDCMessage;
        const targetChannel = data.channel || data.topic;
        if (data && data.event && targetChannel) {
          const ch = this.channels.get(targetChannel);
          if (ch) {
            ch._dispatch(data);
          }
        }
      } catch {
        // ignore non-json
      }
    };

    this.ws.onclose = () => {
      this.stopHeartbeat();
      this.ws = undefined;
      if (!this.isExplicitClose) {
        this.reconnectTimer = setTimeout(() => this.ensureConnected(), 1000);
      }
    };
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.pingInterval = setInterval(() => {
      this.ping();
    }, 30000);
  }

  private stopHeartbeat(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = undefined;
    }
  }

  close(): void {
    this.isExplicitClose = true;
    this.stopHeartbeat();
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.ws) {
      this.ws.close();
      this.ws = undefined;
    }
    this.channels.clear();
  }
}

export function useRealtime(client: LayrClient): RealtimeManager {
  let rtManager = client._singletons.get('realtimeManager') as RealtimeManager | undefined;
  if (!rtManager) {
    rtManager = new RealtimeManager(client);
    client._singletons.set('realtimeManager', rtManager);
  }
  return rtManager;
}
