import { afterEach, describe, expect, it } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { EditorView, runScopeHandlers } from "@codemirror/view";
import type { DataControlPlaneModels } from "layr-client";
import SqlEditor from "../../components/data/SqlEditor.vue";
let wrapper: Awaited<ReturnType<typeof mountSuspended<typeof SqlEditor>>>;
afterEach(() => wrapper?.unmount());
describe("SQL editor", () => {
  it("synchronizes the document, schema completion, read-only state, and keyboard execution", async () => {
    wrapper = await mountSuspended(SqlEditor, {
      props: { modelValue: "select 1;" },
      attachTo: document.body,
    });
    const editorView = EditorView.findFromDOM(wrapper.get(".cm-editor").element as HTMLElement)!;
    expect(editorView.state.doc.toString()).toBe("select 1;");
    editorView.dispatch({ selection: { anchor: 1 } });
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: "select 2;" },
    });
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual(["select 2;"]);
    await wrapper.setProps({ modelValue: "select 2;" });
    await wrapper.setProps({ modelValue: "select 3;" });
    expect(editorView.state.doc.toString()).toBe("select 3;");
    const tables = [
      { schema: "public", name: "one", columns: [{ name: "id" }] },
      { schema: "public", name: "two", columns: null },
    ] as DataControlPlaneModels.Table[];
    await wrapper.setProps({ tables });
    expect(
      runScopeHandlers(
        editorView,
        new KeyboardEvent("keydown", { key: "Enter", ctrlKey: true }),
        "editor",
      ),
    ).toBe(true);
    expect(wrapper.emitted("execute")).toHaveLength(1);
    await wrapper.setProps({ disabled: true });
    expect(editorView.state.readOnly).toBe(true);
    runScopeHandlers(
      editorView,
      new KeyboardEvent("keydown", { key: "Enter", ctrlKey: true }),
      "editor",
    );
    expect(wrapper.emitted("execute")).toHaveLength(1);
    await wrapper.setProps({ disabled: false });
    expect(editorView.state.readOnly).toBe(false);
  });
});
