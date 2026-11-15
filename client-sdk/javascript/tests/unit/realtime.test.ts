import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createClient } from "../../src/core/client";
import { RealtimeManager, useRealtime, type CDCMessage } from "../../src/extensions/realtime";

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  readyState = 0;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: ((err: unknown) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
    setTimeout(() => {
      this.readyState = 1;
      this.onopen?.();
    }, 0);
  }

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = 3;
    this.onclose?.();
  }

  simulateMessage(data: object) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }
}

describe("Realtime WebSocket Unit Tests (RT-01 to RT-08)", () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("RT-01 & RT-02: WebSocket URL Derivation for HTTP and HTTPS", () => {
    const httpClient = createClient({ endpoint: "http://localhost:5297" });
    const httpManager = new RealtimeManager(httpClient);
    httpManager.subscribe("todos");
    expect(MockWebSocket.instances[0].url).toBe("ws://localhost:5297/v1/realtime");
    httpManager.close();

    const httpsClient = createClient({ endpoint: "https://api.layr.sh" });
    const httpsManager = new RealtimeManager(httpsClient);
    httpsManager.subscribe("todos");
    expect(MockWebSocket.instances[1].url).toBe("wss://api.layr.sh/v1/realtime");
    httpsManager.close();
  });

  it("RT-03: Token parameter injected into WebSocket URL when session exists", () => {
    const client = createClient({
      endpoint: "https://api.layr.sh",
      sessionToken: "Bearer test_jwt_token",
    });
    const manager = new RealtimeManager(client);
    manager.subscribe("todos");
    expect(MockWebSocket.instances[0].url).toBe(
      "wss://api.layr.sh/v1/realtime?token=test_jwt_token",
    );
    manager.close();
  });

  it("RT-04: Channel Subscription sends subscribe frame on open", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    manager.subscribe("todos");

    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[0];
    expect(ws.sent.length).toBeGreaterThan(0);
    const frame = JSON.parse(ws.sent[0]);
    expect(frame.action).toBe("subscribe");
    expect(frame.table).toBe("todos");
    expect(frame.schema).toBe("public");
    manager.close();
  });

  it("RT-05: Event Routing dispatches INSERT, UPDATE, DELETE and wildcard events", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    const channel = manager.subscribe("todos");

    const received: string[] = [];
    channel
      .onInsert((payload) => received.push(`insert:${payload.record.title}`))
      .onUpdate((payload) => received.push(`update:${payload.record.title}`))
      .onDelete((payload) => received.push(`delete:${payload.record.title}`))
      .onAll((payload) => received.push(`all:${payload.event}`));

    channel.on("*", (payload) => received.push(`star:${payload.event}`));

    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[0];

    const insertMsg: CDCMessage<{ title: string }> = {
      channel: "public:todos",
      event: "INSERT",
      schema: "public",
      table: "todos",
      commit_timestamp: "2026-10-13T00:00:00Z",
      record: { title: "First Task" },
    };
    ws.simulateMessage(insertMsg);

    const updateMsg: CDCMessage<{ title: string }> = {
      channel: "public:todos",
      event: "UPDATE",
      schema: "public",
      table: "todos",
      commit_timestamp: "2026-10-13T00:00:00Z",
      record: { title: "Updated Task" },
    };
    ws.simulateMessage(updateMsg);

    const deleteMsg: CDCMessage<{ title: string }> = {
      channel: "public:todos",
      event: "DELETE",
      schema: "public",
      table: "todos",
      commit_timestamp: "2026-10-13T00:00:00Z",
      record: { title: "Deleted Task" },
    };
    ws.simulateMessage(deleteMsg);

    expect(received).toContain("insert:First Task");
    expect(received).toContain("all:INSERT");
    expect(received).toContain("star:INSERT");
    expect(received).toContain("update:Updated Task");
    expect(received).toContain("delete:Deleted Task");
    manager.close();
  });

  it("RT-06: Heartbeat Ping sends ping frame periodically", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    manager.subscribe("todos");

    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[0];
    const initialSent = ws.sent.length;

    await vi.advanceTimersByTimeAsync(30000);
    expect(ws.sent.length).toBeGreaterThan(initialSent);
    const pingFrame = JSON.parse(ws.sent[ws.sent.length - 1]);
    expect(pingFrame.action).toBe("ping");
    manager.close();
  });

  it("RT-07: Automatic Reconnection attempts reconnect after abrupt close", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    manager.subscribe("todos");

    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances.length).toBe(1);

    MockWebSocket.instances[0].onclose?.();

    await vi.advanceTimersByTimeAsync(1000);
    expect(MockWebSocket.instances.length).toBe(2);
    manager.close();
  });

  it("RT-08: Clean Teardown cancels heartbeat and ignores reconnect", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    const channel = manager.channel("app.items");
    channel.subscribe();
    expect(channel.schema).toBe("app");
    expect(channel.table).toBe("items");

    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances.length).toBe(1);

    channel.unsubscribe();
    manager.close();

    await vi.advanceTimersByTimeAsync(5000);
    expect(MockWebSocket.instances.length).toBe(1);
  });

  it("RT-09: Subscribing and unsubscribing when socket is already open", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    manager.subscribe("table1");

    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    expect(ws.readyState).toBe(1);

    // Subscribe when already open:
    manager.subscribe("table2");
    expect(ws.sent.some((s) => s.includes("table2"))).toBe(true);

    // Unsubscribe when already open:
    manager.unsubscribe("public:table1");
    expect(ws.sent.some((s) => s.includes("unsubscribe") && s.includes("public:table1"))).toBe(
      true,
    );

    // Ping explicitly
    manager.ping();
    expect(ws.sent.some((s) => s.includes("ping"))).toBe(true);

    manager.close();
  });

  it("RT-10: Handles missing or throwing WebSocket class gracefully", () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const origWs = globalThis.WebSocket;

    // Missing WebSocket
    // @ts-expect-error delete for test
    delete globalThis.WebSocket;
    const manager1 = new RealtimeManager(client);
    expect(() => manager1.ensureConnected()).not.toThrow();

    // Throwing WebSocket constructor
    // @ts-expect-error mock throwing constructor
    globalThis.WebSocket = class {
      constructor() {
        throw new Error("WS connection blocked");
      }
    };
    const manager2 = new RealtimeManager(client);
    expect(() => manager2.ensureConnected()).not.toThrow();

    // Restore
    globalThis.WebSocket = origWs;
  });

  it("RT-11: useRealtime singleton, topic fallback, non-string message, and active close", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });

    // 1. Singleton pattern
    const rt1 = useRealtime(client);
    const rt2 = useRealtime(client);
    expect(rt1).toBe(rt2);

    // 2. Subscribe channel
    const channel = rt1.subscribe("todos");
    let receivedInsert = false;
    channel.onInsert(() => {
      receivedInsert = true;
    });

    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];

    // 3. Dispatch with topic fallback and non-string data
    const nonStringEvent = {
      data: {
        toString: () =>
          JSON.stringify({ topic: "public:todos", event: "INSERT", record: { id: 10 } }),
      },
    };
    // @ts-expect-error non-string data test
    ws.onmessage?.(nonStringEvent);
    expect(receivedInsert).toBe(true);

    // 4. Dispatch to an unknown channel
    const unknownChannelEvent = {
      data: JSON.stringify({ channel: "public:unknown_table", event: "INSERT", record: {} }),
    };
    expect(() => ws.onmessage?.(unknownChannelEvent)).not.toThrow();

    // 5. Close when WebSocket is open
    expect(ws.readyState).toBe(1);
    rt1.close();
    expect(ws.readyState).toBe(3);
  });

  it("RT-12: RealtimeManager disconnected state handling and malformed message shapes", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);

    // 1. Ping and unsubscribe while disconnected
    expect(() => manager.ping()).not.toThrow();
    manager.subscribe("test");
    expect(() => manager.unsubscribe("public:test")).not.toThrow();

    // 2. Close while disconnected
    expect(() => manager.close()).not.toThrow();

    // 3. Connect and send message missing event or targetChannel
    manager.subscribe("items");
    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];

    expect(() => ws.onmessage?.({ data: JSON.stringify({ no_event: true }) })).not.toThrow();
    expect(() => ws.onmessage?.({ data: JSON.stringify({ event: "INSERT" }) })).not.toThrow();

    manager.close();
  });

  it("RT-13: channel.on router, unknown events, nonexistent unsubscribe, and reconnectTimer clearing", async () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const manager = new RealtimeManager(client);
    const channel = manager.subscribe("items");

    const received: string[] = [];
    channel
      .on("INSERT", (m) => received.push(`on-insert:${m.event}`))
      .on("UPDATE", (m) => received.push(`on-update:${m.event}`))
      .on("DELETE", (m) => received.push(`on-delete:${m.event}`))
      .on("*", (m) => received.push(`on-all:${m.event}`));

    // Test channel.on dispatch
    channel._dispatch({
      event: "INSERT",
      schema: "public",
      table: "items",
      record: {},
      commit_timestamp: "",
    });
    channel._dispatch({
      event: "UPDATE",
      schema: "public",
      table: "items",
      record: {},
      commit_timestamp: "",
    });
    channel._dispatch({
      event: "DELETE",
      schema: "public",
      table: "items",
      record: {},
      commit_timestamp: "",
    });
    channel._dispatch({
      event: "UNKNOWN" as any,
      schema: "public",
      table: "items",
      record: {},
      commit_timestamp: "",
    });

    expect(received).toContain("on-insert:INSERT");
    expect(received).toContain("on-update:UPDATE");
    expect(received).toContain("on-delete:DELETE");
    expect(received).toContain("on-all:UNKNOWN");

    // Unsubscribe nonexistent channel
    expect(() => manager.unsubscribe("nonexistent:channel")).not.toThrow();

    // Trigger unexpected close to start reconnectTimer
    await vi.advanceTimersByTimeAsync(10);
    const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1];
    ws.onclose?.();

    // Verify close() clears reconnectTimer
    expect(() => manager.close()).not.toThrow();
  });
});
