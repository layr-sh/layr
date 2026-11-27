<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import type { DataControlPlaneModels } from "layr-client";
import { identifierErrors, quoteDataIdentifier } from "../../utils/data-forms";
const props = defineProps<{
  table: DataControlPlaneModels.Table;
  tables: DataControlPlaneModels.Table[];
  busy: boolean;
  error: string;
}>();
const emit = defineEmits<{ save: [sql: string] }>();
const state = reactive({
  name: "",
  column: "",
  target: "",
  targetColumn: "",
  onDelete: "NO ACTION",
  onUpdate: "NO ACTION",
});
const targets = computed(() =>
  props.tables.map((table) => ({
    label: `${table.schema}.${table.name}`,
    value: `${table.schema}.${table.name}`,
  })),
);
const target = computed(() =>
  props.tables.find((table) => `${table.schema}.${table.name}` === state.target),
);
const targetColumns = computed(() =>
  (target.value?.indexes ?? [])
    .filter((index) => index.isUnique && index.columns?.length === 1)
    .flatMap((index) => index.columns!),
);
const actions = ["NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT"];
function validate(): FormError[] {
  const errors = identifierErrors(state, ["name"]);
  if (!(props.table.columns ?? []).some((column) => column.name === state.column))
    errors.push({ name: "column", message: "Choose a source column." });
  if (!target.value) errors.push({ name: "target", message: "Choose a referenced table." });
  if (!targetColumns.value.includes(state.targetColumn))
    errors.push({ name: "targetColumn", message: "Choose a unique referenced column." });
  return errors;
}
const sql = computed(() => {
  if (!target.value) return "";
  const quote = quoteDataIdentifier;
  return `ALTER TABLE ${quote(props.table.schema)}.${quote(props.table.name)} ADD CONSTRAINT ${quote(state.name)} FOREIGN KEY (${quote(state.column)}) REFERENCES ${quote(target.value.schema)}.${quote(target.value.name)} (${quote(state.targetColumn)}) ON DELETE ${state.onDelete} ON UPDATE ${state.onUpdate};`;
});
function save() {
  emit("save", sql.value);
}
</script>
<template>
  <UForm
    id="data-relationship-form"
    :state="state"
    :validate="validate"
    :disabled="busy"
    class="space-y-6"
    @submit="save"
  >
    <UAlert
      v-if="error"
      title="Relationship Not Saved"
      :description="error"
      color="error"
      role="alert"
    />
    <UFormField label="Constraint Name" name="name" required
      ><UInput v-model="state.name" placeholder="e.g. orders_customer_id_fkey" class="w-full"
    /></UFormField>
    <UFormField label="Source Column" name="column" required
      ><USelectMenu
        v-model="state.column"
        :items="(table.columns ?? []).map((column) => column.name)"
        placeholder="Choose a column…"
        class="w-full"
    /></UFormField>
    <div class="grid gap-4 sm:grid-cols-2">
      <UFormField
        label="Referenced Table"
        name="target"
        description="Choose the table the foreign key points to."
        required
      >
        <USelectMenu
          v-model="state.target"
          :items="targets"
          value-key="value"
          placeholder="Choose a table…"
          :search-input="{ placeholder: 'Search by schema or table name…' }"
          class="w-full"
          @update:model-value="state.targetColumn = ''"
        />
      </UFormField>
      <UFormField
        label="Referenced Column"
        name="targetColumn"
        required
        description="The referenced column must have a single-column unique index or primary key."
      >
        <USelectMenu
          v-model="state.targetColumn"
          :items="targetColumns"
          :disabled="!target"
          placeholder="Choose a unique column…"
          class="w-full"
        />
      </UFormField>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <UFormField label="On Delete" name="onDelete"
        ><USelect v-model="state.onDelete" :items="actions" class="w-full"
      /></UFormField>
      <UFormField label="On Update" name="onUpdate"
        ><USelect v-model="state.onUpdate" :items="actions" class="w-full"
      /></UFormField>
    </div>
    <UAlert
      title="Existing Rows Are Checked"
      description="Adding a foreign key validates current data. Cascading actions can delete or change related rows in future operations."
      color="warning"
    />
    <section v-if="sql" class="space-y-5">
      <USeparator label="SQL Preview" position="start" />
      <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{ sql }}</pre>
    </section>
  </UForm>
</template>
