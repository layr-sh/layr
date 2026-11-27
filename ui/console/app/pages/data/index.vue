<script setup lang="ts">
import type { DataControlPlaneModels } from "layr-client";
import type { TableColumn } from "@nuxt/ui";
import { displayDataValue, estimatedDataRows, quoteDataIdentifier } from "../../utils/data-forms";
const auth = useAuth();
const client = useClient();
const service = client.value.data.cp;
const dataControlPlane = useDataControlPlane(service);
const route = useRoute();
const router = useRouter();
const dataWorkspace = useDataWorkspace();
const inspected = ref<Record<string, unknown> | null>(null);
const columnSizing = ref<Record<string, number>>({});
const appliedFilter = ref("");
const appliedParams = ref<Record<string, string> | null>(null);
const previousTable = ref("");
const related = ref<
  Record<string, { label: string; table: string; column: string; value: string }>
>({});
watch(inspected, async (row, _previous, onCleanup) => {
  related.value = {};
  if (!row || !table.value) return;
  let stale = false;
  onCleanup(() => {
    stale = true;
  });
  const keys = table.value.foreignKeys ?? [];
  await Promise.all(
    keys
      .filter(
        (key) =>
          row[key.column] != null &&
          keys.filter((part) => part.constraintName === key.constraintName).length === 1,
      )
      .map(async (key) => {
        const parent = dataControlPlane.tables.value.find(
          (entry) => entry.schema === key.foreignSchema && entry.name === key.foreignTable,
        );
        if (!parent) return;
        const value = String(row[key.column]);
        try {
          const [match] = await client.value.data.records.list(parent.schema, parent.name, {
            [key.foreignColumn]: `eq.${value}`,
            limit: 1,
          });
          if (stale || !match) return;
          const label = ["name", "email", "title", "label", "username"]
            .map((name) => match[name])
            .find((candidate) => typeof candidate === "string" && candidate.trim());
          related.value[key.column] = {
            label: label ? String(label) : value,
            table: `${parent.schema}.${parent.name}`,
            column: key.foreignColumn,
            value,
          };
        } catch {
          /* Keep the raw field when a related record cannot be read. */
        }
      }),
  );
});
async function followRelation(relation: { table: string; column: string; value: string }) {
  const origin = selectedKey.value;
  await chooseTable(relation.table);
  if (selectedKey.value !== relation.table) return;
  previousTable.value = origin;
  tab.value = "rows";
  filterColumn.value = relation.column;
  filterOperator.value = "eq";
  filterValue.value = relation.value;
  syncLocation();
  await loadRows();
}
async function returnToPreviousTable() {
  const origin = previousTable.value;
  previousTable.value = "";
  await chooseTable(origin);
}
const search = ref("");
const schemaFilter = ref("All schemas");
const selectedKey = ref("");
const tab = ref(auth.hasScope("data:query.read") ? "rows" : "columns");
const relationshipOpen = ref(false);
const visibleColumns = ref<string[]>([]);
const schemaOpen = ref(false);
const kind = ref<"table" | "column" | "index" | "policy">("table");
const editingColumn = ref<DataControlPlaneModels.Column | null>(null);
const schemas = computed(() => [
  ...new Set([
    ...(dataControlPlane.config.value?.schemas ?? ["public"]),
    ...dataControlPlane.tables.value.map((table) => table.schema),
  ]),
]);
const visibleTables = computed(() =>
  dataControlPlane.tables.value.filter(
    (table) =>
      (schemaFilter.value === "All schemas" || table.schema === schemaFilter.value) &&
      `${table.schema}.${table.name}`.toLowerCase().includes(search.value.toLowerCase()),
  ),
);
const tableOptions = computed(() =>
  (table.value ? dataControlPlane.tables.value : visibleTables.value).map((table) => ({
    label: `${table.schema}.${table.name}`,
    value: `${table.schema}.${table.name}`,
  })),
);
const table = dataControlPlane.selected;
const canWrite = computed(() => auth.hasScope("data:schema.write"));
const rows = ref<Record<string, unknown>[]>([]);
const offset = ref(0);
const pageSize = 25;
const hasNext = ref(false);
const orderColumn = ref("");
const direction = ref("asc");
const filterColumn = ref("");
const filterOperator = ref("eq");
const filterValue = ref("");
const filterType = computed(
  () => table.value?.columns?.find((column) => column.name === filterColumn.value)?.type ?? "",
);
const recordOpen = ref(false);
const editingRecord = ref<Record<string, unknown> | null>(null);
const recordId = ref("");
const deleteOpen = ref(false);
const deleteKind = ref<"table" | "column" | "index" | "policy" | "record" | "rls" | "relationship">(
  "table",
);
const deleteName = ref("");
const confirmation = ref("");
const columnNames = computed(() => (table.value?.columns ?? []).map((column) => column.name));
const rowColumns = computed<TableColumn<Record<string, unknown>>[]>(() => [
  ...columnNames.value
    .filter((name) => visibleColumns.value.includes(name))
    .map<TableColumn<Record<string, unknown>>>((name) => ({
      id: `column-${name}`,
      header: name,
      size: 180,
      minSize: 60,
      accessorKey: name,
      meta: { style: { th: (header) => ({ width: `${header.getSize()}px` }) } },
    })),
  { id: "actions", header: "Actions", size: 80, meta: { style: { th: { width: "80px" } } } },
]);
const rowGridWidth = computed(() =>
  rowColumns.value.reduce(
    (width, column) => width + (columnSizing.value[column.id!] ?? column.size!),
    0,
  ),
);
const tabs = computed(() => [
  ...(auth.hasScope("data:query.read") ? [{ label: "Rows", value: "rows" }] : []),
  { label: "Columns", value: "columns" },
  { label: "Relationships", value: "relationships" },
  { label: "Indexes", value: "indexes" },
  { label: "Policies", value: "policies" },
]);
async function load() {
  if (!auth.hasScope("data:schema.read")) return;
  await dataControlPlane.loadTables();
  if (auth.hasScope("data:config.read")) await dataControlPlane.loadConfig();
  const initial = typeof route.query.table === "string" ? route.query.table : selectedKey.value;
  if (tabs.value.some(({ value }) => value === route.query.tab))
    tab.value = String(route.query.tab);
  if (dataControlPlane.tables.value.some((table) => `${table.schema}.${table.name}` === initial))
    await chooseTable(initial);
}
function syncLocation() {
  if (route.path === "/data")
    void router.replace({
      query: {
        ...route.query,
        table: selectedKey.value || undefined,
        tab: selectedKey.value ? tab.value : undefined,
      },
    });
}
function rememberGrid() {
  if (!selectedKey.value) return;
  dataWorkspace.grids.value[selectedKey.value] = {
    columns: [...visibleColumns.value],
    offset: offset.value,
    orderColumn: orderColumn.value,
    direction: direction.value,
    filterColumn: filterColumn.value,
    filterOperator: filterOperator.value,
    filterValue: filterValue.value,
  };
}
function clearSelection() {
  rememberGrid();
  table.value = null;
  selectedKey.value = "";
  syncLocation();
}
async function chooseTable(key: string) {
  if (dataControlPlane.busy.value) return;
  rememberGrid();
  const match = dataControlPlane.tables.value.find(
    (entry) => `${entry.schema}.${entry.name}` === key,
  );
  if (!match) {
    clearSelection();
    return;
  }
  if (!(await dataControlPlane.selectTable(match.schema, match.name))) return;
  const saved = dataWorkspace.grids.value[key];
  selectedKey.value = key;
  syncLocation();
  inspected.value = null;
  columnSizing.value = {};
  rows.value = [];
  hasNext.value = false;
  appliedFilter.value = "";
  appliedParams.value = null;
  offset.value = saved?.offset ?? 0;
  orderColumn.value = saved?.orderColumn ?? "";
  direction.value = saved?.direction ?? "asc";
  filterColumn.value = saved?.filterColumn ?? "";
  filterOperator.value = saved?.filterOperator ?? "eq";
  filterValue.value = saved?.filterValue ?? "";
  visibleColumns.value = saved?.columns.filter((name) => columnNames.value.includes(name)) ?? [
    ...columnNames.value,
  ];
  if (tab.value === "rows") await loadRows(offset.value);
}
function queryParams(nextOffset: number) {
  const current = table.value!;
  const params: Record<string, string> = {
    limit: String(pageSize + 1),
    offset: String(nextOffset),
  };
  const ordering = orderColumn.value || current.primaryKey;
  if (ordering)
    params.order = `${ordering}.${direction.value}${current.primaryKey && ordering !== current.primaryKey ? `,${current.primaryKey}.asc` : ""}`;
  if (filterColumn.value && filterValue.value)
    params[filterColumn.value] = `${filterOperator.value}.${filterValue.value}`;
  return params;
}
async function loadRows(nextOffset = 0) {
  if (!table.value || !auth.hasScope("data:query.read")) return;
  const current = table.value;
  const params = queryParams(nextOffset);
  await dataControlPlane.run(async () => {
    const result = await client.value.data.records.list(current.schema, current.name, params);
    hasNext.value = result.length > pageSize;
    rows.value = result.slice(0, pageSize);
    offset.value = nextOffset;
    appliedParams.value = params;
    appliedFilter.value = params[filterColumn.value]
      ? `${filterColumn.value} ${filterOperator.value} ${filterValue.value}`
      : "";
    rememberGrid();
  });
}
function changeTab() {
  syncLocation();
  if (tab.value === "rows") return loadRows(offset.value);
}
function openApi() {
  rememberGrid();
  const params = {
    ...(appliedParams.value ?? queryParams(offset.value)),
    limit: String(pageSize),
    select: visibleColumns.value.join(","),
  };
  dataWorkspace.handoff.value = { table: selectedKey.value, params };
  return router.push({ path: "/data/api", query: { tab: "rest", table: selectedKey.value } });
}
function inspectKeyboard(event: KeyboardEvent) {
  const target = event.target;
  if (!(target instanceof HTMLElement) || !["ArrowUp", "ArrowDown"].includes(event.key)) return;
  if (target.tagName !== "TR") return;
  const next =
    event.key === "ArrowDown" ? target.nextElementSibling : target.previousElementSibling;
  if (next instanceof HTMLElement) {
    event.preventDefault();
    next.focus();
  }
}
onBeforeUnmount(rememberGrid);
function openSchema(value: typeof kind.value, column: DataControlPlaneModels.Column | null = null) {
  kind.value = value;
  editingColumn.value = column;
  dataControlPlane.error.value = "";
  schemaOpen.value = true;
}
type SchemaDraft = {
  schema: string;
  name: string;
  type: string;
  nullable: boolean;
  defaultValue: string;
  unique: boolean;
  columns: string[];
  tableColumns?: DataControlPlaneModels.Column[];
  command: string;
  roles: string[];
  permissive: string;
  usingExpression: string;
  withCheckExpression: string;
};
async function saveSchema(draft: SchemaDraft) {
  const schema = kind.value === "table" ? draft.schema : table.value!.schema;
  const name = kind.value === "table" ? draft.name : table.value!.name;
  if (
    kind.value === "table" &&
    dataControlPlane.tables.value.some((table) => table.schema === schema && table.name === name)
  ) {
    dataControlPlane.error.value = "A table with this name already exists in this schema.";
    return;
  }
  if (
    kind.value === "column" &&
    editingColumn.value &&
    draft.name === editingColumn.value.name &&
    draft.type === editingColumn.value.type &&
    draft.nullable === editingColumn.value.isNullable &&
    draft.defaultValue === (editingColumn.value.defaultValue ?? "")
  ) {
    schemaOpen.value = false;
    return;
  }
  const saved = await dataControlPlane.mutate(
    async () => {
      if (kind.value === "table")
        return service.tables.create({
          schema,
          name,
          columns: (draft.tableColumns ?? []).map((column) => ({
            ...column,
            defaultValue: column.defaultValue || undefined,
          })),
        });
      if (kind.value === "column") {
        if (editingColumn.value)
          return service.tables.columns.update(schema, name, editingColumn.value.name, {
            newName: draft.name === editingColumn.value.name ? undefined : draft.name,
            newType: draft.type === editingColumn.value.type ? undefined : draft.type,
            isNullable:
              draft.nullable === editingColumn.value.isNullable ? undefined : draft.nullable,
            defaultValue:
              draft.defaultValue === (editingColumn.value.defaultValue ?? "")
                ? undefined
                : draft.defaultValue,
          });
        return service.tables.columns.create(schema, name, {
          name: draft.name,
          type: draft.type,
          isNullable: draft.nullable,
          defaultValue: draft.defaultValue || undefined,
          isPrimaryKey: false,
          isUnique: draft.unique,
        });
      }
      if (kind.value === "index")
        return service.tables.indexes.create(schema, name, {
          indexName: draft.name,
          columns: draft.columns,
          type: draft.type,
          isUnique: draft.type === "btree" && draft.unique,
        });
      return service.tables.policies.create(schema, name, {
        name: draft.name,
        command: draft.command,
        roles: draft.roles,
        permissive: draft.permissive,
        usingExpression: draft.command === "INSERT" ? undefined : draft.usingExpression,
        withCheckExpression: ["ALL", "INSERT", "UPDATE"].includes(draft.command)
          ? draft.withCheckExpression
          : undefined,
      });
    },
    "Schema Saved",
    schema,
    name,
  );
  if (saved) {
    schemaOpen.value = false;
    const key = `${schema}.${name}`;
    if (selectedKey.value !== key) {
      rememberGrid();
      visibleColumns.value = [...columnNames.value];
      orderColumn.value = "";
      filterColumn.value = "";
      filterValue.value = "";
      appliedParams.value = null;
    }
    selectedKey.value = key;
    syncLocation();
    if (tab.value === "rows") await loadRows();
    dataControlPlane.notice.value = "Schema Saved";
  }
}
function requestDelete(value: typeof deleteKind.value, name: string) {
  deleteKind.value = value;
  deleteName.value = name;
  confirmation.value = "";
  dataControlPlane.error.value = "";
  deleteOpen.value = true;
}
async function confirmDelete() {
  const current = table.value!;
  let saved: boolean;
  if (deleteKind.value === "relationship") {
    saved = await dataControlPlane.mutateSQL(
      `ALTER TABLE ${quoteDataIdentifier(current.schema)}.${quoteDataIdentifier(current.name)} DROP CONSTRAINT ${quoteDataIdentifier(deleteName.value)};`,
      current.schema,
      current.name,
    );
  } else if (deleteKind.value === "record") {
    saved = await dataControlPlane.run(async () => {
      await client.value.data.records.delete(current.schema, current.name, deleteName.value);
    }, "Row Deleted");
    if (saved) await loadRows(offset.value);
  } else {
    saved = await dataControlPlane.mutate(
      async () => {
        if (deleteKind.value === "table")
          return service.tables.delete(current.schema, current.name);
        if (deleteKind.value === "column")
          return service.tables.columns.delete(current.schema, current.name, deleteName.value);
        if (deleteKind.value === "index")
          return service.tables.indexes.delete(current.schema, current.name, deleteName.value);
        if (deleteKind.value === "policy")
          return service.tables.policies.delete(current.schema, current.name, deleteName.value);
        return service.tables.rls.update(current.schema, current.name, {
          action: deleteName.value,
        });
      },
      "Schema Updated",
      current.schema,
      current.name,
    );
  }
  if (saved) {
    deleteOpen.value = false;
    if (!table.value) {
      Reflect.deleteProperty(dataWorkspace.grids.value, selectedKey.value);
      selectedKey.value = "";
      syncLocation();
    }
  }
}
async function enableRLS() {
  const current = table.value!;
  await dataControlPlane.mutate(
    () => service.tables.rls.update(current.schema, current.name, { action: "ENABLE" }),
    "Row-Level Security Enabled",
    current.schema,
    current.name,
  );
}
function editRecord(row?: Record<string, unknown>) {
  recordId.value = row ? String(row[table.value!.primaryKey]) : "";
  editingRecord.value = row ?? null;
  dataControlPlane.error.value = "";
  recordOpen.value = true;
}
async function saveRecord(input: Record<string, unknown>) {
  const current = table.value!;
  const saved = await dataControlPlane.run(async () => {
    if (recordId.value)
      await client.value.data.records.update(current.schema, current.name, recordId.value, input);
    else await client.value.data.records.create(current.schema, current.name, input);
  }, "Row Saved");
  if (saved) {
    recordOpen.value = false;
    await loadRows(offset.value);
  }
}
async function saveRelationship(sql: string) {
  const current = table.value!;
  if (await dataControlPlane.mutateSQL(sql, current.schema, current.name))
    relationshipOpen.value = false;
}
function queryTable() {
  const current = table.value!;
  return `/data/sql?table=${encodeURIComponent(`${current.schema}.${current.name}`)}`;
}
function rowCanEdit(row: Record<string, unknown>) {
  return (
    auth.hasScope("data:query.write") &&
    !!table.value?.primaryKey &&
    !(table.value.indexes ?? []).some(
      (index) =>
        index.isUnique &&
        (index.columns ?? []).length > 1 &&
        index.columns!.includes(table.value!.primaryKey),
    ) &&
    row[table.value.primaryKey] != null
  );
}
onMounted(load);
</script>
<template>
  <div class="space-y-6" :aria-busy="dataControlPlane.busy.value">
    <UAlert
      v-if="
        dataControlPlane.error.value &&
        !schemaOpen &&
        !recordOpen &&
        !deleteOpen &&
        !relationshipOpen
      "
      title="Request Failed"
      :description="dataControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="dataControlPlane.notice.value"
      :title="dataControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      v-if="!table"
      title="Tables"
      description="Browse tables in exposed schemas. Row counts are PostgreSQL estimates."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <div class="grid gap-4 sm:grid-cols-2">
        <UFormField label="Schema">
          <USelectMenu v-model="schemaFilter" :items="['All schemas', ...schemas]" class="w-full" />
        </UFormField>
        <UFormField label="Search Tables">
          <UInput
            v-model="search"
            icon="i-lucide-search"
            placeholder="Search by schema or table name…"
            class="w-full"
          />
        </UFormField>
      </div>
      <UTable
        :data="visibleTables"
        :loading="dataControlPlane.busy.value"
        :inert="dataControlPlane.busy.value"
        caption="Database tables and row-level security settings. Select a table to open it."
        :columns="[
          { id: 'table', header: 'Table' },
          { id: 'rowEstimate', header: 'Estimated Rows' },
          { id: 'size', header: 'Size (bytes)' },
          { id: 'security', header: 'Row-Level Security' },
        ]"
        @select="(_event, row) => chooseTable(`${row.original.schema}.${row.original.name}`)"
      >
        <template #table-cell="{ row }">
          <span class="font-medium text-highlighted"
            >{{ row.original.schema }}.{{ row.original.name }}</span
          >
        </template>
        <template #rowEstimate-cell="{ row }">{{
          estimatedDataRows(row.original.estimatedRowCount)
        }}</template>
        <template #size-cell="{ row }">{{ row.original.sizeBytes.toLocaleString() }}</template>
        <template #security-cell="{ row }"
          ><UBadge :color="row.original.rlsEnabled ? 'success' : 'warning'" variant="subtle">{{
            row.original.rlsEnabled ? "Enabled" : "Disabled"
          }}</UBadge></template
        >
        <template #empty>
          <UEmpty
            title="No Tables Found"
            description="Check the selected schema and search term."
            icon="i-lucide-table-2"
          />
        </template>
        <template #loading>
          <UEmpty title="Loading Tables…" icon="i-lucide-table-2" loading />
        </template>
      </UTable>
      <template #footer>
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="load"
        />
        <UButton
          v-if="canWrite"
          label="Create Table"
          icon="i-lucide-plus"
          :disabled="dataControlPlane.busy.value"
          @click="openSchema('table')"
        />
      </template>
    </UCard>
    <UCard
      v-else
      :title="selectedKey"
      :description="`Estimated rows: ${estimatedDataRows(table.estimatedRowCount)} · Size: ${table.sizeBytes.toLocaleString()} bytes`"
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <div class="flex flex-wrap items-end gap-3">
        <UFormField label="Switch Table" class="min-w-0 grow basis-48">
          <USelectMenu
            :model-value="selectedKey"
            :items="tableOptions"
            value-key="value"
            placeholder="Choose a table…"
            :search-input="{ placeholder: 'Search by schema or table name…' }"
            :disabled="dataControlPlane.busy.value"
            class="w-full"
            @update:model-value="chooseTable"
          />
        </UFormField>
        <UButton
          label="All Tables"
          icon="i-lucide-arrow-left"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="clearSelection"
        />
        <UButton
          v-if="previousTable"
          :label="`Back to ${previousTable}`"
          icon="i-lucide-arrow-left"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="returnToPreviousTable"
        />
      </div>
      <UAlert
        v-if="!table.rlsEnabled"
        title="Row-Level Security Disabled"
        description="Row-level policies do not restrict access to this table until RLS is enabled. Privileged Service Account requests can bypass RLS."
        color="warning"
      />
      <UTabs v-model="tab" :items="tabs" :content="false" @update:model-value="changeTab" />
      <div class="space-y-5">
        <template v-if="tab === 'columns'">
          <UTable
            :data="table.columns ?? []"
            :columns="[
              { accessorKey: 'name', header: 'Name' },
              { accessorKey: 'type', header: 'Type' },
              { id: 'nullable', header: 'Nullable' },
              { id: 'default', header: 'Default' },
              { id: 'actions' },
            ]"
          >
            <template #name-cell="{ row }"
              ><span class="font-mono">{{ row.original.name }}</span
              ><UBadge
                v-if="row.original.isPrimaryKey"
                color="neutral"
                variant="outline"
                class="ml-2"
                >PK</UBadge
              ></template
            >
            <template #nullable-cell="{ row }">{{
              row.original.isNullable ? "Yes" : "No"
            }}</template>
            <template #default-cell="{ row }"
              ><code>{{ row.original.defaultValue ?? "—" }}</code></template
            >
            <template #actions-cell="{ row }">
              <div v-if="canWrite" class="flex justify-end">
                <UDropdownMenu
                  :items="[
                    {
                      label: 'Edit',
                      icon: 'i-lucide-pencil',
                      onSelect: () => openSchema('column', row.original),
                    },
                    {
                      label: 'Delete',
                      icon: 'i-lucide-trash-2',
                      color: 'error',
                      disabled: row.original.isPrimaryKey,
                      onSelect: () => requestDelete('column', row.original.name),
                    },
                  ]"
                  :content="{ align: 'end' }"
                >
                  <UButton
                    square
                    icon="i-lucide-ellipsis"
                    color="neutral"
                    variant="ghost"
                    :aria-label="`Actions for ${row.original.name}`"
                    :disabled="dataControlPlane.busy.value"
                  />
                </UDropdownMenu>
              </div>
            </template>
          </UTable>
        </template>
        <template v-if="tab === 'relationships'">
          <UTable
            :data="table.foreignKeys ?? []"
            :columns="[
              { accessorKey: 'constraintName', header: 'Constraint' },
              { accessorKey: 'column', header: 'Column' },
              { accessorKey: 'foreignSchema', header: 'Target Schema' },
              { accessorKey: 'foreignTable', header: 'Target Table' },
              { accessorKey: 'foreignColumn', header: 'Target Column' },
              { accessorKey: 'onDelete', header: 'On Delete' },
              { accessorKey: 'onUpdate', header: 'On Update' },
              { id: 'actions' },
            ]"
            ><template #actions-cell="{ row }">
              <div v-if="canWrite && row.original.constraintName" class="flex justify-end">
                <UDropdownMenu
                  :items="[
                    {
                      label: 'Delete',
                      icon: 'i-lucide-trash-2',
                      color: 'error',
                      onSelect: () => requestDelete('relationship', row.original.constraintName!),
                    },
                  ]"
                  :content="{ align: 'end' }"
                >
                  <UButton
                    square
                    icon="i-lucide-ellipsis"
                    color="neutral"
                    variant="ghost"
                    :aria-label="`Actions for ${row.original.constraintName}`"
                    :disabled="dataControlPlane.busy.value"
                  />
                </UDropdownMenu>
              </div> </template
            ><template #empty>
              <UEmpty
                title="No Foreign Keys"
                description="Foreign keys link columns to records in another table."
                icon="i-lucide-link"
              /> </template
          ></UTable>
        </template>
        <template v-if="tab === 'indexes'">
          <UTable
            :data="table.indexes ?? []"
            :columns="[
              { accessorKey: 'indexName', header: 'Name' },
              { accessorKey: 'columns', header: 'Columns' },
              { accessorKey: 'type', header: 'Method' },
              { accessorKey: 'isUnique', header: 'Unique' },
              { id: 'actions' },
            ]"
            ><template #actions-cell="{ row }">
              <div v-if="canWrite" class="flex justify-end">
                <UDropdownMenu
                  :items="[
                    {
                      label: 'Delete',
                      icon: 'i-lucide-trash-2',
                      color: 'error',
                      onSelect: () => requestDelete('index', row.original.indexName),
                    },
                  ]"
                  :content="{ align: 'end' }"
                >
                  <UButton
                    square
                    icon="i-lucide-ellipsis"
                    color="neutral"
                    variant="ghost"
                    :aria-label="`Actions for ${row.original.indexName}`"
                    :disabled="dataControlPlane.busy.value"
                  />
                </UDropdownMenu>
              </div> </template
          ></UTable>
        </template>
        <template v-if="tab === 'policies'">
          <UTable
            :data="dataControlPlane.policies.value"
            :columns="[
              { accessorKey: 'name', header: 'Name' },
              { accessorKey: 'command', header: 'Command' },
              { accessorKey: 'roles', header: 'Roles' },
              { accessorKey: 'permissive', header: 'Type' },
              { accessorKey: 'usingExpression', header: 'USING' },
              { accessorKey: 'withCheckExpression', header: 'WITH CHECK' },
              { id: 'actions' },
            ]"
            ><template #actions-cell="{ row }">
              <div v-if="canWrite" class="flex justify-end">
                <UDropdownMenu
                  :items="[
                    {
                      label: 'Delete',
                      icon: 'i-lucide-trash-2',
                      color: 'error',
                      onSelect: () => requestDelete('policy', row.original.name),
                    },
                  ]"
                  :content="{ align: 'end' }"
                >
                  <UButton
                    square
                    icon="i-lucide-ellipsis"
                    color="neutral"
                    variant="ghost"
                    :aria-label="`Actions for ${row.original.name}`"
                    :disabled="dataControlPlane.busy.value"
                  />
                </UDropdownMenu>
              </div> </template
            ><template #empty>
              <UEmpty
                title="No Policies"
                description="When RLS is enabled, access is denied by default unless the database role bypasses it."
                icon="i-lucide-shield"
              /> </template
          ></UTable>
        </template>
        <template v-if="tab === 'rows'">
          <UAlert
            title="Service Account Access"
            description="The Rows tab uses the REST API as the Service Account linked to your console session. With data:query.read, requests use the server’s database privileges and can bypass row-level security. This is not an application-user preview."
            color="warning"
          />
          <div class="grid gap-4 sm:grid-cols-3">
            <UFormField label="Visible Columns">
              <USelectMenu
                v-model="visibleColumns"
                :items="columnNames"
                multiple
                placeholder="Visible columns"
                aria-label="Visible columns"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Sort Column">
              <USelectMenu
                v-model="orderColumn"
                :items="columnNames"
                placeholder="Choose a column…"
                aria-label="Order column"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Sort Direction">
              <USelect
                v-model="direction"
                :items="['asc', 'desc']"
                aria-label="Sort direction"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Filter Column">
              <USelectMenu
                v-model="filterColumn"
                :items="columnNames"
                placeholder="Filter column"
                aria-label="Filter column"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Operator">
              <USelect
                v-model="filterOperator"
                :items="['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'like', 'ilike', 'is']"
                aria-label="Filter operator"
                class="w-full"
              />
            </UFormField>
            <UFormField label="Filter Value">
              <USelect
                v-if="filterOperator === 'is'"
                v-model="filterValue"
                :items="[
                  { label: 'NULL', value: 'null' },
                  { label: 'Not NULL', value: 'not.null' },
                ]"
                aria-label="Filter value"
                class="w-full"
              />
              <USelect
                v-else-if="['boolean', 'bool'].includes(filterType)"
                v-model="filterValue"
                :items="['true', 'false']"
                aria-label="Filter value"
                class="w-full"
              />
              <UInput
                v-else
                v-model="filterValue"
                :type="filterType === 'date' ? 'date' : 'text'"
                placeholder="Filter value…"
                aria-label="Filter value"
                class="w-full"
                @keydown.enter.prevent="loadRows()"
              />
            </UFormField>
          </div>
          <div class="flex flex-wrap justify-end gap-3">
            <UButton
              label="Apply Filter"
              color="neutral"
              variant="outline"
              :disabled="dataControlPlane.busy.value"
              @click="loadRows()"
            />
          </div>
          <div v-if="appliedFilter" class="flex flex-wrap items-center gap-3">
            <UBadge color="neutral" variant="subtle">{{ appliedFilter }}</UBadge>
            <UButton
              label="Clear Filter"
              color="neutral"
              variant="outline"
              @click="
                filterValue = '';
                loadRows();
              "
            />
          </div>
          <div class="grid gap-4" :class="inspected && 'xl:grid-cols-3'">
            <UTable
              v-model:column-sizing="columnSizing"
              :column-sizing-options="{ columnResizeMode: 'onChange' }"
              :data="rows"
              :columns="rowColumns"
              :loading="dataControlPlane.busy.value"
              :inert="dataControlPlane.busy.value"
              caption="Rows on the current page. Select a row to inspect its values."
              sticky
              class="max-h-120 min-w-0"
              :class="inspected && 'xl:col-span-2'"
              :style="{ '--row-grid-width': `${rowGridWidth}px` }"
              :ui="{ base: 'min-w-0 table-fixed w-(--row-grid-width)', td: 'truncate' }"
              @select="(_event, row) => (inspected = row.original)"
              @keydown="inspectKeyboard"
            >
              <template
                v-for="name in visibleColumns"
                :key="name"
                #[`column-${name}-header`]="{ column, table: tableApi }"
              >
                <div class="flex items-center justify-between gap-2">
                  <span class="truncate">{{ name }}</span>
                  <UButton
                    square
                    icon="i-lucide-grip-vertical"
                    :aria-label="`Resize ${name}`"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    class="shrink-0 cursor-col-resize touch-none -mr-6 hover:bg-transparent active:bg-transparent focus:bg-transparent **:data-[slot=leadingIcon]:size-3 **:data-[slot=leadingIcon]:text-muted hover:**:data-[slot=leadingIcon]:text-default **:data-[slot=leadingIcon]:transition-colors"
                    @mousedown="
                      tableApi
                        .getFlatHeaders()
                        .find((header) => header.column.id === column.id)
                        ?.getResizeHandler()($event)
                    "
                    @touchstart="
                      tableApi
                        .getFlatHeaders()
                        .find((header) => header.column.id === column.id)
                        ?.getResizeHandler()($event)
                    "
                    @keydown.left.prevent="
                      columnSizing[`column-${name}`] = Math.max(60, column.getSize() - 20)
                    "
                    @keydown.right.prevent="columnSizing[`column-${name}`] = column.getSize() + 20"
                  />
                </div>
              </template>
              <template
                v-for="name in visibleColumns"
                :key="name"
                #[`column-${name}-cell`]="{ row }"
              >
                {{ displayDataValue(row.original[name]) }}
              </template>
              <template #actions-cell="{ row }">
                <div v-if="rowCanEdit(row.original)" class="flex justify-end">
                  <UDropdownMenu
                    :items="[
                      {
                        label: 'Edit',
                        icon: 'i-lucide-pencil',
                        onSelect: () => editRecord(row.original),
                      },
                      {
                        label: 'Delete',
                        icon: 'i-lucide-trash-2',
                        color: 'error',
                        onSelect: () =>
                          requestDelete('record', String(row.original[table!.primaryKey])),
                      },
                    ]"
                    :content="{ align: 'end' }"
                  >
                    <UButton
                      square
                      icon="i-lucide-ellipsis"
                      color="neutral"
                      variant="ghost"
                      :aria-label="`Actions for ${String(row.original[table.primaryKey])}`"
                      :disabled="dataControlPlane.busy.value"
                    />
                  </UDropdownMenu>
                </div>
              </template>
              <template #empty>
                <UEmpty
                  title="No Rows Returned"
                  description="Adjust your filters or insert a row if you have write access."
                  icon="i-lucide-table-2"
                />
              </template>
              <template #loading>
                <UEmpty title="Loading Rows…" icon="i-lucide-table-2" loading />
              </template>
            </UTable>
            <UCard
              v-if="inspected"
              as="aside"
              aria-label="Row details"
              title="Row Details"
              :description="selectedKey"
              class="min-w-0 self-start"
              :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
            >
              <dl class="max-h-96 divide-y divide-default overflow-auto">
                <div
                  v-for="(value, name) in inspected"
                  :key="name"
                  class="flex items-start justify-between gap-3 py-3"
                >
                  <div class="min-w-0">
                    <dt class="text-sm text-muted">{{ name }}</dt>
                    <dd class="whitespace-pre-wrap break-all font-mono text-sm">
                      <UButton
                        v-if="related[String(name)]"
                        :label="related[String(name)]!.label"
                        :aria-label="`Open related record: ${related[String(name)]!.label}`"
                        icon="i-lucide-arrow-up-right"
                        color="neutral"
                        variant="link"
                        @click="followRelation(related[String(name)]!)"
                      />
                      <span v-else>{{ displayDataValue(value) }}</span>
                    </dd>
                  </div>
                  <UButton
                    square
                    icon="i-lucide-copy"
                    :aria-label="`Copy ${name}`"
                    color="neutral"
                    variant="ghost"
                    @click="dataWorkspace.copy(displayDataValue(value))"
                  />
                </div>
              </dl>
              <template #footer>
                <UButton
                  label="Close"
                  aria-label="Close row details"
                  color="neutral"
                  variant="outline"
                  @click="inspected = null"
                />
                <UButton
                  label="Copy row JSON"
                  color="neutral"
                  variant="outline"
                  @click="dataWorkspace.copy(JSON.stringify(inspected, null, 2))"
                />
                <UButton
                  v-if="inspected && rowCanEdit(inspected)"
                  label="Edit Row"
                  @click="
                    editRecord(inspected);
                    inspected = null;
                  "
                />
              </template>
            </UCard>
          </div>
        </template>
      </div>
      <template #footer>
        <p v-if="tab === 'rows'" class="mr-auto text-sm text-muted">
          {{ rows.length ? offset + 1 : 0 }}–{{ offset + rows.length }} rows
        </p>
        <UBadge
          v-else-if="tab === 'policies' && table.rlsEnabled"
          color="success"
          variant="subtle"
          label="RLS Enabled"
          class="mr-auto"
        />
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="load"
        />
        <template v-if="tab === 'rows'">
          <UButton
            label="Previous"
            color="neutral"
            variant="outline"
            :disabled="offset === 0 || dataControlPlane.busy.value"
            @click="loadRows(Math.max(0, offset - pageSize))"
          /><UButton
            label="Next"
            color="neutral"
            variant="outline"
            :disabled="!hasNext || dataControlPlane.busy.value"
            @click="loadRows(offset + pageSize)"
          />
        </template>
        <UDropdownMenu
          v-if="canWrite || auth.hasScope('data:query.read')"
          :items="[
            [
              ...(canWrite
                ? [{ label: 'Query Table', icon: 'i-lucide-terminal', to: queryTable() }]
                : []),
              ...(auth.hasScope('data:query.read')
                ? [
                    { label: 'Open in API Explorer', icon: 'i-lucide-braces', onSelect: openApi },
                    {
                      label: 'Watch Changes',
                      icon: 'i-lucide-radio',
                      to: { path: '/data/realtime', query: { table: selectedKey } },
                    },
                  ]
                : []),
            ],
            canWrite && tab === 'policies'
              ? [
                  table.rlsEnabled
                    ? {
                        label: 'Disable RLS',
                        icon: 'i-lucide-shield-off',
                        color: 'warning',
                        onSelect: () => requestDelete('rls', 'DISABLE'),
                      }
                    : { label: 'Enable RLS', icon: 'i-lucide-shield-check', onSelect: enableRLS },
                  {
                    label: table.rlsForced ? 'Unforce RLS' : 'Force RLS',
                    icon: 'i-lucide-shield',
                    onSelect: () => requestDelete('rls', table!.rlsForced ? 'UNFORCE' : 'FORCE'),
                  },
                ]
              : [],
            canWrite
              ? [
                  {
                    label: 'Create Table',
                    icon: 'i-lucide-plus',
                    onSelect: () => openSchema('table'),
                  },
                  {
                    label: 'Delete Table',
                    icon: 'i-lucide-trash-2',
                    color: 'error',
                    onSelect: () => requestDelete('table', table!.name),
                  },
                ]
              : [],
          ]"
          :content="{ align: 'end' }"
        >
          <UButton
            square
            icon="i-lucide-ellipsis"
            color="neutral"
            variant="ghost"
            aria-label="Table actions"
            :disabled="dataControlPlane.busy.value"
          />
        </UDropdownMenu>
        <template v-if="canWrite">
          <UButton
            v-if="tab === 'columns'"
            label="Add Column"
            icon="i-lucide-plus"
            :disabled="dataControlPlane.busy.value"
            @click="openSchema('column')"
          />
          <UButton
            v-else-if="tab === 'relationships'"
            label="Add Relationship"
            icon="i-lucide-plus"
            :disabled="dataControlPlane.busy.value"
            @click="
              dataControlPlane.error.value = '';
              relationshipOpen = true;
            "
          />
          <UButton
            v-else-if="tab === 'indexes'"
            label="Create Index"
            icon="i-lucide-plus"
            :disabled="dataControlPlane.busy.value"
            @click="openSchema('index')"
          />
          <UButton
            v-else-if="tab === 'policies'"
            label="Create Policy"
            icon="i-lucide-plus"
            :disabled="dataControlPlane.busy.value"
            @click="openSchema('policy')"
          />
        </template>
        <UButton
          v-if="tab === 'rows' && auth.hasScope('data:query.write')"
          label="Insert Row"
          icon="i-lucide-plus"
          :disabled="dataControlPlane.busy.value"
          @click="editRecord()"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="relationshipOpen"
      title="Add Relationship"
      description="Create a foreign key between existing tables. Review the generated SQL before saving."
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body
        ><DataRelationshipForm
          v-if="relationshipOpen && table"
          :table="table"
          :tables="dataControlPlane.tables.value"
          :busy="dataControlPlane.busy.value"
          :error="dataControlPlane.error.value"
          @save="saveRelationship"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="relationshipOpen = false" /><UButton
          label="Save Relationship"
          type="submit"
          form="data-relationship-form"
          :loading="dataControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      v-model:open="schemaOpen"
      :title="
        kind === 'column' && editingColumn
          ? 'Edit Column'
          : {
              table: 'Create Table',
              column: 'Add Column',
              index: 'Create Index',
              policy: 'Create Policy',
            }[kind]
      "
      description="Review changes before saving. Schema changes apply immediately."
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body
        ><DataSchemaForm
          v-if="schemaOpen"
          :kind="kind"
          :table="table"
          :column="editingColumn"
          :busy="dataControlPlane.busy.value"
          :error="dataControlPlane.error.value"
          :schemas="schemas"
          @save="saveSchema"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="schemaOpen = false" /><UButton
          label="Save"
          type="submit"
          form="data-schema-form"
          :loading="dataControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      v-model:open="deleteOpen"
      :title="deleteKind === 'rls' ? 'Change Row-Level Security?' : `Delete ${deleteKind}?`"
      :description="
        deleteKind === 'rls'
          ? 'Changing RLS changes who can access table rows. FORCE also applies policies to table owners, except roles that bypass RLS.'
          : deleteKind === 'record'
            ? 'This permanently deletes the row. Foreign keys can also delete or update related rows, or block the deletion.'
            : deleteKind === 'table'
              ? 'This permanently drops the table and all its rows. Dependencies can block the operation.'
              : 'This removes the selected database object. Dependencies can block the operation.'
      "
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          v-if="dataControlPlane.error.value"
          title="Request Failed"
          :description="dataControlPlane.error.value"
          color="error"
          role="alert"
        />
        <UFormField :label="`Type ${deleteName} to confirm`">
          <UInput v-model="confirmation" autocomplete="off" class="w-full" />
        </UFormField>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="deleteOpen = false" /><UButton
          label="Confirm"
          color="error"
          :disabled="confirmation !== deleteName"
          :loading="dataControlPlane.busy.value"
          @click="confirmDelete"
      /></template>
    </UModal>
    <UModal
      v-model:open="recordOpen"
      :title="recordId ? 'Edit Row' : 'Insert Row'"
      description="Set column values, use defaults, or choose NULL. Related rows can be selected without entering IDs manually."
      :ui="{ content: 'max-w-3xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
    >
      <template #body>
        <DataRowForm
          v-if="recordOpen && table"
          :table="table"
          :tables="dataControlPlane.tables.value"
          :record="editingRecord"
          :busy="dataControlPlane.busy.value"
          :error="dataControlPlane.error.value"
          @save="saveRecord"
        />
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="recordOpen = false" /><UButton
          label="Save Row"
          :loading="dataControlPlane.busy.value"
          type="submit"
          form="data-row-form"
      /></template>
    </UModal>
  </div>
</template>
