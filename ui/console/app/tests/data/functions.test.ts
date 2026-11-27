import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, type Component } from "vue";
import { UForm, USelectMenu } from "#components";
import Functions from "../../pages/data/functions.vue";
import {
  action,
  activeDialog,
  field,
  mountPage,
  pageDOM,
  signInConsoleUser,
  flushPromises,
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
describe("Database functions UI", () => {
  it("discovers actual PostgreSQL functions and confirms named RPC invocation", async () => {
    ({ wrapper } = await mountPage(Functions));
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    const name = `ui_function_${crypto.randomUUID().replaceAll("-", "")}`;
    expect(
      await setup.dataControlPlane.executeSQL(
        `CREATE FUNCTION public.${name}(value text) RETURNS text LANGUAGE sql IMMUTABLE AS 'SELECT value';`,
      ),
    ).toBe(true);
    try {
      await action("Refresh").trigger("click");
      await settle();
      await vi.waitFor(() =>
        expect(setup.functions.value.some((item: { name: string }) => item.name === name)).toBe(
          true,
        ),
      );
      setup.selectedId.value = setup.functions.value.find(
        (item: { name: string }) => item.name === name,
      ).id;
      await nextTick();
      expect(pageDOM().text()).toContain("Immutable");
      await updateDataModels(wrapper, true);
      expect(setup.supportsFields.value).toBe(true);
      expect(setup.validate()).toHaveLength(1);
      setup.fields.value.value = "hello";
      wrapper.findComponent(UForm).vm.$emit("submit", {});
      await nextTick();
      expect(setup.confirmOpen.value).toBe(true);
      await action("Cancel", activeDialog()!).trigger("click");
      setup.prepareInvocation();
      await nextTick();
      await action("Invoke", activeDialog()!).trigger("click");
      await settle();
      expect(setup.response.value).toBeDefined();
      expect(pageDOM().text()).toContain("hello");
      setup.mode.value = "json";
      for (const raw of ["null", "[]", "invalid"]) {
        setup.invocation.arguments = raw;
        expect(setup.validate()).toHaveLength(1);
      }
      setup.invocation.arguments = '{"wrong_argument":true}';
      expect(setup.validate()).toEqual([]);
      setup.prepareInvocation();
      await setup.invoke();
      await nextTick();
      expect(pageDOM().text()).toContain("Request Failed");
      setup.selected.value.volatility = "s";
      setup.selected.value.security_definer = true;
      await nextTick();
      expect(pageDOM().text()).toContain("Security Definer");
      expect(pageDOM().text()).toContain("Stable");
      setup.selected.value.volatility = "v";
      await nextTick();
      expect(pageDOM().text()).toContain("Volatile");
      await field("Function").trigger("click");
      await flushPromises();
      const picker = wrapper.findAllComponents(USelectMenu as Component)[0]!;
      picker.vm.$emit("update:searchTerm", "no_matching_console_function");
      await nextTick();
      expect(setup.options.value).toEqual([]);
      expect(pageDOM().text()).toContain("No matching functions");
    } finally {
      await setup.dataControlPlane.executeSQL(`DROP FUNCTION public.${name}(text);`);
    }
    await signInConsoleUser(["data:schema.read"]);
    await setup.load();
  });
  it("shows overloads and lets optional arguments use their defaults", async () => {
    ({ wrapper } = await mountPage(Functions));
    const setup = wrapper.setupState;
    const name = `ui_contract_${crypto.randomUUID().replaceAll("-", "")}`;
    expect(
      await setup.dataControlPlane.executeSQL(`
      CREATE FUNCTION public.${name}(enabled boolean, settings jsonb, started date DEFAULT CURRENT_DATE)
      RETURNS jsonb LANGUAGE sql AS 'SELECT jsonb_build_object(''enabled'', enabled, ''settings'', settings, ''started'', started)';
      CREATE FUNCTION public.${name}(message text) RETURNS text LANGUAGE sql AS 'SELECT message';
      CREATE FUNCTION public.${name}() RETURNS integer LANGUAGE sql AS 'SELECT 1';
    `),
    ).toBe(true);
    try {
      await setup.load();
      const contract = setup.functions.value.find(
        (item: { name: string; arguments: string }) =>
          item.name === name && item.arguments.includes("boolean"),
      );
      setup.selectedId.value = contract.id;
      await nextTick();
      expect(pageDOM().text()).toContain("Overloaded Function");
      setup.fields.value = { enabled: "true", settings: '{"limit":5}', started: "2026-01-01" };
      await nextTick();
      await updateDataModels(wrapper, true);
      await action("Use default").trigger("click");
      expect(setup.argumentPayload()).toEqual({ enabled: true, settings: { limit: 5 } });
      setup.prepareInvocation();
      await setup.invoke();
      expect(setup.response.value).toMatchObject({ enabled: true, settings: { limit: 5 } });
      await action("Copy response").trigger("click");
      setup.mode.value = "json";
      await nextTick();
      await updateDataModels(wrapper);
      setup.selectedId.value = setup.functions.value.find(
        (item: { name: string; arguments: string }) => item.name === name && !item.arguments,
      ).id;
      await nextTick();
      expect(setup.supportsFields.value).toBe(false);
      expect(setup.argumentPayload()).toEqual({});
      expect(pageDOM().text()).not.toContain("Argument fields");
    } finally {
      await setup.dataControlPlane.executeSQL(
        `DROP FUNCTION public.${name}(boolean,jsonb,date); DROP FUNCTION public.${name}(text); DROP FUNCTION public.${name}();`,
      );
    }
  });
  it("handles empty SQL contracts and failed metadata refreshes", async () => {
    ({ wrapper } = await mountPage(Functions));
    const setup = wrapper.setupState;
    vi.spyOn(setup.dataControlPlane, "loadTables").mockResolvedValue(true);
    vi.spyOn(setup.dataControlPlane, "loadConfig").mockResolvedValue(true);
    setup.dataControlPlane.tables.value = [];
    setup.dataControlPlane.config.value.schemas = [];
    setup.dataControlPlane.result.value = null;
    vi.spyOn(setup.dataControlPlane, "executeSQL").mockResolvedValueOnce(true);
    await setup.load();
    expect(setup.functions.value).toEqual([]);
    await nextTick();
    expect(pageDOM().text()).toContain("No Functions Found");
    vi.spyOn(setup.dataControlPlane, "executeSQL").mockResolvedValueOnce(false);
    await setup.load();
    expect(setup.functions.value).toEqual([]);
    vi.restoreAllMocks();
  });
  it("supports schema writers without configuration access", async () => {
    await signInConsoleUser(["data:schema.write"]);
    ({ wrapper } = await mountPage(Functions));
    await settle();
    expect(wrapper.setupState.dataControlPlane.error.value).toBe("");
  });
});
