import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { USelectMenu } from "#components";
import type { Component } from "vue";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import type { DataControlPlaneModels } from "layr-client";
import RowForm from "../../components/data/RowForm.vue";
import { action, signInConsoleUser } from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountSuspended<typeof RowForm>>>;
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => {
  wrapper?.unmount();
  vi.restoreAllMocks();
});
const column = (name: string, type = "text", extra = {}) => ({
  name,
  type,
  isNullable: true,
  isPrimaryKey: false,
  isUnique: false,
  ...extra,
});
const table = {
  schema: "public",
  name: "sample",
  columns: [
    column("id", "uuid", { isPrimaryKey: true, defaultValue: "uuidv7()" }),
    column("title"),
    column("number", "bigint", { isNullable: false }),
    column("flag", "boolean"),
    column("details", "jsonb"),
    column("plainjson", "json"),
    column("array", "ARRAY"),
    column("values", "text[]"),
    column("date", "date"),
    column("time", "timestamp with time zone"),
  ],
  foreignKeys: [],
} as unknown as DataControlPlaneModels.Table;
const props = { table, tables: [], record: null, busy: false, error: "" };
describe("Data typed row forms", () => {
  it("distinguishes defaults, NULL, booleans, JSON, and exact numeric strings", async () => {
    wrapper = await mountSuspended(RowForm, { props, attachTo: document.body });
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.state.id.mode).toBe("omit");
    expect(setup.state.title.mode).toBe("null");
    setup.state.number.value = "9223372036854775807";
    setup.state.flag.mode = "value";
    setup.state.flag.boolean = true;
    setup.state.details.mode = "value";
    setup.state.details.value = '{"ok":true}';
    setup.state.plainjson.mode = "value";
    setup.state.plainjson.value = "{}";
    setup.state.array.mode = "value";
    setup.state.array.value = "[]";
    setup.state.values.mode = "value";
    setup.state.values.value = '["a"]';
    setup.state.date.mode = "value";
    setup.state.time.mode = "value";
    expect(setup.validate()).toEqual([]);
    setup.save();
    expect(wrapper.emitted("save")?.[0]?.[0]).toMatchObject({
      number: "9223372036854775807",
      flag: true,
      details: { ok: true },
      values: ["a"],
      title: null,
    });
    setup.state.details.value = "invalid";
    expect(setup.validate()).toHaveLength(1);
    setup.save();
    expect(setup.formError.value).toBeTruthy();
    await nextTick();
    await updateDataModels(wrapper);
    expect(wrapper.text()).toContain("Row Not Saved");
    setup.state.date.mode = "omit";
    await nextTick();
    expect(wrapper.text()).toContain("No default defined");
    setup.toggleAdvanced();
    expect(setup.advanced.value).toBe(true);
    expect(setup.json.value).toBe("{}");
    await nextTick();
    await updateDataModels(wrapper);
    setup.json.value = '{"title":"json"}';
    expect(setup.payload()).toEqual({ title: "json" });
    setup.save();
    setup.toggleAdvanced();
    setup.state.details.value = "{}";
    setup.toggleAdvanced();
    expect(setup.json.value).toContain("9223372036854775807");
  });
  it("preserves existing values, hides primary keys, and accepts advanced edits", async () => {
    wrapper = await mountSuspended(RowForm, {
      props: {
        ...props,
        record: { id: "key", title: "Hello", details: { value: 1 }, flag: false },
        error: "Save failed",
      },
    });
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.columns.value.some((value: { name: string }) => value.name === "id")).toBe(false);
    expect(setup.state.title.mode).toBe("omit");
    expect(() => setup.payload()).toThrow("Set at least one column value");
    setup.state.flag.mode = "value";
    expect(setup.payload()).toEqual({ flag: false });
    await nextTick();
    expect(wrapper.text()).toContain("Save failed");
    setup.toggleAdvanced();
    expect(setup.validate()).toEqual([]);
    wrapper.unmount();
    wrapper = await mountSuspended(RowForm, {
      props: { ...props, table: { ...table, columns: null, foreignKeys: null } },
    });
    expect(wrapper.setupState.columns.value).toEqual([]);
    expect(wrapper.setupState.foreignKey("missing")).toBeUndefined();
  });
  it("searches actual related rows and recovers failed reference queries", async () => {
    const api = useNuxtApp().$client.value.data.cp;
    const name = `ref_${crypto.randomUUID().replaceAll("-", "")}`;
    await api.tables.create({ schema: "public", name, columns: [column("name")] });
    try {
      const row = await useNuxtApp().$client.value.data.records.create("public", name, {
        name: "Related row",
      });
      const related = await api.tables.get("public", name);
      const source = {
        ...table,
        columns: [column("related_id", "uuid")],
        foreignKeys: [
          {
            column: "related_id",
            foreignSchema: "public",
            foreignTable: name,
            foreignColumn: "id",
          },
        ],
      } as DataControlPlaneModels.Table;
      wrapper = await mountSuspended(RowForm, {
        props: { ...props, table: source, tables: [related] },
        attachTo: document.body,
      });
      const setup = wrapper.setupState;
      await updateDataModels(wrapper, true);
      await vi.waitFor(() => expect(setup.references.related_id?.loading).toBe(false));
      expect(setup.references.related_id.options[0].label).toBe("Related row");
      setup.state.related_id.mode = "value";
      setup.state.related_id.value = row.id;
      await setup.searchReferenceDebounceFn("related_id", "Related");
      expect(setup.references.related_id.options[0].value).toBe(row.id);
      await setup.loadReference("related_id", "missing");
      expect(setup.references.related_id.options[0].value).toBe(row.id);
      wrapper.unmount();
      wrapper = await mountSuspended(RowForm, {
        props: { ...props, table: source, tables: [] },
        attachTo: document.body,
      });
      await wrapper.setupState.loadReference("related_id", String(row.id));
      expect(wrapper.setupState.references.related_id.options[0].value).toBe(row.id);
      const records = useNuxtApp().$client.value.data.records;
      let release!: (value: never[]) => void;
      vi.spyOn(records, "list").mockReturnValueOnce(
        new Promise<never[]>((resolve) => {
          release = resolve;
        }),
      );
      const stale = wrapper.setupState.loadReference("related_id", "");
      await wrapper.setupState.loadReference("related_id", String(row.id));
      release([]);
      await stale;
      let reject!: (reason: unknown) => void;
      vi.spyOn(records, "list").mockReturnValueOnce(
        new Promise<never[]>((_resolve, fail) => {
          reject = fail;
        }),
      );
      const staleFailure = wrapper.setupState.loadReference("related_id", "");
      await wrapper.setupState.loadReference("related_id", String(row.id));
      reject(new Error("Stale"));
      await staleFailure;
      vi.spyOn(records, "list").mockRejectedValueOnce("Unavailable");
      await wrapper.setupState.loadReference("related_id", "");
      expect(wrapper.setupState.references.related_id.error).toBe("Could not load related rows.");
      await api.tables.delete("public", name);
      vi.spyOn(records, "list").mockRejectedValueOnce(new Error("Related rows unavailable"));
      await wrapper.setupState.loadReference("related_id", "");
      expect(wrapper.setupState.references.related_id.error).toBeTruthy();
      wrapper.setupState.state.related_id.mode = "value";
      await nextTick();
      await updateDataModels(wrapper);
      await action("Retry Related Rows").trigger("click");
      delete wrapper.setupState.references.related_id;
      await nextTick();
      wrapper.findComponent(USelectMenu as Component).vm.$emit("update:searchTerm", "");
      await wrapper.setupState.searchReferenceDebounceFn("related_id", "");
    } finally {
      await api.tables.delete("public", name);
    }
  });
});
