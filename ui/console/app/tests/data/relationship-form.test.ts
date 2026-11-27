import { updateDataModels } from "./model-events";
import { afterEach, describe, expect, it } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { nextTick, type Component } from "vue";
import { USelectMenu } from "#components";
import type { DataControlPlaneModels } from "layr-client";
import RelationshipForm from "../../components/data/RelationshipForm.vue";
let wrapper: Awaited<ReturnType<typeof mountSuspended<typeof RelationshipForm>>>;
afterEach(() => wrapper?.unmount());
describe("Data relationship form", () => {
  it("selects actual tables and unique columns and previews quoted DDL", async () => {
    const source = {
      schema: "public",
      name: "orders",
      columns: [{ name: "customer_id" }],
    } as DataControlPlaneModels.Table;
    const target = {
      schema: "public",
      name: "customers",
      indexes: [
        { isUnique: true, columns: ["id"] },
        { isUnique: false, columns: ["name"] },
        { isUnique: true, columns: ["a", "b"] },
        { isUnique: true, columns: null },
      ],
    } as DataControlPlaneModels.Table;
    wrapper = await mountSuspended(RelationshipForm, {
      props: { table: source, tables: [source, target], busy: false, error: "" },
    });
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    expect(setup.validate()).toHaveLength(4);
    expect(setup.sql.value).toBe("");
    setup.state.name = "orders_customer_fkey";
    setup.state.column = "customer_id";
    setup.state.targetColumn = "old";
    wrapper
      .findAllComponents(USelectMenu as Component)[1]!
      .vm.$emit("update:modelValue", "public.customers");
    expect(setup.state.targetColumn).toBe("");
    setup.state.targetColumn = "id";
    expect(setup.targetColumns.value).toEqual(["id"]);
    expect(setup.validate()).toEqual([]);
    expect(setup.sql.value).toContain('ALTER TABLE "public"."orders"');
    expect(setup.sql.value).toContain('REFERENCES "public"."customers" ("id")');
    setup.save();
    expect(wrapper.emitted("save")?.[0]?.[0]).toBe(setup.sql.value);
    setup.state.target = "public.orders";
    await nextTick();
    expect(setup.targetColumns.value).toEqual([]);
    await wrapper.setProps({
      table: { ...source, columns: null },
      error: "Cannot create constraint",
    });
    expect(wrapper.text()).toContain("Relationship Not Saved");
    expect(setup.validate()).toHaveLength(2);
  });
});
