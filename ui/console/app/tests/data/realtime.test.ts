import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, type Component } from "vue";
import { USelectMenu } from "#components";
import { useNuxtApp } from "#app";
import Realtime from "../../pages/data/realtime.vue";
import {
  action,
  activeDialog,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
class InspectorWebSocket {
  static OPEN = 1;
  static instances: InspectorWebSocket[] = [];
  static failure: unknown;
  readyState = 0;
  sent: string[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  url: string;
  constructor(url: string) {
    if (InspectorWebSocket.failure) throw InspectorWebSocket.failure;
    this.url = url;
    InspectorWebSocket.instances.push(this);
  }
  send(value: string) {
    this.sent.push(value);
  }
  close() {
    this.readyState = 3;
  }
}
beforeEach(async () => {
  await signInConsoleUser();
  InspectorWebSocket.instances = [];
  InspectorWebSocket.failure = undefined;
});
afterEach(async () => {
  wrapper?.unmount();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  await useNuxtApp().$router.push("/");
});
function firstTable() {
  const setup = wrapper.setupState;
  const table = setup.dataControlPlane.tables.value[0];
  setup.state.table = `${table.schema}.${table.name}`;
  return setup;
}
function message(connection: InspectorWebSocket, value: unknown) {
  connection.onmessage?.(
    new MessageEvent("message", {
      data: typeof value === "string" ? value : JSON.stringify(value),
    }),
  );
}
describe("Data realtime inspector", () => {
  it("connects to the actual backend and waits for subscription acknowledgement", async () => {
    const { $router: router, $client: client } = useNuxtApp();
    const first = (await client.value.data.cp.tables.list()).data![0]!;
    await router.push({
      path: "/data/realtime",
      query: { table: `${first.schema}.${first.name}` },
    });
    ({ wrapper } = await mountPage(Realtime));
    const setup = wrapper.setupState;
    expect(setup.state.table).toBe(`${first.schema}.${first.name}`);
    const next = setup.options.value.find(
      (option: { value: string }) => option.value !== setup.state.table,
    )!;
    wrapper
      .findAllComponents(USelectMenu as Component)[0]!
      .vm.$emit("update:modelValue", next.value);
    await nextTick();
    expect(setup.state.table).toBe(next.value);
    setup.connect();
    await vi.waitFor(() => expect(setup.status.value).toBe("Listening"), { timeout: 11000 });
    await action("Disconnect").trigger("click");
    expect(setup.status.value).toBe("Disconnected");
  });
  it("handles the actual realtime protocol, bounded logs, pauses, and cleanup", async () => {
    vi.stubGlobal("WebSocket", InspectorWebSocket);
    ({ wrapper } = await mountPage(Realtime));
    const setup = firstTable();
    expect(setup.validate()).toEqual([]);
    vi.useFakeTimers();
    setup.connect();
    const connection = InspectorWebSocket.instances.at(-1)!;
    expect(connection.url).toContain("/v1/realtime?token=");
    connection.readyState = 1;
    connection.onopen?.(new Event("open"));
    expect(JSON.parse(connection.sent[0]!)).toMatchObject({
      action: "subscribe",
      channel: "console-data-inspector",
    });
    message(connection, { event: "INSERT" });
    expect(setup.messages.value).toEqual([]);
    message(connection, { status: "subscribed" });
    message(connection, { status: "subscribed" });
    expect(setup.status.value).toBe("Listening");
    message(connection, { status: "pong" });
    await vi.advanceTimersByTimeAsync(15000);
    expect(connection.sent.at(-1)).toContain("ping");
    connection.readyState = 0;
    await vi.advanceTimersByTimeAsync(15000);
    connection.readyState = 1;
    message(connection, { event: "INSERT", schema: "public", table: "items", record: { id: 1 } });
    for (let index = 0; index < 101; index++)
      message(connection, { event: "UPDATE", record: { index } });
    expect(setup.messages.value).toHaveLength(100);
    await nextTick();
    await action("Pause Display").trigger("click");
    message(connection, { event: "DELETE" });
    expect(setup.messages.value[0].event).toBe("UPDATE");
    expect(setup.skipped.value).toBe(1);
    await action("Resume Display").trigger("click");
    const eventRow = pageDOM().get("tbody tr");
    await eventRow.trigger("click");
    expect(activeDialog()?.textContent).toContain("UPDATE");
    expect(activeDialog()?.textContent).toContain("Previous Values Unavailable");
    setup.selected.value = {
      event: "UPDATE",
      truncated: true,
      old_record: { id: 1, name: "old" },
      record: { id: 1, name: "new" },
    };
    await nextTick();
    expect(activeDialog()?.textContent).toContain("Partial row payload");
    await action("Copy Event JSON", activeDialog()!).trigger("click");
    expect(setup.changes.value).toEqual([{ name: "name", before: '"old"', after: '"new"' }]);
    setup.setEventOpen(true);
    await action("Close", activeDialog()!).trigger("click");
    await menuAction("Inspect", eventRow.element);
    await action("Close", activeDialog()!).trigger("click");
    await action("Clear Events").trigger("click");
    expect(setup.messages.value).toEqual([]);
    await action("Pause Display").trigger("click");
    expect(pageDOM().text()).toContain("Display Paused");
    await action("Resume Display").trigger("click");
    message(connection, "invalid json");
    expect(setup.error.value).toContain("invalid realtime message");
    message(connection, { status: "error", error: "Subscription denied" });
    expect(setup.status.value).toBe("Disconnected");
    expect(setup.error.value).toBe("Subscription denied");
    setup.connect();
    const second = InspectorWebSocket.instances.at(-1)!;
    second.onopen?.(new Event("open"));
    message(second, { type: "subscribed" });
    second.onclose?.(new CloseEvent("close", { code: 1008, reason: "Policy changed" }));
    setup.connect();
    InspectorWebSocket.instances.at(-1)!.onclose?.(new CloseEvent("close", { code: 1006 }));
    expect(setup.error.value).toContain("closed (code 1006)");
    setup.connect();
    InspectorWebSocket.instances.at(-1)!.onerror?.(new Event("error"));
    expect(setup.error.value).toContain("connection failed");
    setup.connect();
    await vi.advanceTimersByTimeAsync(10000);
    expect(setup.error.value).toContain("not acknowledged");
    const client = useNuxtApp().$client.value;
    const endpoint = client.endpoint;
    const token = client.sessionToken!;
    Object.defineProperty(client, "endpoint", {
      value: "https://realtime.example.test",
      configurable: true,
    });
    client.clearSession();
    setup.connect();
    expect(InspectorWebSocket.instances.at(-1)!.url).toBe(
      "wss://realtime.example.test/v1/realtime",
    );
    message(InspectorWebSocket.instances.at(-1)!, { error: "Denied" });
    Object.defineProperty(client, "endpoint", { value: endpoint, configurable: true });
    client.setSession(token);
    InspectorWebSocket.failure = new Error("Unavailable");
    setup.connect();
    expect(setup.error.value).toBe("Unavailable");
    InspectorWebSocket.failure = "unavailable";
    setup.connect();
    expect(setup.error.value).toContain("Could not connect");
  });
  it("does not connect without a selected table or query access", async () => {
    vi.stubGlobal("WebSocket", InspectorWebSocket);
    ({ wrapper } = await mountPage(Realtime));
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.validate()).toHaveLength(1);
    setup.connect();
    expect(InspectorWebSocket.instances).toHaveLength(0);
    firstTable();
    await signInConsoleUser(["data:schema.read"]);
    setup.connect();
    expect(InspectorWebSocket.instances).toHaveLength(0);
    wrapper.unmount();
    await signInConsoleUser(["data:query.read"]);
    ({ wrapper } = await mountPage(Realtime));
    wrapper.setupState.error.value = "";
    wrapper.setupState.dataControlPlane.error.value = "Metadata unavailable";
    await nextTick();
    expect(pageDOM().text()).toContain("Metadata unavailable");
    expect(wrapper.setupState.options.value).toEqual([]);
    expect(pageDOM().text()).toContain("Realtime Inspector");
  });
});
