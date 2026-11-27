import { EditorView } from "@codemirror/view";
import SqlEditor from "../../components/data/SqlEditor.vue";
import { updateDataModels } from "./model-events";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { UModal, USplitter, UTabs } from "#components";
import { useNuxtApp } from "#app";
import Sql from "../../pages/data/sql.vue";
import {
  action,
  activeDialog,
  menuAction,
  mountPage,
  pageDOM,
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
describe("SQL page", () => {
  it("confirms administrative execution, renders dynamic values, and clears failed results", async () => {
    ({ wrapper } = await mountPage(Sql));
    const state = wrapper.setupState;
    await updateDataModels(wrapper, true);
    state.query.value =
      "select 1 as number, null as empty, jsonb_build_object('ok', true) as value; select 99 as second_statement;";
    await nextTick();
    await action("Run Query").trigger("click");
    expect(activeDialog()?.textContent).toContain("bypass row-level security");
    await action("Cancel", activeDialog()!).trigger("click");
    expect(state.confirmOpen.value).toBe(false);
    await state.requestExecution();
    await updateDataModels(wrapper);
    wrapper.findComponent(SqlEditor).vm.$emit("update:modelValue", state.query.value);
    await action("Execute SQL", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("1 row returned");
    expect(state.dataControlPlane.result.value.rows).toHaveLength(1);
    expect(state.dataControlPlane.result.value.columns).toEqual(["number", "empty", "value"]);
    expect(pageDOM().text()).not.toContain("rows affected");
    expect(pageDOM().text()).toContain("NULL");
    expect(pageDOM().text()).toContain('{"ok":true}');
    state.query.value = "select 2 as answer";
    await nextTick();
    expect(pageDOM().text()).toContain("Query changed since this run");
    await state.requestExecution();
    await action("Execute SQL", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("2");
    state.addDraft("select 3 as another_query");
    expect(state.dataControlPlane.result.value).toBeNull();
    state.changeDraft("1");
    expect(state.dataControlPlane.result.value?.rows).toEqual([{ answer: 2 }]);
    state.query.value = "select * from console_missing_table";
    await state.requestExecution();
    await action("Execute SQL", activeDialog()!).trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Request Failed");
    expect(state.dataControlPlane.result.value).toBeNull();
    state.query.value = " ";
    await state.requestExecution();
    state.query.value = "select 1";
    state.dataControlPlane.busy.value = true;
    await state.requestExecution();
    state.dataControlPlane.busy.value = false;
    await signInConsoleUser(["data:schema.read"]);
    await state.requestExecution();
    expect(state.confirmOpen.value).toBe(false);
  });
  it("keeps an empty editor and its results full width, with execution controls above the splitter", async () => {
    ({ wrapper } = await mountPage(Sql));
    await action("New query", wrapper.element).trigger("click");
    expect(wrapper.setupState.query.value).toBe("");
    expect(wrapper.findComponent(SqlEditor).classes()).toContain("w-full");
    expect(wrapper.get('[aria-labelledby="sql-results-title"]').classes()).toContain("w-full");
    const toolbar = wrapper.get('[aria-label="Query execution"]');
    const splitter = wrapper.findComponent(USplitter);
    expect(wrapper.text()).toContain(
      "Queries run with database privileges and can bypass row-level security",
    );
    expect(toolbar.element.parentElement?.nextElementSibling).toBe(splitter.element.parentElement);
    expect(splitter.element.parentElement?.classList.contains("h-128")).toBe(true);
    expect(action("Run Query", toolbar.element).attributes("disabled")).toBeDefined();
    expect(action("Run Selection", toolbar.element).exists()).toBe(true);
    const tabs = wrapper.findComponent(UTabs);
    expect(tabs.findAll('[role="tab"]')).toHaveLength(2);
    const warning = wrapper.getComponent({ name: "UAlert" });
    expect(
      warning.element.compareDocumentPosition(tabs.element) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    await action("New query", wrapper.element).trigger("click");
    await tabs.get('[data-close-query="1"]').trigger("mousedown");
    await tabs.get('[data-close-query="1"]').trigger("click");
    expect(wrapper.setupState.activeDraft.value).toBe("3");
    tabs.vm.$emit("update:modelValue", "2");
    await nextTick();
    await tabs.get('[role="tab"][aria-selected="true"]').trigger("keydown", { key: "Delete" });
    expect(wrapper.setupState.activeDraft.value).toBe("3");
    expect(tabs.findAll('[role="tab"]')).toHaveLength(1);
    await action("New query", wrapper.element).trigger("keydown", { key: "Delete" });
    expect(tabs.findAll('[role="tab"]')).toHaveLength(1);
    await action("New query", wrapper.element).trigger("click");
    await tabs.get('[role="tab"][aria-selected="false"]').trigger("keydown", { key: "Delete" });
    expect(wrapper.setupState.activeDraft.value).toBe("4");
    await tabs.get('[role="tab"]').trigger("keydown", { key: "Delete" });
    expect(tabs.findAll('[role="tab"]')).toHaveLength(1);
  });
  it("runs a selection, recovers opted-in drafts, and keeps private drafts off disk", async () => {
    ({ wrapper } = await mountPage(Sql));
    const state = wrapper.setupState;
    await action("History & Drafts").trigger("click");
    expect(activeDialog()?.textContent).toContain("No Recent Executions");
    await action("Close", activeDialog()!).trigger("click");
    const sql = "select 3 as selected; select 4 as ignored;";
    state.query.value = sql;
    await nextTick();
    const editorView = EditorView.findFromDOM(wrapper.get(".cm-editor").element as HTMLElement)!;
    editorView.dispatch({ selection: { anchor: 0, head: sql.indexOf(";") + 1 } });
    await action("Run Selection").trigger("click");
    expect(state.pendingQuery.value).toBe("select 3 as selected;");
    await action("Execute SQL", activeDialog()!).trigger("click");
    await settle();
    expect(state.dataControlPlane.result.value?.rows).toEqual([{ selected: 3 }]);
    expect(localStorage.getItem(state.storageKey)).toBeNull();
    const objectUrl = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:test");
    const release = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
    const download = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    await menuAction("Download JSON", wrapper.element);
    expect(download).toHaveBeenCalledOnce();
    await vi.waitFor(() => expect(release).toHaveBeenCalledWith("blob:test"));
    objectUrl.mockRestore();
    release.mockRestore();
    download.mockRestore();
    await action("Inspect", wrapper.element).trigger("click");
    expect(activeDialog()?.textContent).toContain('"selected": 3');
    const resultModal = wrapper
      .findAllComponents(UModal)
      .find((item) => item.props("title") === "Result Row")!;
    resultModal.vm.$emit("update:open", true);
    expect(state.inspectedResult.value).toEqual({ selected: 3 });
    await action("Close", activeDialog()!).trigger("click");
    expect(state.inspectedResult.value).toBeNull();
    await pageDOM().get("tbody tr").trigger("click");
    expect(state.inspectedResult.value).toEqual({ selected: 3 });
    await action("Close", activeDialog()!).trigger("click");
    await action("Executed SQL").trigger("click");
    expect(activeDialog()?.textContent).toContain("select 3 as selected;");
    await action("Close", activeDialog()!).trigger("click");
    await menuAction("Copy JSON", wrapper.element);
    const clipboard = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockResolvedValueOnce()
      .mockRejectedValueOnce(new Error("blocked"));
    await state.dataWorkspace.copy("copied SQL");
    await state.dataWorkspace.copy("blocked SQL");
    expect(clipboard).toHaveBeenCalledTimes(2);
    clipboard.mockRestore();
    await action("History & Drafts").trigger("click");
    await pageDOM().get('[role="switch"]').trigger("click");
    await nextTick();
    expect(localStorage.getItem(state.storageKey)).toContain(sql);
    wrapper.unmount();
    ({ wrapper } = await mountPage(Sql));
    const restored = wrapper.setupState;
    expect(restored.query.value).toBe(sql);
    expect(restored.history.value[0].text).toBe("select 3 as selected;");
    await action("New query").trigger("click");
    expect(restored.query.value).toBe("");
    await wrapper.get('[data-close-query="2"]').trigger("click");
    expect(restored.query.value).toBe(sql);
    await action("History & Drafts").trigger("click");
    const execution = pageDOM().get('[role="dialog"][data-state="open"] tbody tr[tabindex="0"]');
    expect(execution.get("code").text()).toBe("select 3 as selected;");
    await execution.trigger("keydown", { key: "Enter" });
    await flushPromises();
    expect(restored.historyOpen.value).toBe(false);
    expect(restored.query.value).toBe("select 3 as selected;");
    wrapper.findComponent(SqlEditor).vm.$emit("execute");
    await nextTick();
    restored.changeDraft("1");
    expect(restored.confirmOpen.value).toBe(true);
    await action("Cancel", activeDialog()!).trigger("click");
    restored.changeDraft("1");
    restored.closeDraft();
    restored.closeDraft();
    restored.changeDraft("not-a-tab");
    restored.recoverDrafts.value = false;
    await nextTick();
    expect(localStorage.getItem(restored.storageKey)).toBeNull();
    localStorage.setItem(restored.storageKey, JSON.stringify({ drafts: [], history: [] }));
    wrapper.unmount();
    ({ wrapper } = await mountPage(Sql));
    expect(wrapper.setupState.recoverDrafts.value).toBe(false);
    expect(wrapper.setupState.drafts.value).toHaveLength(1);
    localStorage.removeItem(restored.storageKey);
  });
  it("uses the correct plural label for multi-row results", async () => {
    ({ wrapper } = await mountPage(Sql));
    const state = wrapper.setupState;
    state.query.value = "select generate_series(1, 2) as number";
    await state.requestExecution();
    await state.execute();
    await settle();
    expect(pageDOM().text()).toContain("2 rows returned");
  });
  it("uses a verified table deep link and tolerates nullable results", async () => {
    const api = useNuxtApp().$client.value.data.cp;
    const name = `sql_${crypto.randomUUID().replaceAll("-", "")}`;
    await api.tables.create({ schema: "public", name, columns: [] });
    await useNuxtApp().$router.push({ path: "/data/sql", query: { table: `public.${name}` } });
    ({ wrapper } = await mountPage(Sql));
    await vi.waitFor(() => expect(wrapper.setupState.query.value).toContain(`"${name}"`));
    const state = wrapper.setupState;
    await updateDataModels(wrapper, true);
    state.dataControlPlane.result.value = {
      columns: null,
      rows: null,
      rows_affected: 0,
      execution_time_ms: 0,
    };
    await nextTick();
    expect(pageDOM().text()).toContain("The displayed statement returned no rows.");
    expect(pageDOM().text()).toContain("0 rows reported");
    state.recoverDrafts.value = true;
    state.query.value = "select 'saved draft'";
    await nextTick();
    wrapper.unmount();
    ({ wrapper } = await mountPage(Sql));
    const restored = wrapper.setupState;
    await vi.waitFor(() => expect(restored.query.value).toContain(`"${name}"`));
    expect(restored.drafts.value[0].text).toBe("select 'saved draft'");
    restored.recoverDrafts.value = false;
    await nextTick();
    await api.tables.delete("public", name);
    wrapper.unmount();
    useNuxtApp().$auth.state.value = null;
    ({ wrapper } = await mountPage(Sql));
    expect(wrapper.setupState.dataControlPlane.tables.value).toEqual([]);
  });
});
