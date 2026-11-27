import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";
import { useNuxtApp } from "#app";
import { LayrAPIError } from "layr-client";
import useDataControlPlane from "../../composables/use-data-control-plane";
import { mountPage, signInConsoleUser } from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});
const Harness = defineComponent({
  setup() {
    const dataControlPlane = useDataControlPlane(useNuxtApp().$client.value.data.cp);
    return { dataControlPlane };
  },
  render: () => h("div"),
});
describe("Data control plane", () => {
  it("loads and mutates real metadata, configuration, cache, and SQL", async () => {
    ({ wrapper } = await mountPage(Harness));
    const dataControlPlane = wrapper.setupState.dataControlPlane as ReturnType<
      typeof useDataControlPlane
    >;
    const api = useNuxtApp().$client.value.data.cp;
    const name = `console_${crypto.randomUUID().replaceAll("-", "")}`;
    expect(await dataControlPlane.loadTables()).toBe(true);
    expect(
      await dataControlPlane.executeSQL("select 42 as answer"),
      dataControlPlane.error.value,
    ).toBe(true);
    expect(dataControlPlane.result.value?.rows?.[0]?.answer).toBe(42);
    expect(
      await dataControlPlane.mutate(
        () => api.tables.create({ schema: "public", name, columns: [] }),
        "Created",
        "public",
        name,
      ),
    ).toBe(true);
    expect(dataControlPlane.selected.value?.name).toBe(name);
    expect(await dataControlPlane.selectTable("public", name)).toBe(true);
    expect(await dataControlPlane.loadConfig()).toBe(true);
    expect(await dataControlPlane.saveConfig(dataControlPlane.config.value!)).toBe(true);
    expect(await dataControlPlane.invalidateCache({ schema: "public", table: name })).toBe(true);
    expect(await dataControlPlane.flushCache()).toBe(true);
    expect(
      await dataControlPlane.mutate(
        () => api.tables.delete("public", name),
        "Deleted",
        "public",
        name,
      ),
    ).toBe(true);
    expect(dataControlPlane.selected.value).toBeNull();
    expect(dataControlPlane.policies.value).toEqual([]);
    expect(await dataControlPlane.selectTable("public", name)).toBe(false);
    expect(dataControlPlane.error.value).toBeTruthy();
    expect(await dataControlPlane.executeSQL("select * from table_that_does_not_exist_123")).toBe(
      false,
    );
    expect(dataControlPlane.result.value).toBeNull();
  });
  it("normalizes nullable SDK list contracts and handles requests without a bearer token", async () => {
    ({ wrapper } = await mountPage(Harness));
    const dataControlPlane = wrapper.setupState.dataControlPlane as ReturnType<
      typeof useDataControlPlane
    >;
    const api = useNuxtApp().$client.value.data.cp;
    const table = (await api.tables.list()).data![0]!;
    vi.spyOn(api.tables, "list").mockResolvedValueOnce({ data: null, count: 0 });
    await dataControlPlane.loadTables();
    expect(dataControlPlane.tables.value).toEqual([]);
    vi.spyOn(api.tables.policies, "list").mockResolvedValueOnce({ data: null, count: 0 });
    await dataControlPlane.selectTable(table.schema, table.name);
    expect(dataControlPlane.policies.value).toEqual([]);
    vi.spyOn(api.tables.policies, "list").mockResolvedValueOnce({ data: null, count: 0 });
    await dataControlPlane.mutate(async () => {}, "Refreshed", table.schema, table.name);
    vi.spyOn(api.tables, "list").mockResolvedValueOnce({ data: null, count: 0 });
    await dataControlPlane.mutate(async () => {}, "Refreshed", table.schema, table.name);
    useNuxtApp().$client.value.clearSession();
    await dataControlPlane.executeSQL("select 1");
  });
  it("prevents overlapping operations and preserves errors", async () => {
    ({ wrapper } = await mountPage(Harness));
    const dataControlPlane = wrapper.setupState.dataControlPlane as ReturnType<
      typeof useDataControlPlane
    >;
    let resolve!: () => void;
    const pending = dataControlPlane.run(
      () =>
        new Promise<void>((done) => {
          resolve = done;
        }),
    );
    expect(
      await dataControlPlane.run(async () => {
        throw new Error("Should not run");
      }),
    ).toBe(false);
    resolve();
    await pending;
    expect(
      await dataControlPlane.run(async () => {
        throw "unavailable";
      }),
    ).toBe(false);
    expect(dataControlPlane.error.value).toBe("The request failed. Please try again.");
    expect(
      await dataControlPlane.run(async () => {
        throw new Error("Connection lost");
      }),
    ).toBe(false);
    expect(dataControlPlane.error.value).toBe("Connection lost");
    expect(
      await dataControlPlane.run(async () => {
        throw new LayrAPIError({ status: 500, detail: "Unavailable" });
      }),
    ).toBe(false);
    await signInConsoleUser(["data:schema.read"]);
    expect(await dataControlPlane.executeSQL("select 1")).toBe(false);
    expect(useNuxtApp().$auth.state.value).not.toBeNull();
    await useNuxtApp().$auth.signOut();
    expect(
      await dataControlPlane.run(async () => {
        throw new LayrAPIError({ status: 401, detail: "Expired" });
      }),
    ).toBe(false);
    await vi.waitFor(() => expect(useNuxtApp().$router.currentRoute.value.path).toBe("/sign-in"));
  });
});
