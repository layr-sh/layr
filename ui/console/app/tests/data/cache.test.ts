import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import Cache from "../../pages/data/cache.vue";
import {
  action,
  activeDialog,
  field,
  mountPage,
  pageDOM,
  selectOption,
  signInConsoleUser,
  flushPromises,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(async () => {
  wrapper?.unmount();
  await useNuxtApp().$router.push("/");
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.dataControlPlane.busy.value).toBe(false));
}
describe("Data cache", () => {
  it("invalidates a selected table and confirms global cache flushes", async () => {
    const { $router: router, $client: client } = useNuxtApp();
    const firstTable = (await client.value.data.cp.tables.list()).data![0]!;
    await router.push({
      path: "/data/cache",
      query: { table: `${firstTable.schema}.${firstTable.name}` },
    });
    ({ wrapper } = await mountPage(Cache));
    const state = wrapper.setupState;
    expect(state.selected.value).toBe(`${firstTable.schema}.${firstTable.name}`);
    await updateDataModels(wrapper, true);
    state.selected.value = "";
    await state.invalidate();
    const first = state.dataControlPlane.tables.value[0];
    await nextTick();
    await selectOption("Table", `${first.schema}.${first.name}`);
    expect(state.selected.value).toBe(`${first.schema}.${first.name}`);
    await action("Invalidate Table Cache").trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Cache Invalidation Request Accepted");
    state.scope.value = "schema";
    await state.invalidate();
    state.schema.value = first.schema;
    await nextTick();
    await updateDataModels(wrapper);
    expect(pageDOM().text()).toContain("Global Query Cache Impact");
    await action("Invalidate Schema Cache").trigger("click");
    await action("Cancel", activeDialog()!).trigger("click");
    await action("Invalidate Schema Cache").trigger("click");
    await action("Confirm Invalidation", activeDialog()!).trigger("click");
    await settle();
    expect(state.schemaOpen.value).toBe(false);
    await action("Flush All Caches").trigger("click");
    expect(action("Confirm Flush", activeDialog()!).attributes("disabled")).toBeDefined();
    await action("Cancel", activeDialog()!).trigger("click");
    await action("Flush All Caches").trigger("click");
    await field("Type FLUSH to confirm").setValue("FLUSH");
    await action("Confirm Flush", activeDialog()!).trigger("click");
    await settle();
    expect(state.flushOpen.value).toBe(false);
    expect(pageDOM().text()).toContain("Cache Flush Request Accepted");
    await action("Flush All Caches").trigger("click");
    await signInConsoleUser(["data:schema.read"]);
    await state.flush();
    expect(state.flushOpen.value).toBe(true);
    expect(pageDOM().text()).toContain("Request Failed");
  });
  it("allows global cache operations without exposing schema metadata", async () => {
    await signInConsoleUser(["data:cache.write"]);
    ({ wrapper } = await mountPage(Cache));
    expect(pageDOM().text()).toContain("Schema Access Required");
    expect(wrapper.setupState.options.value).toEqual([]);
    expect(wrapper.setupState.dataControlPlane.tables.value).toEqual([]);
  });
});
