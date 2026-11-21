<script setup lang="ts">
import { authFieldValue, setAuthField, type AuthField } from "../../utils/auth-forms";
const props = defineProps<{ state: object; fields: AuthField[]; disabledKey?: string }>();
const visibleFields = computed(() =>
  props.fields.filter(
    (field) => !field.when || authFieldValue(props.state, field.when.name) === field.when.value,
  ),
);
</script>
<template>
  <div class="grid gap-5 sm:grid-cols-2">
    <UFormField
      v-for="field in visibleFields"
      :key="field.name"
      :name="field.name"
      :label="field.label"
      :description="field.description"
      :required="field.required"
      :class="{
        'sm:col-span-2':
          field.type === 'textarea' ||
          field.type === 'tags' ||
          field.type === 'multiselect' ||
          field.type === 'switch',
      }"
    >
      <USwitch
        v-if="field.type === 'switch'"
        :model-value="!!authFieldValue(state, field.name)"
        :disabled="field.disabled || disabledKey === field.name"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <UInputNumber
        v-else-if="field.type === 'number'"
        :model-value="authFieldValue(state, field.name) as number"
        :min="field.min"
        :disabled="field.disabled || disabledKey === field.name"
        class="w-full"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <USelect
        v-else-if="field.type === 'select'"
        :disabled="field.disabled || disabledKey === field.name"
        :model-value="(authFieldValue(state, field.name) || '__unconfigured') as string"
        :items="
          field.options!.map((value) => ({
            label: value || field.emptyLabel || 'Unconfigured',
            value: value || '__unconfigured',
          }))
        "
        class="w-full"
        @update:model-value="
          setAuthField(state, field.name, $event === '__unconfigured' ? '' : $event)
        "
      />
      <USelectMenu
        v-else-if="field.type === 'multiselect'"
        :model-value="(authFieldValue(state, field.name) || []) as string[]"
        :items="field.choices"
        value-key="value"
        multiple
        :search-input="false"
        :create-item="false"
        placeholder="Select risk signals"
        :disabled="field.disabled || disabledKey === field.name"
        class="w-full"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <UInputTags
        v-else-if="field.type === 'tags'"
        :disabled="field.disabled || disabledKey === field.name"
        :model-value="(authFieldValue(state, field.name) || []) as string[]"
        :delimiter="/[\n,]/"
        add-on-paste
        add-on-blur
        class="w-full"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <UTextarea
        v-else-if="field.type === 'textarea'"
        :disabled="field.disabled || disabledKey === field.name"
        :model-value="(authFieldValue(state, field.name) || '') as string"
        :rows="4"
        class="w-full"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <UInput
        v-else
        :model-value="(authFieldValue(state, field.name) || '') as string"
        :type="field.type === 'password' ? 'password' : 'text'"
        :autocomplete="field.type === 'password' ? 'new-password' : 'off'"
        :disabled="field.disabled || disabledKey === field.name"
        class="w-full"
        @update:model-value="setAuthField(state, field.name, $event)"
      />
      <template v-if="field.configured" #hint
        ><UBadge
          :label="authFieldValue(state, field.configured) ? 'Secret Configured' : 'Not Configured'"
          color="neutral"
          variant="subtle"
      /></template>
    </UFormField>
  </div>
</template>
