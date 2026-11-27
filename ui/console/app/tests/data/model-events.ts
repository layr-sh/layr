import type { Component } from "vue";
import { nextTick } from "vue";
import type { VueWrapper } from "@vue/test-utils";
import {
  UInput,
  UInputNumber,
  UInputTags,
  UModal,
  USelect,
  USelectMenu,
  USwitch,
  UTabs,
  UTextarea,
} from "#components";

export async function updateDataModels(wrapper: VueWrapper, tabs = false) {
  for (const component of [
    UInput,
    UInputNumber,
    UInputTags,
    USelect,
    USelectMenu,
    USwitch,
    UTextarea,
  ]) {
    for (const input of wrapper.findAllComponents(component as Component)) {
      const props = input.props() as Record<string, unknown>;
      if (props.placeholder !== "Choose a table…")
        input.vm.$emit("update:modelValue", props.modelValue);
    }
  }
  if (tabs)
    for (const input of wrapper.findAllComponents(UTabs))
      input.vm.$emit("update:modelValue", (input.props() as Record<string, unknown>).modelValue);
  for (const modal of wrapper.findAllComponents(UModal))
    modal.vm.$emit("update:open", modal.props("open"));
  await nextTick();
}
