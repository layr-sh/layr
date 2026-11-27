<script setup lang="ts">
import { basicSetup } from "codemirror";
import { EditorView, keymap } from "@codemirror/view";
import { Compartment, EditorState, Prec } from "@codemirror/state";
import { PostgreSQL, sql } from "@codemirror/lang-sql";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags } from "@lezer/highlight";
import type { DataControlPlaneModels } from "layr-client";
const props = withDefaults(
  defineProps<{
    modelValue: string;
    tables?: DataControlPlaneModels.Table[];
    disabled?: boolean;
  }>(),
  { tables: () => [], disabled: false },
);
const emit = defineEmits<{ "update:modelValue": [value: string]; execute: [] }>();
const hostTemplateRef = useTemplateRef("host");
const languageCompartment = new Compartment();
const editableCompartment = new Compartment();
let editorView: EditorView;
function getSelection() {
  const { from, to: end } = editorView.state.selection.main;
  return editorView.state.sliceDoc(from, end);
}
defineExpose({ getSelection });
function sqlLanguage() {
  const schema: Record<string, Record<string, string[]>> = {};
  for (const table of props.tables) {
    const namespace = (schema[table.schema] ??= {});
    namespace[table.name] = (table.columns ?? []).map((column) => column.name);
  }
  return sql({ dialect: PostgreSQL, schema, defaultSchema: "public" });
}
onMounted(() => {
  editorView = new EditorView({
    parent: hostTemplateRef.value!,
    doc: props.modelValue,
    extensions: [
      basicSetup,
      languageCompartment.of(sqlLanguage()),
      editableCompartment.of(EditorState.readOnly.of(props.disabled)),
      Prec.highest(
        keymap.of([
          {
            key: "Mod-Enter",
            run: () => {
              if (!props.disabled) emit("execute");
              return true;
            },
          },
        ]),
      ),
      EditorView.contentAttributes.of({
        "aria-label": "SQL query",
        "aria-multiline": "true",
        role: "textbox",
        spellcheck: "false",
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) emit("update:modelValue", update.state.doc.toString());
      }),
      syntaxHighlighting(
        HighlightStyle.define([
          { tag: tags.keyword, color: "var(--ui-primary)", fontWeight: "600" },
          { tag: [tags.string, tags.number, tags.bool], color: "var(--ui-text-highlighted)" },
          { tag: tags.comment, color: "var(--ui-text-muted)", fontStyle: "italic" },
        ]),
      ),
      EditorView.theme({
        "&": { color: "var(--ui-text)", backgroundColor: "var(--ui-bg)", height: "100%" },
        "&.cm-focused": { outline: "2px solid var(--ui-primary)", outlineOffset: "-2px" },
        ".cm-scroller": { overflow: "auto", fontFamily: "var(--font-mono)" },
        ".cm-content": { caretColor: "var(--ui-text-highlighted)" },
        ".cm-cursor": { borderLeftColor: "var(--ui-text-highlighted)" },
        ".cm-gutters": {
          backgroundColor: "var(--ui-bg-muted)",
          color: "var(--ui-text-muted)",
          borderColor: "var(--ui-border)",
        },
        ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "var(--ui-bg-elevated)" },
        ".cm-selectionBackground, &.cm-focused .cm-selectionBackground, .cm-content ::selection": {
          backgroundColor: "var(--ui-bg-accented)",
        },
        ".cm-tooltip, .cm-panels": {
          backgroundColor: "var(--ui-bg-elevated)",
          color: "var(--ui-text)",
          borderColor: "var(--ui-border)",
        },
        ".cm-tooltip-autocomplete ul li[aria-selected]": {
          backgroundColor: "var(--ui-bg-accented)",
          color: "var(--ui-text-highlighted)",
        },
        ".cm-searchMatch, .cm-selectionMatch, .cm-matchingBracket": {
          backgroundColor: "var(--ui-bg-accented)",
        },
      }),
    ],
  });
});
watch(
  () => props.modelValue,
  (value) => {
    if (value !== editorView.state.doc.toString())
      editorView.dispatch({ changes: { from: 0, to: editorView.state.doc.length, insert: value } });
  },
);
watch(
  () => props.tables,
  () => editorView.dispatch({ effects: languageCompartment.reconfigure(sqlLanguage()) }),
  { deep: true },
);
watch(
  () => props.disabled,
  (value) =>
    editorView.dispatch({
      effects: editableCompartment.reconfigure(EditorState.readOnly.of(value)),
    }),
);
onBeforeUnmount(() => editorView.destroy());
</script>
<template>
  <div
    ref="host"
    class="h-full w-full min-w-0 overflow-hidden rounded-md border border-default text-sm"
  />
</template>
