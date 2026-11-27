import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import Configuration from "../../pages/data/configurations.vue";
import {
  action,
  field,
  flushPromises,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
describe("Data configuration", () => {
  it("validates and saves service settings while preserving unchanged limits", async () => {
    const api = useNuxtApp().$client.value.data.cp;
    const original = await api.config.get();
    ({ wrapper } = await mountPage(Configuration));
    const state = wrapper.setupState;
    await updateDataModels(wrapper, true);
    try {
      state.schemas.value = [];
      expect(state.validate()).toHaveLength(1);
      state.schemas.value = ["invalid.name"];
      expect(state.validate()).toHaveLength(1);
      state.schemas.value = [...original.schemas!];
      state.excludedTables.value = ["a.b.c", "invalid-name"];
      expect(state.validate()).toHaveLength(1);
      state.excludedTables.value = [...original.rest.excludedTables!];
      state.config.value.rest.defaultLimit = 0;
      expect(state.validate()).toHaveLength(1);
      state.config.value.rest.defaultLimit = original.rest.maxLimit + 1;
      expect(state.validate()).toHaveLength(1);
      state.config.value.rest.defaultLimit = 24;
      for (const invalid of [-1, 1.5, undefined]) {
        state.config.value.cache.queryTtlSeconds = invalid;
        expect(state.validate()).toHaveLength(1);
      }
      state.config.value.cache.queryTtlSeconds = 0;
      expect(state.validate()).toEqual([]);
      state.config.value.rateLimiting.enabled = true;
      state.config.value.rateLimiting.burst = 0;
      expect(state.validate()).toHaveLength(1);
      state.config.value.rateLimiting = { ...original.rateLimiting };
      await nextTick();
      await field("Default Row Limit").trigger("keydown", { key: "ArrowUp" });
      expect(state.config.value.rest.defaultLimit).toBe(25);
      expect(pageDOM().text()).toContain("Unsaved Changes");
      await action("Reload").trigger("click");
      await action("Keep Editing").trigger("click");
      expect(state.config.value.rest.defaultLimit).toBe(25);
      await wrapper.get("#data-config").trigger("submit");
      await vi.waitFor(() =>
        expect(state.dataControlPlane.notice.value).toBe("Data Configuration Saved"),
      );
      expect((await api.config.get()).rest.defaultLimit).toBe(25);
      expect(pageDOM().text()).toContain("Data Configuration Saved");
      expect((await api.config.get()).rateLimiting).toEqual(original.rateLimiting);
      await action("Reload").trigger("click");
      await vi.waitFor(() => expect(state.dataControlPlane.busy.value).toBe(false));
      state.config.value = null;
      await nextTick();
      expect(pageDOM().text()).toContain("Configuration Unavailable");
      expect(state.validate()).toEqual([]);
      await state.save();
      vi.spyOn(state.dataControlPlane, "loadConfig").mockResolvedValueOnce(false);
      await state.load();
      await state.load();
      await signInConsoleUser(["data:config.read"]);
      await nextTick();
      expect(pageDOM().text()).not.toContain("Save Data Config");
      await signInConsoleUser([]);
      await state.load();
    } finally {
      await signInConsoleUser();
      await api.config.update(original);
    }
  });
  it("normalizes nullable configuration arrays", async () => {
    ({ wrapper } = await mountPage(Configuration));
    const state = wrapper.setupState;
    await updateDataModels(wrapper, true);
    const api = useNuxtApp().$client.value.data.cp;
    const original = await api.config.get();
    try {
      vi.spyOn(api.config, "get").mockResolvedValueOnce({
        ...original,
        schemas: null,
        rest: { ...original.rest, excludedTables: null },
        cors: { ...original.cors, allowedOrigins: null, allowedHeaders: null },
      });
      await state.load();
      expect(state.schemas.value).toEqual([]);
      expect(state.excludedTables.value).toEqual([]);
      expect(state.origins.value).toEqual([]);
      expect(state.headers.value).toEqual([]);
    } finally {
      await api.config.update(original);
    }
    await signInConsoleUser(["data:schema.read"]);
    await state.dataControlPlane.loadConfig();
    expect(state.dataControlPlane.error.value).toBeTruthy();
    await flushPromises();
  });
});
