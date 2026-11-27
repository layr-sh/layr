import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import KeyValue from "../../pages/data/key-value.vue";
import {
  action,
  activeDialog,
  field,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  selectOption,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.dataControlPlane.busy.value).toBe(false));
}
describe("Data key–value UI", () => {
  it("validates key names, byte lengths, TTLs, and integer steps", async () => {
    ({ wrapper } = await mountPage(KeyValue));
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.validate()).toHaveLength(1);
    setup.state.key = "key";
    setup.state.ttl = -1;
    setup.state.step = 1.1;
    expect(setup.validate()).toHaveLength(2);
    setup.state.key = "mget";
    expect(setup.validate()).toHaveLength(3);
    setup.state.key = "é".repeat(257);
    expect(setup.validate()).toHaveLength(3);
    await setup.save();
    await setup.touch();
    await setup.increment();
    setup.state.key = "key";
    setup.state.ttl = 0;
    setup.state.step = -1;
    expect(setup.validate()).toEqual([]);
    await setup.touch();
  });
  it("reads, saves, expires, increments, and confirms key deletion against the backend", async () => {
    ({ wrapper } = await mountPage(KeyValue));
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    const key = `console:${crypto.randomUUID()}`;
    await field("Key").setValue(key);
    await field("Value").setValue("10");
    await setup.save();
    await settle();
    expect(pageDOM().text()).toContain("Key Saved");
    expect(pageDOM().text()).toContain("No expiration requested");
    setup.valueFormat.value = "json";
    await nextTick();
    expect(setup.preview.value).toBe("10");
    await action("Copy Value").trigger("click");
    setup.state.value = "not JSON";
    await nextTick();
    expect(pageDOM().text()).toContain("Invalid JSON");
    setup.valueFormat.value = "text";
    expect(setup.preview.value).toBe("");
    expect(setup.recentKeys.value).toEqual([key]);
    await action("Read Key").trigger("click");
    await settle();
    expect(setup.entry.value.value).toBe("10");
    await field("Key").setValue("");
    await selectOption("Recent Keys", key);
    expect(setup.state.key).toBe(key);
    await settle();
    setup.state.ttl = 60;
    await nextTick();
    await menuAction("Update Expiration", wrapper.element);
    await settle();
    expect(pageDOM().text()).toContain("Expiration Updated");
    expect(pageDOM().text()).toContain("Expiration set for");
    setup.state.step = -2;
    await menuAction("Apply Counter Step", wrapper.element);
    await settle();
    expect(setup.state.value).toBe("8");
    await menuAction("Delete Key", wrapper.element);
    await action("Cancel", activeDialog()!).trigger("click");
    await menuAction("Delete Key", wrapper.element);
    await field(`Type ${key} to confirm`).setValue(key);
    await action("Confirm Delete", activeDialog()!).trigger("click");
    await settle();
    expect(setup.entry.value).toBeNull();
    expect(setup.deleteOpen.value).toBe(false);
    expect(setup.recentKeys.value).toEqual([]);
    await setup.lookup(key);
    expect(pageDOM().text()).toContain("Key Not Found");
    const get = vi
      .spyOn(useNuxtApp().$client.value.data.kv, "get")
      .mockRejectedValueOnce(new Error("Connection interrupted"));
    await setup.lookup(key);
    expect(pageDOM().text()).toContain("Connection interrupted");
    get.mockRestore();
    setup.pendingKey.value = "mget";
    setup.deleteOpen.value = true;
    await setup.remove();
    expect(setup.deleteOpen.value).toBe(true);
    await action("Cancel", activeDialog()!).trigger("click");
    setup.remember("one");
    setup.remember("two");
    setup.remember("one");
    expect(setup.recentKeys.value).toEqual(["one", "two"]);
    await signInConsoleUser(["data:query.read"]);
    await nextTick();
    expect(action("Save Key").attributes("disabled")).toBeDefined();
  });
});
