import { updateDataModels } from "./model-events";
import { UModal, UTable, UTabs } from "#components";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import Tables from "../../pages/data/index.vue";
import SchemaForm from "../../components/data/SchemaForm.vue";
import {
  action,
  activeDialog,
  field,
  menuAction,
  flushPromises,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let names: string[] = [];
beforeEach(async () => {
  await signInConsoleUser();
  names = [];
});
afterEach(async () => {
  wrapper?.unmount();
  await signInConsoleUser();
  for (const name of names.toReversed())
    await useNuxtApp().$client.value.data.cp.tables.delete("public", name);
  await useNuxtApp().$router.push("/");
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.dataControlPlane.busy.value).toBe(false));
}
function name() {
  const value = `ui_${crypto.randomUUID().replaceAll("-", "")}`;
  names.push(value);
  return value;
}
function draft(overrides = {}) {
  return {
    schema: "public",
    name: "title",
    type: "text",
    nullable: true,
    defaultValue: "",
    unique: false,
    columns: [],
    command: "SELECT",
    roles: ["PUBLIC"],
    permissive: "PERMISSIVE",
    usingExpression: "true",
    withCheckExpression: "",
    ...overrides,
  };
}
async function tab(value: string) {
  wrapper.setupState.tab.value = value;
  await nextTick();
  await updateDataModels(wrapper);
  wrapper.findComponent(UTabs).vm.$emit("update:modelValue", value);
  await settle();
}
async function confirm() {
  await field(`Type ${wrapper.setupState.deleteName.value} to confirm`).setValue(
    wrapper.setupState.deleteName.value,
  );
  await action("Confirm", activeDialog()!).trigger("click");
  await settle();
}
function dataRow(text: string) {
  const row = pageDOM()
    .findAll("tbody tr")
    .find((row) => row.text().includes(text));
  expect(row).toBeDefined();
  return row!;
}
function rowAction(text: string, label: string) {
  return menuAction(label, dataRow(text).element);
}
function tableAction(label: string) {
  return menuAction(label, wrapper.get('[aria-label="Table actions"]').element.parentElement!);
}
describe("Data table editor", () => {
  it("creates tables, edits columns and indexes, and uses exact destructive confirmations", async () => {
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    await action("Create Table").trigger("click");
    const tableName = name();
    const schemaForm = wrapper.findComponent(SchemaForm);
    schemaForm.vm.$emit("save", draft({ name: tableName }));
    await settle();
    expect(state.table.value.name).toBe(tableName);
    expect(state.selectedKey.value).toBe(`public.${tableName}`);
    expect(pageDOM().text()).toContain("Row-Level Security Disabled");
    await tab("columns");
    expect(state.queryTable()).toContain(encodeURIComponent(`public.${tableName}`));
    await tableAction("Create Table");
    await state.saveSchema(draft({ name: tableName }));
    await nextTick();
    expect(activeDialog()?.textContent).toContain("already exists");
    await action("Cancel", activeDialog()!).trigger("click");
    await action("Add Column").trigger("click");
    await state.saveSchema(draft());
    await settle();
    expect(state.table.value.columns.map((column: { name: string }) => column.name)).toContain(
      "title",
    );
    await rowAction("title", "Edit");
    await state.saveSchema(draft({ name: "description", defaultValue: "'draft'" }));
    await settle();
    expect(pageDOM().text()).toContain("description");
    await action("Add Column").trigger("click");
    await state.saveSchema(
      draft({ name: "nullable", type: "integer", defaultValue: "0", unique: true }),
    );
    await settle();
    await tab("relationships");
    expect(pageDOM().text()).toContain("No Foreign Keys");
    await tab("indexes");
    await action("Create Index").trigger("click");
    await state.saveSchema(
      draft({
        name: "index_for_" + tableName,
        type: "btree",
        columns: ["description"],
        unique: true,
      }),
    );
    await settle();
    expect(pageDOM().text()).toContain("index_for_");
    await rowAction("index_for_", "Delete");
    expect(action("Confirm", activeDialog()!).attributes("disabled")).toBeDefined();
    await confirm();
    await action("Create Index").trigger("click");
    await state.saveSchema(
      draft({ name: "hash_for_" + tableName, type: "hash", columns: ["description"] }),
    );
    await settle();
    await tab("columns");
    await rowAction("description", "Delete");
    await confirm();
    expect(state.table.value.columns.map((column: { name: string }) => column.name)).not.toContain(
      "description",
    );
    await tableAction("Delete Table");
    await action("Cancel", activeDialog()!).trigger("click");
    await tableAction("Delete Table");
    await confirm();
    expect(state.table.value).toBeNull();
    expect(state.selectedKey.value).toBe("");
  });
  it("manages RLS and policies without pretending policy editing exists", async () => {
    const tableName = name();
    const api = useNuxtApp().$client.value.data.cp;
    await api.tables.create({ schema: "public", name: tableName, columns: [] });
    await useNuxtApp().$router.push({ path: "/data", query: { table: `public.${tableName}` } });
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    await tab("policies");
    expect(pageDOM().text()).toContain("Row-Level Security Disabled");
    await action("Create Policy").trigger("click");
    await state.saveSchema(draft({ name: "read_rows" }));
    await settle();
    expect(pageDOM().text()).toContain("read_rows");
    await tableAction("Enable RLS");
    await settle();
    expect(state.table.value.rlsEnabled).toBe(true);
    await tableAction("Force RLS");
    await confirm();
    expect(state.table.value.rlsForced).toBe(true);
    await tableAction("Unforce RLS");
    await confirm();
    expect(state.table.value.rlsForced).toBe(false);
    await tableAction("Disable RLS");
    await confirm();
    expect(state.table.value.rlsEnabled).toBe(false);
    await rowAction("read_rows", "Delete");
    await confirm();
    expect(state.dataControlPlane.policies.value).toEqual([]);
    await action("Create Policy").trigger("click");
    await state.saveSchema(
      draft({ name: "insert_rows", command: "INSERT", withCheckExpression: "true" }),
    );
    await settle();
    await action("Create Policy").trigger("click");
    await state.saveSchema(
      draft({ name: "all_rows", command: "ALL", withCheckExpression: "true" }),
    );
    await settle();
    await action("Create Policy").trigger("click");
    await state.saveSchema(draft({ name: "bad_policy", usingExpression: "does_not_exist" }));
    await settle();
    expect(state.schemaOpen.value).toBe(true);
    expect(activeDialog()?.textContent).toContain("Request Failed");
    await action("Cancel", activeDialog()!).trigger("click");
    await tableAction("Delete Table");
    await signInConsoleUser(["data:schema.read"]);
    await confirm();
    expect(state.deleteOpen.value).toBe(true);
    expect(activeDialog()?.textContent).toContain("Request Failed");
    await action("Cancel", activeDialog()!).trigger("click");
    expect(pageDOM().text()).not.toContain("Create Policy");
  });
  it("browses, filters, paginates, inserts, edits, and deletes real REST rows", async () => {
    const tableName = name();
    const api = useNuxtApp().$client.value.data.cp;
    await api.tables.create({
      schema: "public",
      name: tableName,
      columns: [
        { name: "title", type: "text", isNullable: true, isPrimaryKey: false, isUnique: false },
      ],
    });
    for (let index = 0; index < 28; index++)
      await useNuxtApp().$client.value.data.records.create("public", tableName, {
        title: `Row ${index}`,
      });
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    await updateDataModels(wrapper);
    await state.chooseTable(`public.${tableName}`);
    await tab("rows");
    const rowTable = wrapper.findAllComponents(UTable).at(-1)!;
    rowTable.vm.$emit("update:columnSizing", {});
    expect(state.rows.value).toHaveLength(25);
    expect(state.hasNext.value).toBe(true);
    const [firstRow, secondRow] = rowTable.findAll("tbody tr");
    await firstRow!.trigger("keydown", { key: "ArrowDown" });
    expect(document.activeElement).toBe(secondRow!.element);
    await secondRow!.trigger("keydown", { key: "ArrowUp" });
    expect(document.activeElement).toBe(firstRow!.element);
    const resize = action("Resize title");
    const header = resize.element.closest("th")!;
    const table = header.closest("table")!;
    expect(table.classList.contains("table-fixed")).toBe(true);
    expect(firstRow!.get("td").classes()).toContain("truncate");
    expect(header.style.width).toBe("180px");
    await resize.trigger("keydown", { key: "ArrowRight" });
    expect(state.columnSizing.value["column-title"]).toBe(200);
    expect(header.style.width).toBe("200px");
    await resize.trigger("keydown", { key: "ArrowLeft" });
    expect(state.columnSizing.value["column-title"]).toBe(180);
    expect(header.style.width).toBe("180px");
    for (let step = 0; step < 7; step++) await resize.trigger("keydown", { key: "ArrowLeft" });
    expect(header.style.width).toBe("60px");
    await resize.trigger("keydown", { key: "ArrowDown" });
    await resize.trigger("mousedown", { clientX: 100 });
    document.dispatchEvent(new MouseEvent("mouseup", { clientX: 100 }));
    await resize.trigger("touchstart", { touches: [{ clientX: 100 }, { clientX: 101 }] });
    await action("Next").trigger("click");
    await settle();
    expect(state.offset.value).toBe(25);
    await action("Previous").trigger("click");
    await settle();
    expect(state.offset.value).toBe(0);
    state.filterColumn.value = "title";
    state.filterValue.value = "Row 0";
    state.orderColumn.value = "title";
    await action("Apply Filter").trigger("click");
    await settle();
    expect(state.rows.value).toHaveLength(1);
    await dataRow("Row 0").trigger("click");
    expect(pageDOM().text()).toContain("Row Details");
    await action("Copy title").trigger("click");
    await action("Copy row JSON").trigger("click");
    await action("Close row details").trigger("click");
    await rowAction("Row 0", "Edit");
    wrapper
      .findAllComponents(UModal)
      .find((item) => item.props("title") === "Edit Row")!
      .vm.$emit("update:open", false);
    await nextTick();
    await dataRow("Row 0").trigger("click");
    await action("Edit Row").trigger("click");

    await state.saveRecord({ title: "Edited" });
    await settle();
    expect(state.recordOpen.value).toBe(false);
    await action("Clear Filter").trigger("click");
    await settle();
    await action("Insert Row").trigger("click");
    await state.saveRecord({ title: "Created" });
    await settle();
    state.filterValue.value = "Created";
    await state.loadRows();
    await settle();
    expect(state.rows.value).toHaveLength(1);
    expect(state.rowCanEdit(state.rows.value[0])).toBe(true);
    await rowAction("Created", "Delete");
    await confirm();
    expect(state.rows.value).toHaveLength(0);
    await action("Insert Row").trigger("click");
    await state.saveRecord({ missing_column: true });
    await settle();
    expect(state.recordOpen.value).toBe(true);
    expect(activeDialog()?.textContent).toContain("Row Not Saved");
    await action("Cancel", activeDialog()!).trigger("click");
    await action("Insert Row").trigger("click");
    await action("Cancel", activeDialog()!).trigger("click");
    state.table.value.primaryKey = "";
    expect(state.rowCanEdit({})).toBe(false);
    state.orderColumn.value = "";
    state.filterColumn.value = "";
    await state.loadRows();
    state.table.value = null;
    await state.loadRows();
  });
  it("keeps table context when opening API Explorer", async () => {
    const { $router: router, $client: client } = useNuxtApp();
    const tableName = name();
    await client.value.data.cp.tables.create({
      schema: "public",
      name: tableName,
      columns: [
        {
          name: "enabled",
          type: "boolean",
          isNullable: true,
          isPrimaryKey: false,
          isUnique: false,
        },
        { name: "starts", type: "date", isNullable: true, isPrimaryKey: false, isUnique: false },
      ],
    });
    try {
      const key = `public.${tableName}`;
      await router.push({ path: "/data", query: { table: key, tab: "rows" } });
      ({ wrapper } = await mountPage(Tables));
      const state = wrapper.setupState;
      await vi.waitFor(() => expect(state.selectedKey.value).toBe(key));
      state.filterColumn.value = "enabled";
      state.filterValue.value = "true";
      await nextTick();
      await updateDataModels(wrapper);
      state.filterOperator.value = "is";
      state.filterValue.value = "null";
      await nextTick();
      await updateDataModels(wrapper);
      state.filterColumn.value = "starts";
      state.filterOperator.value = "eq";
      state.filterValue.value = "2026-01-01";
      await nextTick();
      await wrapper.get('input[aria-label="Filter value"]').trigger("keydown", { key: "Enter" });
      await settle();
      expect(state.appliedFilter.value).toBe("starts eq 2026-01-01");
      await action("All Tables", wrapper.element).trigger("click");
      expect(state.dataWorkspace.grids.value[key].filterValue).toBe("2026-01-01");
      state.tab.value = "columns";
      await state.chooseTable(key);
      expect(state.filterValue.value).toBe("2026-01-01");
      await tableAction("Open in API Explorer");
      await vi.waitFor(() =>
        expect(router.currentRoute.value.query).toMatchObject({ table: key, tab: "rest" }),
      );
      expect(router.currentRoute.value.fullPath).not.toContain("2026-01-01");
    } finally {
      await router.push("/");
      await client.value.data.cp.tables.delete("public", tableName);
    }
  });
  it("creates and removes real foreign key relationships", async () => {
    const parentName = name();
    const childName = name();
    const api = useNuxtApp().$client.value.data.cp;
    await api.tables.create({
      schema: "public",
      name: parentName,
      columns: [
        { name: "name", type: "text", isNullable: true, isPrimaryKey: false, isUnique: false },
      ],
    });
    await api.tables.create({
      schema: "public",
      name: childName,
      columns: [
        { name: "parent_id", type: "uuid", isNullable: true, isPrimaryKey: false, isUnique: false },
      ],
    });
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    await state.chooseTable(`public.${childName}`);
    await tab("relationships");
    await action("Add Relationship").trigger("click");
    await state.saveRelationship(
      `ALTER TABLE public.${childName} ADD CONSTRAINT child_parent_fkey FOREIGN KEY (parent_id) REFERENCES public.${parentName}(id);`,
    );
    await settle();
    expect(state.relationshipOpen.value).toBe(false);
    expect(pageDOM().text()).toContain("child_parent_fkey");
    const records = useNuxtApp().$client.value.data.records;
    await records.create("public", parentName, { name: "Related customer" });
    const [parent] = await records.list("public", parentName);
    await records.create("public", childName, { parent_id: parent!.id });
    await tab("rows");
    state.filterColumn.value = "id";
    state.filterValue.value = state.rows.value[0].id;
    await state.loadRows();
    await dataRow(String(parent!.id)).trigger("click");
    await vi.waitFor(() => expect(state.related.value.parent_id?.label).toBe("Related customer"));
    await nextTick();
    await action("Open related record: Related customer").trigger("click");
    await settle();
    expect(state.selectedKey.value).toBe(`public.${parentName}`);
    expect(state.rows.value[0].name).toBe("Related customer");
    await action(`Back to public.${childName}`).trigger("click");
    await settle();
    expect(state.filterColumn.value).toBe("id");
    expect(state.rows.value).toHaveLength(1);
    await state.followRelation({ table: "public.missing_related", column: "id", value: "missing" });
    expect(state.previousTable.value).toBe("");
    await state.chooseTable(`public.${childName}`);
    const catalog = state.dataControlPlane.tables.value;
    const row = state.rows.value[0];
    state.dataControlPlane.tables.value = catalog.filter(
      (entry: { name: string }) => entry.name !== parentName,
    );
    state.inspected.value = row;
    await nextTick();
    expect(state.related.value).toEqual({});
    state.inspected.value = null;
    state.dataControlPlane.tables.value = catalog;
    const list = vi
      .spyOn(records, "list")
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([{ id: parent!.id }]);
    await nextTick();
    state.inspected.value = row;
    await flushPromises();
    expect(state.related.value).toEqual({});
    state.inspected.value = null;
    await nextTick();
    state.inspected.value = row;
    await vi.waitFor(() => expect(state.related.value.parent_id.label).toBe(parent!.id));
    const pending = Promise.withResolvers<Awaited<ReturnType<typeof records.list>>>();
    list.mockReturnValueOnce(pending.promise);
    state.inspected.value = null;
    await nextTick();
    state.inspected.value = row;
    await nextTick();
    state.inspected.value = null;
    await nextTick();
    pending.resolve([{ id: parent!.id, name: "Stale" }]);
    await flushPromises();
    expect(state.related.value).toEqual({});
    list.mockRestore();
    await tab("relationships");
    await rowAction("child_parent_fkey", "Delete");
    await confirm();
    expect(state.table.value.foreignKeys).toEqual([]);
    state.table.value.foreignKeys = null;
    state.inspected.value = row;
    await nextTick();
    expect(state.related.value).toEqual({});
    state.inspected.value = null;
    await action("Add Relationship").trigger("click");
    await state.saveRelationship("INVALID SQL");
    await nextTick();
    expect(activeDialog()?.textContent).toContain("Relationship Not Saved");
    await action("Cancel", activeDialog()!).trigger("click");
    await action("All Tables").trigger("click");
    expect(state.table.value).toBeNull();
  });
  it("preserves unchanged column metadata and safely handles unsupported row identities", async () => {
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    const first = state.visibleTables.value[0];
    await dataRow(`${first.schema}.${first.name}`).trigger("click");
    await settle();
    await action("All Tables").trigger("click");
    const tableName = name();
    state.kind.value = "table";
    await state.saveSchema(
      draft({
        name: tableName,
        tableColumns: [
          {
            name: "value",
            type: "integer",
            isNullable: true,
            defaultValue: "0",
            isPrimaryKey: false,
            isUnique: false,
          },
          { name: "extra", type: "text", isNullable: true, isUnique: false, isPrimaryKey: false },
        ],
      }),
    );
    await tab("columns");
    const extra = state.table.value.columns.find(
      (column: { name: string }) => column.name === "extra",
    );
    state.openSchema("column", extra);
    await state.saveSchema(draft({ name: "extra" }));
    const current = state.table.value.columns.find(
      (column: { name: string }) => column.name === "value",
    );
    state.openSchema("column", current);
    await state.saveSchema(draft({ name: "value", type: "integer", defaultValue: "0" }));
    expect(state.schemaOpen.value).toBe(false);
    state.openSchema("column", current);
    await state.saveSchema(
      draft({ name: "value", type: "integer", nullable: false, defaultValue: "0" }),
    );
    state.openSchema("column", current);
    await state.saveSchema(
      draft({ name: "value", type: "bigint", nullable: false, defaultValue: "" }),
    );
    await settle();
    expect(
      state.table.value.columns.find((column: { name: string }) => column.name === "value").type,
    ).toBe("bigint");
    state.table.value.indexes = null;
    expect(state.rowCanEdit({ id: "known" })).toBe(true);
    state.table.value.indexes = [
      { isUnique: false, columns: [] },
      { isUnique: true, columns: null },
    ];
    expect(state.rowCanEdit({ id: "known" })).toBe(true);
    state.table.value.indexes = [{ isUnique: true, columns: ["id", "value"] }];
    expect(state.rowCanEdit({ id: "known" })).toBe(false);
    state.table.value.indexes = [{ isUnique: true, columns: ["value", "other"] }];
    expect(state.rowCanEdit({})).toBe(false);
    state.table.value.indexes = [];
    expect(state.rowCanEdit({})).toBe(false);
    state.table.value.primaryKey = "";
    state.orderColumn.value = "value";
    await state.loadRows();
    state.requestDelete("record", crypto.randomUUID());
    await nextTick();
    await confirm();
    expect(state.deleteOpen.value).toBe(true);
    await action("Cancel", activeDialog()!).trigger("click");
    state.table.value = null;
    expect(state.rowCanEdit({})).toBe(false);
    await useNuxtApp().$client.value.data.cp.tables.delete("public", tableName);
    const previousColumns = [...state.visibleColumns.value];
    await state.chooseTable(`public.${tableName}`);
    expect(state.visibleColumns.value).toEqual(previousColumns);
    expect(state.dataControlPlane.error.value).toBeTruthy();
    wrapper.unmount();
    await signInConsoleUser(["data:schema.read"]);
    ({ wrapper } = await mountPage(Tables));
    expect(wrapper.setupState.tab.value).toBe("columns");
  });
  it("handles scope restrictions, empty metadata, and safe table switching", async () => {
    ({ wrapper } = await mountPage(Tables));
    const state = wrapper.setupState;
    const tableName = name();
    await useNuxtApp().$client.value.data.cp.tables.create({
      schema: "public",
      name: tableName,
      columns: [],
    });
    await action("Refresh").trigger("click");
    await settle();
    state.search.value = "no-matching-table";
    await nextTick();
    expect(pageDOM().text()).toContain("No Tables Found");
    state.search.value = "";
    state.schemaFilter.value = "public";
    await nextTick();
    expect(
      state.tableOptions.value.every((option: { value: string }) =>
        option.value.startsWith("public."),
      ),
    ).toBe(true);
    await state.chooseTable(`public.${tableName}`);
    state.tab.value = "rows";
    await state.chooseTable(`public.${tableName}`);
    await settle();
    state.dataControlPlane.busy.value = true;
    await state.chooseTable("missing");
    state.dataControlPlane.busy.value = false;
    await state.chooseTable("missing");
    expect(state.table.value).toBeNull();
    await state.chooseTable(`public.${tableName}`);
    await signInConsoleUser(["data:schema.read"]);
    await nextTick();
    expect(state.tabs.value.map((tab: { value: string }) => tab.value)).not.toContain("rows");
    expect(state.rowCanEdit({ id: "test" })).toBe(false);
    await state.loadRows();
    state.table.value.columns = null;
    state.table.value.foreignKeys = null;
    state.table.value.indexes = null;
    await nextTick();
    expect(state.columnNames.value).toEqual([]);
    for (const value of ["indexes", "relationships", "policies", "columns"]) await tab(value);
    state.dataControlPlane.tables.value = [];
    await nextTick();
    expect(state.tableOptions.value).toEqual([]);
    await signInConsoleUser([]);
    await state.load();
  });
});
