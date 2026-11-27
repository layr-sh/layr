<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import type { DataControlPlaneModels } from "layr-client";
import { displayDataValue, parseDataRecord } from "../../utils/data-forms";
const props = defineProps<{
  table: DataControlPlaneModels.Table;
  tables: DataControlPlaneModels.Table[];
  record: Record<string, unknown> | null;
  busy: boolean;
  error: string;
}>();
const emit = defineEmits<{ save: [record: Record<string, unknown>] }>();
const client = useClient();
const advanced = ref(false);
const json = ref(JSON.stringify(props.record ?? {}, null, 2));
const formError = ref("");
const state = reactive<Record<string, { mode: string; value: string; boolean: boolean }>>({});
const references = reactive<
  Record<
    string,
    {
      options: { label: string; value: string; description?: string }[];
      loading: boolean;
      error: string;
      request: number;
    }
  >
>({});
const columns = computed(() =>
  (props.table.columns ?? []).filter((column) => !props.record || !column.isPrimaryKey),
);
for (const column of columns.value) {
  const value = props.record?.[column.name];
  state[column.name] = {
    mode: props.record
      ? "omit"
      : column.defaultValue
        ? "omit"
        : column.isNullable
          ? "null"
          : "value",
    value:
      value == null
        ? ""
        : typeof value === "object"
          ? JSON.stringify(value, null, 2)
          : String(value),
    boolean: value === true,
  };
}
function foreignKey(name: string) {
  return (props.table.foreignKeys ?? []).find((key) => key.column === name);
}
async function loadReference(name: string, search: string) {
  const key = foreignKey(name)!;
  const status = (references[name] ??= { options: [], loading: false, error: "", request: 0 });
  const request = ++status.request;
  status.loading = true;
  status.error = "";
  const target = props.tables.find(
    (table) => table.schema === key.foreignSchema && table.name === key.foreignTable,
  );
  const label =
    (target?.columns ?? []).find((column) =>
      ["name", "title", "label", "email", "identifier"].includes(column.name),
    )?.name ?? key.foreignColumn;
  try {
    const params: Record<string, string | number> = { limit: 30, order: `${label}.asc` };
    if (search)
      params[label] =
        `${label === key.foreignColumn ? "eq" : "ilike"}.${label === key.foreignColumn ? search : `*${search}*`}`;
    const rows = await client.value.data.records.list(key.foreignSchema, key.foreignTable, params);
    if (request !== status.request) return;
    status.options = rows.map((row) => ({
      label: displayDataValue(row[label]),
      value: String(row[key.foreignColumn]),
      description: `${key.foreignSchema}.${key.foreignTable} · ${displayDataValue(row[key.foreignColumn])}`,
    }));
    if (state[name]!.value && !status.options.some((option) => option.value === state[name]!.value))
      status.options.unshift({ label: state[name]!.value, value: state[name]!.value });
  } catch (err) {
    if (request === status.request)
      status.error = err instanceof Error ? err.message : "Could not load related rows.";
  } finally {
    if (request === status.request) status.loading = false;
  }
}
const searchReferenceDebounceFn = useDebounceFn(loadReference, 200);
function payload() {
  if (advanced.value) return parseDataRecord(json.value);
  const output: Record<string, unknown> = {};
  for (const column of columns.value) {
    const field = state[column.name]!;
    if (field.mode === "omit") continue;
    if (field.mode === "null") {
      output[column.name] = null;
      continue;
    }
    if (column.type === "boolean") output[column.name] = field.boolean;
    else if (
      column.type === "json" ||
      column.type === "jsonb" ||
      column.type.endsWith("[]") ||
      column.type === "ARRAY"
    )
      output[column.name] = JSON.parse(field.value);
    else output[column.name] = field.value;
  }
  if (!Object.keys(output).length) throw new Error("Set at least one column value before saving.");
  return output;
}
function validate(): FormError[] {
  const errors: FormError[] = [];
  if (!advanced.value)
    for (const column of columns.value) {
      const field = state[column.name]!;
      if (
        field.mode === "value" &&
        (column.type === "json" ||
          column.type === "jsonb" ||
          column.type.endsWith("[]") ||
          column.type === "ARRAY")
      ) {
        try {
          JSON.parse(field.value);
        } catch {
          errors.push({ name: column.name, message: "Enter valid JSON." });
        }
      }
    }
  return errors;
}
function save() {
  formError.value = "";
  try {
    emit("save", payload());
  } catch (err) {
    formError.value = (err as Error).message;
  }
}
function toggleAdvanced(mode = advanced.value ? "fields" : "json") {
  const next = mode === "json";
  if (next === advanced.value) return;
  if (next) {
    try {
      json.value = JSON.stringify(payload(), null, 2);
    } catch {
      json.value = "{}";
    }
  }
  advanced.value = next;
}
onMounted(() => {
  for (const column of columns.value)
    if (foreignKey(column.name)) void loadReference(column.name, "");
});
</script>
<template>
  <UForm
    id="data-row-form"
    :state="state"
    :validate="validate"
    :disabled="busy"
    class="space-y-6"
    @submit="save"
  >
    <UAlert
      v-if="error || formError"
      title="Row Not Saved"
      :description="error || formError"
      color="error"
      role="alert"
    />
    <UTabs
      :model-value="advanced ? 'json' : 'fields'"
      :items="[
        { label: 'Column Fields', value: 'fields', disabled: busy },
        { label: 'Raw JSON', value: 'json', disabled: busy },
      ]"
      :content="false"
      @update:model-value="toggleAdvanced(String($event))"
    />
    <UFormField
      v-if="advanced"
      label="Row JSON"
      description="Omit columns to preserve values or use defaults. Use null for SQL NULL. Switching back to column fields discards unsaved JSON edits."
      ><UTextarea v-model="json" :rows="14" spellcheck="false" class="w-full font-mono"
    /></UFormField>
    <template v-else>
      <UFormField
        v-for="column in columns"
        :key="column.name"
        :name="column.name"
        :label="column.name"
        :hint="column.type"
      >
        <div class="space-y-2">
          <USelect
            v-model="state[column.name]!.mode"
            :items="[
              { label: record ? 'Keep existing value' : 'Use default', value: 'omit' },
              { label: 'Set value', value: 'value' },
              ...(column.isNullable ? [{ label: 'NULL', value: 'null' }] : []),
            ]"
            :aria-label="`${column.name} value mode`"
            class="w-full"
          />
          <template v-if="state[column.name]!.mode === 'value'">
            <template v-if="foreignKey(column.name)">
              <USelectMenu
                v-model="state[column.name]!.value"
                :items="references[column.name]?.options ?? []"
                value-key="value"
                description-key="description"
                :loading="references[column.name]?.loading"
                ignore-filter
                :search-input="{ placeholder: 'Search related rows…' }"
                placeholder="Choose a related row…"
                class="w-full"
                @update:search-term="searchReferenceDebounceFn(column.name, $event)"
              />
              <UAlert
                v-if="references[column.name]?.error"
                title="Related Rows Unavailable"
                :description="references[column.name]?.error"
                color="error"
                role="alert"
              />
              <UButton
                v-if="references[column.name]?.error"
                label="Retry Related Rows"
                color="neutral"
                variant="outline"
                @click="searchReferenceDebounceFn(column.name, '')"
              />
            </template>
            <USwitch
              v-else-if="column.type === 'boolean'"
              v-model="state[column.name]!.boolean"
              :label="state[column.name]!.boolean ? 'true' : 'false'"
            />
            <UTextarea
              v-else-if="
                ['json', 'jsonb', 'ARRAY'].includes(column.type) || column.type.endsWith('[]')
              "
              v-model="state[column.name]!.value"
              placeholder="Enter JSON…"
              :rows="4"
              class="w-full font-mono"
            />
            <UInput
              v-else
              v-model="state[column.name]!.value"
              :type="column.type === 'date' ? 'date' : 'text'"
              :placeholder="
                column.type.includes('timestamp') ? '2026-01-01T12:00:00Z' : 'Enter a value…'
              "
              class="w-full"
            />
          </template>
          <p v-else-if="state[column.name]!.mode === 'omit'" class="text-sm text-muted">
            {{
              record
                ? displayDataValue(record[column.name])
                : (column.defaultValue ?? "No default defined")
            }}
          </p>
        </div>
      </UFormField>
    </template>
  </UForm>
</template>
