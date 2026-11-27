import { updateDataModels } from "./model-events";
import { afterEach, describe, expect, it } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { nextTick } from "vue";
import type { DataControlPlaneModels } from "layr-client";
import SchemaForm from "../../components/data/SchemaForm.vue";
let wrapper: Awaited<ReturnType<typeof mountSuspended<typeof SchemaForm>>>;
afterEach(() => wrapper?.unmount());
const props = {
  kind: "table" as const,
  table: null,
  column: null,
  busy: false,
  error: "",
  schemas: ["public"],
};
describe("Data schema forms", () => {
  it("validates table and column names and preserves column defaults", async () => {
    wrapper = await mountSuspended(SchemaForm, { props });
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.validate()).toHaveLength(1);
    setup.state.name = "items";
    expect(setup.validate()).toEqual([]);
    setup.save();
    expect(wrapper.emitted("save")?.[0]?.[0]).toMatchObject({ name: "items", schema: "public" });
    wrapper.unmount();
    wrapper = await mountSuspended(SchemaForm, {
      props: {
        ...props,
        kind: "column",
        table: { schema: "custom" } as DataControlPlaneModels.Table,
        column: {
          name: "created",
          type: "timestamp",
          isNullable: false,
          isPrimaryKey: false,
          defaultValue: "now()",
        } as DataControlPlaneModels.Column,
        error: "Failed",
      },
    });
    expect(wrapper.setupState.state).toMatchObject({
      schema: "custom",
      name: "created",
      type: "timestamp",
      nullable: false,
      defaultValue: "now()",
    });
    expect(wrapper.text()).toContain("Schema Change");
    expect(wrapper.text()).toContain("Request Failed");
    await updateDataModels(wrapper);
    expect(wrapper.setupState.validate()).toEqual([]);
    wrapper.unmount();
    wrapper = await mountSuspended(SchemaForm, { props: { ...props, kind: "column" } });
    expect(wrapper.text()).toContain("Unique values");
    await updateDataModels(wrapper);
  });
  it("creates and validates initial table columns", async () => {
    wrapper = await mountSuspended(SchemaForm, { props: { ...props, schemas: [] } });
    const setup = wrapper.setupState;
    setup.state.name = "items";
    setup.addTableColumn();
    expect(setup.validate()).toHaveLength(1);
    setup.state.tableColumns[0].name = "id";
    expect(setup.validate()).toHaveLength(1);
    setup.state.tableColumns[0].name = "title";
    expect(setup.validate()).toEqual([]);
    setup.addTableColumn();
    setup.state.tableColumns[1].name = "title";
    expect(setup.validate()).toHaveLength(1);
    setup.state.tableColumns[1].name = "description";
    await nextTick();
    await updateDataModels(wrapper);
    const remove = wrapper.findAll("button").find((button) => button.text() === "Remove Column")!;
    await remove.trigger("click");
    expect(setup.state.tableColumns).toHaveLength(1);
    setup.save();
    expect(wrapper.emitted("save")?.[0]?.[0]).toMatchObject({ schema: "public", name: "items" });
  });
  it("requires index columns and validates policy expressions", async () => {
    wrapper = await mountSuspended(SchemaForm, {
      props: {
        ...props,
        kind: "index",
        table: { columns: [{ name: "id" }] } as DataControlPlaneModels.Table,
      },
    });
    await updateDataModels(wrapper);
    wrapper.setupState.state.name = "items_idx";
    expect(wrapper.setupState.state.type).toBe("btree");
    expect(wrapper.setupState.validate()).toHaveLength(1);
    wrapper.setupState.state.columns = ["id"];
    expect(wrapper.setupState.validate()).toEqual([]);
    wrapper.unmount();
    wrapper = await mountSuspended(SchemaForm, { props: { ...props, kind: "index" } });
    expect(wrapper.text()).toContain("Columns");
    wrapper.unmount();
    wrapper = await mountSuspended(SchemaForm, { props: { ...props, kind: "policy" } });
    await updateDataModels(wrapper);
    const state = wrapper.setupState.state;
    state.name = "read_own";
    state.roles = [];
    state.usingExpression = "true;";
    expect(wrapper.setupState.validate()).toHaveLength(2);
    state.roles = ["PUBLIC"];
    state.usingExpression = "true";
    expect(wrapper.setupState.validate()).toEqual([]);
    for (const command of ["INSERT", "UPDATE", "ALL", "DELETE", "SELECT"]) {
      state.command = command;
      await nextTick();
      await updateDataModels(wrapper);
      expect(wrapper.text().includes("USING Expression")).toBe(command !== "INSERT");
      expect(wrapper.text().includes("WITH CHECK Expression")).toBe(
        ["INSERT", "UPDATE", "ALL"].includes(command),
      );
    }
  });
});
