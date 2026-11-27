<script setup lang="ts">
import type { DataControlPlaneModels } from "layr-client";
import type { FormError } from "@nuxt/ui";
import { dataTypes, identifierErrors } from "../../utils/data-forms";
const props = defineProps<{
  kind: "table" | "column" | "index" | "policy";
  table: DataControlPlaneModels.Table | null;
  column: DataControlPlaneModels.Column | null;
  busy: boolean;
  error: string;
  schemas: string[];
}>();
const emit = defineEmits<{
  save: [
    value: {
      schema: string;
      name: string;
      type: string;
      nullable: boolean;
      defaultValue: string;
      unique: boolean;
      columns: string[];
      tableColumns: DataControlPlaneModels.Column[];
      command: string;
      roles: string[];
      permissive: string;
      usingExpression: string;
      withCheckExpression: string;
    },
  ];
  cancel: [];
}>();
const state = reactive({
  schema: props.table?.schema ?? props.schemas[0] ?? "public",
  name: props.column?.name ?? "",
  type: props.column?.type ?? (props.kind === "index" ? "btree" : "text"),
  nullable: props.column?.isNullable ?? true,
  defaultValue: props.column?.defaultValue ?? "",
  unique: false,
  columns: [] as string[],
  tableColumns: [] as DataControlPlaneModels.Column[],
  command: "SELECT",
  roles: ["PUBLIC"],
  permissive: "PERMISSIVE",
  usingExpression: "",
  withCheckExpression: "",
});
function validate(): FormError[] {
  const errors = identifierErrors(state, props.kind === "table" ? ["schema", "name"] : ["name"]);
  if (props.kind === "table") {
    const names = new Set(["id"]);
    state.tableColumns.forEach((column, index) => {
      const nameErrors = identifierErrors(column as unknown as Record<string, unknown>, ["name"]);
      if (nameErrors.length || names.has(column.name))
        errors.push({
          name: `tableColumns.${index}.name`,
          message: names.has(column.name)
            ? "Column names must be unique. id is already the primary key."
            : nameErrors[0]!.message,
        });
      names.add(column.name);
    });
  }
  if (props.kind === "index" && !state.columns.length)
    errors.push({ name: "columns", message: "Choose at least one column." });
  if (props.kind === "policy") {
    if (!state.roles.length)
      errors.push({ name: "roles", message: "Choose at least one database role." });
    for (const name of ["usingExpression", "withCheckExpression"] as const)
      if (/[;]|--|\/\*/.test(state[name]))
        errors.push({ name, message: "Enter a single expression without comments or semicolons." });
  }
  return errors;
}
function save() {
  emit("save", { ...state });
}
function addTableColumn() {
  state.tableColumns.push({
    name: "",
    type: "text",
    isNullable: true,
    isPrimaryKey: false,
    isUnique: false,
  });
}
</script>
<template>
  <UForm
    id="data-schema-form"
    :state="state"
    :validate="validate"
    :disabled="busy"
    class="space-y-6"
    @submit="save"
  >
    <UAlert v-if="error" title="Request Failed" :description="error" color="error" role="alert" />
    <UFormField
      v-if="kind === 'table'"
      label="Schema"
      name="schema"
      required
      description="Choose an existing PostgreSQL schema. This does not create a schema."
    >
      <USelectMenu
        v-model="state.schema"
        :items="schemas"
        placeholder="Choose schema"
        class="w-full"
      />
    </UFormField>
    <UFormField
      label="Name"
      name="name"
      :description="
        kind === 'table'
          ? 'An id column is created as UUID PRIMARY KEY with a uuidv7() default.'
          : undefined
      "
      required
      ><UInput
        v-model="state.name"
        placeholder="Use letters, numbers, and underscores"
        class="w-full"
    /></UFormField>

    <template v-if="kind === 'table'">
      <USeparator label="Columns" position="start" />
      <section v-for="(draftColumn, index) in state.tableColumns" :key="index" class="space-y-5">
        <USeparator :label="`Column ${index + 1}`" position="start" />
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Column Name" :name="`tableColumns.${index}.name`" required
            ><UInput v-model="draftColumn.name" placeholder="e.g. title" class="w-full"
          /></UFormField>
          <UFormField label="Column Type" :name="`tableColumns.${index}.type`"
            ><USelectMenu v-model="draftColumn.type" :items="dataTypes" class="w-full"
          /></UFormField>
        </div>
        <UFormField label="Default Expression" :name="`tableColumns.${index}.defaultValue`"
          ><UInput
            v-model="draftColumn.defaultValue"
            placeholder="Optional SQL expression"
            class="w-full font-mono"
        /></UFormField>
        <div class="flex flex-wrap items-center gap-4">
          <USwitch v-model="draftColumn.isNullable" label="Allow NULL" /><USwitch
            v-model="draftColumn.isUnique"
            label="Unique"
          />
          <UButton
            label="Remove Column"
            icon="i-lucide-x"
            color="neutral"
            variant="outline"
            class="ml-auto"
            @click="state.tableColumns.splice(index, 1)"
          />
        </div>
      </section>
      <UButton
        label="Add Column"
        icon="i-lucide-plus"
        color="neutral"
        variant="outline"
        @click="addTableColumn"
      />
    </template>
    <template v-if="kind === 'column'">
      <UFormField label="Type" name="type" required
        ><USelectMenu
          v-model="state.type"
          :items="[...new Set([...dataTypes, state.type])]"
          class="w-full"
      /></UFormField>
      <UFormField
        label="Default Expression"
        name="defaultValue"
        description="Leave empty for no default. Values are SQL expressions, not JSON."
        ><UInput
          v-model="state.defaultValue"
          placeholder="e.g. now() or 'pending'"
          class="w-full font-mono"
      /></UFormField>
      <USwitch
        v-model="state.nullable"
        label="Allow NULL values"
        :disabled="column?.isPrimaryKey"
      />
      <USwitch v-if="!column" v-model="state.unique" label="Unique values" />
      <UAlert
        v-if="column"
        title="Schema Change"
        description="Type conversions and constraints apply to existing rows. Review compatibility before saving."
        color="warning"
      />
    </template>
    <template v-if="kind === 'index'">
      <UFormField label="Columns" name="columns" required
        ><USelectMenu
          v-model="state.columns"
          :items="(table?.columns ?? []).map((column) => column.name)"
          multiple
          placeholder="Choose columns in index order"
          class="w-full"
      /></UFormField>
      <UFormField label="Index Method" name="type"
        ><USelect
          v-model="state.type"
          :items="['btree', 'gin', 'gist', 'brin', 'hash']"
          class="w-full"
      /></UFormField>
      <USwitch v-model="state.unique" label="Unique index" :disabled="state.type !== 'btree'" />
    </template>
    <template v-if="kind === 'policy'">
      <UFormField label="Command" name="command"
        ><USelect
          v-model="state.command"
          :items="['ALL', 'SELECT', 'INSERT', 'UPDATE', 'DELETE']"
          class="w-full"
      /></UFormField>
      <UFormField
        label="Database Roles"
        name="roles"
        required
        description="PostgreSQL roles, not application roles. PUBLIC applies to every database role."
        ><UInputTags v-model="state.roles" placeholder="Add a database role" class="w-full"
      /></UFormField>
      <UFormField label="Policy Type" name="permissive"
        ><USelect v-model="state.permissive" :items="['PERMISSIVE', 'RESTRICTIVE']" class="w-full"
      /></UFormField>
      <UFormField
        v-if="state.command !== 'INSERT'"
        label="USING Expression"
        name="usingExpression"
        description="Controls which existing rows are visible or eligible for changes."
        ><UTextarea
          v-model="state.usingExpression"
          placeholder="Enter a PostgreSQL boolean expression"
          class="w-full font-mono"
      /></UFormField>
      <UFormField
        v-if="['ALL', 'INSERT', 'UPDATE'].includes(state.command)"
        label="WITH CHECK Expression"
        name="withCheckExpression"
        description="Controls which new or updated rows are permitted."
        ><UTextarea
          v-model="state.withCheckExpression"
          placeholder="Enter a PostgreSQL boolean expression"
          class="w-full font-mono"
      /></UFormField>
    </template>
  </UForm>
</template>
