import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick, type Component } from "vue";
import { USelectMenu } from "#components";
import { useNuxtApp, useState } from "#app";

import Api from "../../pages/data/api.vue";
import {
  action,
  activeDialog,
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
describe("Data API explorer", () => {
  it("validates GraphQL documents, confirms execution, and renders real results and errors", async () => {
    ({ wrapper } = await mountPage(Api));
    const setup = wrapper.setupState;
    await updateDataModels(wrapper, true);
    setup.state.query = "";
    setup.state.variables = "[]";
    expect(setup.validate()).toHaveLength(2);
    setup.state.variables = "null";
    expect(setup.validate()).toHaveLength(2);
    setup.state.variables = "invalid";
    expect(setup.validate()).toHaveLength(2);
    setup.state.query = "query Example { __schema { queryType { name } } }";
    setup.state.operationName = "Example";
    setup.state.variables = "{}";
    expect(setup.validate()).toEqual([]);
    setup.request();
    await nextTick();
    await action("Cancel", activeDialog()!).trigger("click");
    setup.request();
    await nextTick();
    await action("Execute", activeDialog()!).trigger("click");
    await settle();
    expect(setup.response.value, setup.dataControlPlane.error.value).not.toBeNull();
    expect(pageDOM().text()).toContain("GraphQL Response");
    setup.state.query = "query { missing_console_field }";
    setup.state.operationName = "";
    await nextTick();
    expect(pageDOM().text()).toContain("Document Changed");
    await setup.execute();
    await settle();
    expect(setup.response.value?.errors?.length || setup.dataControlPlane.error.value).toBeTruthy();
    setup.response.value = { errors: [{ message: "Example validation error" }], data: null };
    await nextTick();
    expect(pageDOM().text()).toContain("GraphQL Errors");
    setup.tab.value = "rest";
    await nextTick();
    await updateDataModels(wrapper, true);
    expect(pageDOM().text()).toContain("Choose a Table");
    const first = setup.dataControlPlane.tables.value[0];
    setup.tableName.value = `${first.schema}.${first.name}`;
    await nextTick();
    expect(setup.restUrl.value).toContain(`/v1/data/${first.schema}/${first.name}?limit=25`);
    expect(pageDOM().text()).toContain("Never expose a Service Account key");
    await action("Send Request").trigger("click");
    await settle();
    expect(setup.restResult.value?.status, setup.dataControlPlane.error.value).toBe(200);
    expect(setup.restResult.value?.request).toContain("limit=25");
    expect(setup.curl.value).toContain("<YOUR_TOKEN>");
    setup.tab.value = "graphql";
    await signInConsoleUser(["data:query.read"]);
    await nextTick();
    expect(pageDOM().text()).toContain("Execution Access Required");
  });
  it("carries a table request into REST and confirms writes", async () => {
    const nuxtApp = useNuxtApp();
    const dataClient = nuxtApp.$client.value.data;
    const name = `api_${crypto.randomUUID().replaceAll("-", "")}`;
    await dataClient.cp.tables.create({
      schema: "public",
      name,
      columns: [
        { name: "title", type: "text", isNullable: true, isPrimaryKey: false, isUnique: false },
      ],
    });
    try {
      useState("data-workspace-owner").value = nuxtApp.$auth.state.value?.user.id;
      useState("data-workspace-request").value = {
        table: `public.${name}`,
        params: { limit: "1", select: "id,title" },
      };
      await nuxtApp.$router.push({
        path: "/data/api",
        query: { tab: "rest", table: `public.${name}` },
      });
      ({ wrapper } = await mountPage(Api));
      const setup = wrapper.setupState;
      expect(setup.tab.value).toBe("rest");
      expect(setup.tableName.value).toBe(`public.${name}`);
      expect(setup.parameters.value).toContainEqual({ name: "select", value: "id,title" });
      setup.recordId.value = "old-record";
      wrapper
        .findAllComponents(USelectMenu as Component)[0]!
        .vm.$emit("update:modelValue", `public.${name}`);
      await nextTick();
      expect(setup.recordId.value).toBe("");
      await action("Add Parameter").trigger("click");
      await action("Remove parameter 3").trigger("click");
      setup.method.value = "POST";
      setup.restBody.value = '{"title":"Console REST"}';
      await nextTick();
      await updateDataModels(wrapper);
      await action("Copy curl").trigger("click");
      const request = wrapper.get('[aria-label="REST Request"]');
      expect(request.get('[aria-label="Request URL"]').attributes("readonly")).toBeDefined();
      expect(action("Send Request", request.element).attributes("type")).toBe("submit");
      await wrapper.get("#data-rest").trigger("submit");
      await flushPromises();
      expect(setup.restResult.value).toBeNull();
      await action("Cancel", activeDialog()!).trigger("click");
      await wrapper.get("#data-rest").trigger("submit");
      await flushPromises();
      await action("Send write request", activeDialog()!).trigger("click");
      await settle();
      expect(setup.restResult.value?.status).toBe(201);
      const inserted = JSON.parse(setup.restResult.value.body);
      expect(inserted.title).toBe("Console REST");
      setup.method.value = "PATCH";
      expect(setup.canSend.value).toBe(false);
      await setup.executeRest();
      await nextTick();
      await updateDataModels(wrapper);
      setup.recordId.value = inserted.id;
      setup.restBody.value = '{"title":"Updated REST"}';
      expect(setup.curl.value).toContain("--data");
      expect(setup.curl.value).not.toContain(nuxtApp.$client.value.sessionToken!);
      await setup.executeRest();
      expect(setup.restResult.value?.status).toBe(200);
      expect(JSON.parse(setup.restResult.value.body).title).toBe("Updated REST");
      setup.method.value = "DELETE";
      await setup.executeRest();
      expect(setup.restResult.value?.status).toBe(204);
      setup.method.value = "GET";
      await setup.executeRest();
      expect(setup.restResult.value?.status).toBe(404);
      await action("Response Headers").trigger("click");
      expect(activeDialog()?.textContent).toContain("content-type");
      setup.restResult.value.headers = [];
      await nextTick();
      expect(activeDialog()?.textContent).toContain("No Headers Available");
      await action("Close", activeDialog()!).trigger("click");
      await action("Copy body").trigger("click");
    } finally {
      await dataClient.cp.tables.delete("public", name);
      await nuxtApp.$router.push("/");
    }
  });
  it("does not fetch schema metadata without permission", async () => {
    await signInConsoleUser(["data:query.read"]);
    ({ wrapper } = await mountPage(Api));
    wrapper.setupState.tab.value = "rest";
    await nextTick();
    expect(wrapper.setupState.options.value).toEqual([]);
    expect(wrapper.setupState.restUrl.value).toBe("");
  });
});
