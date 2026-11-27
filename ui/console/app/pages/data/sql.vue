<script setup lang="ts">
import type { SQLResult } from "../../composables/use-data-control-plane";
import type { TableColumn } from "@nuxt/ui";
import { displayDataValue, quoteDataIdentifier } from "../../utils/data-forms";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const route = useRoute();
const query = ref("select current_database(), current_timestamp;");
const dataWorkspace = useDataWorkspace();
const editorTemplateRef = useTemplateRef<{ getSelection: () => string }>("editor");
const drafts = ref([{ id: "1", text: query.value }]);
const activeDraft = ref("1");
const tabStripTemplateRef = useTemplateRef<HTMLElement>("tabStrip");
const tabsLocked = computed(() => dataControlPlane.busy.value || confirmOpen.value);
const history = ref<{ text: string; time: string }[]>([]);
const recoverDrafts = ref(false);
const historyOpen = ref(false);
const executedSqlOpen = ref(false);
const inspectedResult = ref<Record<string, unknown> | null>(null);
const resultOpen = computed({
  get: () => inspectedResult.value !== null,
  set: (open) => {
    if (!open) inspectedResult.value = null;
  },
});
const results = new Map<string, { result: SQLResult | null; sql: string; source: string }>();
const executedSource = ref("");
const resultRows = computed(() => dataControlPlane.result.value?.rows ?? []);
const storageKey = `layr:data:sql-workspace:${auth.state.value?.user.id ?? ""}`;
let nextDraft = 1;
function changeDraft(id: string) {
  if (tabsLocked.value) return;
  const next = drafts.value.find((draft) => draft.id === id);
  if (!next) return;
  const current = drafts.value.find((draft) => draft.id === activeDraft.value)!;
  current.text = query.value;
  activeDraft.value = id;
  query.value = next.text;
  const prior = results.get(id);
  dataControlPlane.result.value = prior?.result ?? null;
  executedQuery.value = prior?.sql ?? "";
  executedSource.value = prior?.source ?? "";
  dataControlPlane.error.value = "";
}
function addDraft(text = "") {
  const id = String(++nextDraft);
  drafts.value.push({ id, text });
  changeDraft(id);
}
function downloadResults() {
  const objectUrl = URL.createObjectURL(
    new Blob([JSON.stringify(resultRows.value, null, 2)], {
      type: "application/json",
    }),
  );
  const linkElement = document.createElement("a");
  linkElement.href = objectUrl;
  linkElement.download = `layr-query-${activeDraft.value}.json`;
  linkElement.click();
  setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
}
function focusDraft() {
  void nextTick(() =>
    tabStripTemplateRef.value
      ?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
      ?.focus(),
  );
}
function newQueryTab() {
  addDraft();
  focusDraft();
}
function closeDraft(id = activeDraft.value) {
  if (tabsLocked.value || drafts.value.length < 2) return;
  if (id === activeDraft.value) {
    const index = drafts.value.findIndex((draft) => draft.id === id);
    changeDraft((drafts.value[index + 1] ?? drafts.value[index - 1]!).id);
  }
  drafts.value = drafts.value.filter((draft) => draft.id !== id);
  results.delete(id);
  focusDraft();
}
function closeFocusedDraft(event: KeyboardEvent) {
  if (!(event.target instanceof HTMLElement) || event.target.getAttribute("role") !== "tab") return;
  event.preventDefault();
  closeDraft(event.target.querySelector<HTMLElement>("[data-close-query]")?.dataset.closeQuery);
}
watch(
  [recoverDrafts, drafts, query, history],
  () => {
    try {
      if (!recoverDrafts.value) localStorage.removeItem(storageKey);
      else
        localStorage.setItem(
          storageKey,
          JSON.stringify({
            drafts: drafts.value.map((draft) => ({
              ...draft,
              text: draft.id === activeDraft.value ? query.value : draft.text,
            })),
            history: history.value,
          }),
        );
    } catch {
      /* The workspace still works when browser storage is disabled. */
    }
  },
  { deep: true },
);
const confirmOpen = ref(false);
const pendingQuery = ref("");
const executedQuery = ref("");
const columns = computed<TableColumn<Record<string, unknown>>[]>(() =>
  (dataControlPlane.result.value?.columns ?? []).map((name, index) => ({
    id: `result-${index}`,
    header: name,
    accessorFn: (row) => row[name],
    cell: ({ getValue }) => displayDataValue(getValue()),
  })),
);
function requestExecution(selection = false) {
  const sql = selection ? editorTemplateRef.value?.getSelection() : query.value;
  if (dataControlPlane.busy.value || !auth.hasScope("data:schema.write") || !sql?.trim()) return;
  pendingQuery.value = sql;
  confirmOpen.value = true;
}
async function execute() {
  confirmOpen.value = false;
  executedQuery.value = pendingQuery.value;
  executedSource.value = query.value;
  await dataControlPlane.executeSQL(pendingQuery.value);
  results.set(activeDraft.value, {
    result: dataControlPlane.result.value,
    sql: executedQuery.value,
    source: executedSource.value,
  });
  history.value = [
    { text: pendingQuery.value, time: new Date().toISOString() },
    ...history.value,
  ].slice(0, 20);
}
onMounted(async () => {
  try {
    const storedItem = localStorage.getItem(storageKey);
    if (storedItem) {
      const saved = JSON.parse(storedItem) as {
        drafts: { id: string; text: string }[];
        history: typeof history.value;
      };
      if (saved.drafts.length) {
        drafts.value = saved.drafts;
        activeDraft.value = saved.drafts[0]!.id;
        query.value = saved.drafts[0]!.text;
        nextDraft = Math.max(...saved.drafts.map((draft) => Number(draft.id)));
        history.value = saved.history;
        recoverDrafts.value = true;
      }
    }
  } catch {
    /* Ignore unavailable storage or an invalid saved draft. */
  }
  if (!auth.hasScope("data:schema.read")) return;
  await dataControlPlane.loadTables();
  const table = dataControlPlane.tables.value.find(
    (table) => `${table.schema}.${table.name}` === route.query.table,
  );
  if (table) {
    const text = `select * from ${quoteDataIdentifier(table.schema)}.${quoteDataIdentifier(table.name)} limit 100;`;
    if (recoverDrafts.value) addDraft(text);
    else query.value = text;
  }
});
</script>
<template>
  <div class="space-y-6" :aria-busy="dataControlPlane.busy.value">
    <UAlert
      v-if="dataControlPlane.error.value"
      title="Request Failed"
      :description="dataControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UCard
      title="SQL Editor"
      description="Write and run PostgreSQL queries against this database."
      :ui="{ body: 'space-y-5' }"
    >
      <UAlert
        title="Administrative SQL"
        description="Queries run with database privileges and can bypass row-level security or permanently change data. This is not an application-user preview."
        icon="i-lucide-shield-alert"
        color="warning"
      />
      <div class="flex flex-wrap items-center gap-3">
        <div ref="tabStrip" class="min-w-0 grow basis-40 overflow-x-auto">
          <UTabs
            :model-value="activeDraft"
            :items="
              drafts.map(({ id }) => ({ value: id, label: `Query ${id}`, disabled: tabsLocked }))
            "
            activation-mode="manual"
            :content="false"
            class="w-max"
            aria-label="Query tabs. Press Delete on a tab to close it."
            @update:model-value="changeDraft(String($event))"
            @keydown.delete="closeFocusedDraft"
          >
            <template #trailing="{ item }">
              <!-- A UTabs trigger is already a button; Delete handles keyboard closure. -->
              <span
                v-if="drafts.length > 1"
                :data-close-query="item.value"
                :title="`Close ${item.label}`"
                aria-hidden="true"
                class="inline-flex cursor-pointer p-1"
                @mousedown.stop.prevent
                @click.stop="closeDraft(item.value)"
              >
                <UIcon name="i-lucide-x" class="size-3.5" />
              </span>
            </template>
            <template #list-trailing>
              <UButton
                square
                icon="i-lucide-plus"
                aria-label="New query"
                title="New query"
                color="neutral"
                variant="ghost"
                :disabled="tabsLocked"
                @click="newQueryTab"
              />
            </template>
          </UTabs>
        </div>
        <div
          class="ml-auto flex flex-wrap items-center justify-end gap-3"
          role="group"
          aria-label="Query execution"
        >
          <UModal
            v-model:open="historyOpen"
            title="History & Drafts"
            description="Open a recent execution in a new query tab."
            :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
          >
            <UButton
              label="History & Drafts"
              icon="i-lucide-history"
              color="neutral"
              variant="outline"
              :disabled="tabsLocked"
            />
            <template #body>
              <USwitch
                v-model="recoverDrafts"
                label="Remember on this device"
                description="Save drafts and recent SQL in this browser. Turning it off clears this browser’s copies only."
              />
              <UTable
                :data="history"
                :columns="[
                  { accessorKey: 'text', header: 'SQL' },
                  { accessorKey: 'time', header: 'Executed At' },
                ]"
                caption="Recent SQL executions. Select an execution to open it in a new query tab."
                @select="
                  (_event, row) => {
                    addDraft(row.original.text);
                    historyOpen = false;
                  }
                "
              >
                <template #text-cell="{ row }">
                  <code class="whitespace-pre-wrap break-all">{{ row.original.text }}</code>
                </template>
                <template #time-cell="{ row }">{{
                  new Date(row.original.time).toLocaleString()
                }}</template>
                <template #empty>
                  <UEmpty title="No Recent Executions" icon="i-lucide-history" />
                </template>
              </UTable>
            </template>
            <template #footer>
              <UButton
                label="Close"
                color="neutral"
                variant="outline"
                @click="historyOpen = false"
              />
            </template>
          </UModal>
          <UButton
            label="Run Selection"
            color="neutral"
            variant="outline"
            :disabled="dataControlPlane.busy.value || !auth.hasScope('data:schema.write')"
            @click="requestExecution(true)"
          />
          <UButton
            label="Run Query"
            icon="i-lucide-play"
            title="Run the full script (⌘ / Ctrl + Enter in the editor)"
            aria-keyshortcuts="Meta+Enter Control+Enter"
            :loading="dataControlPlane.busy.value"
            :disabled="!query.trim() || !auth.hasScope('data:schema.write')"
            @click="requestExecution()"
          />
        </div>
      </div>
      <div class="h-128 lg:h-144">
        <USplitter
          id="data-sql-workspace"
          orientation="vertical"
          :items="[
            { slot: 'editor', defaultSize: 48, minSize: 30 },
            { slot: 'results', defaultSize: 52, minSize: 35 },
          ]"
          :ui="{ handle: 'flex items-center', panel: 'min-h-0 min-w-0 overflow-hidden' }"
        >
          <template #resize-handle><USeparator /></template>
          <template #editor>
            <DataSqlEditor
              ref="editor"
              v-model="query"
              :tables="dataControlPlane.tables.value"
              :disabled="dataControlPlane.busy.value || !auth.hasScope('data:schema.write')"
              @execute="requestExecution()"
            />
          </template>
          <template #results>
            <section
              aria-labelledby="sql-results-title"
              class="flex h-full w-full min-w-0 flex-col gap-3 pt-3"
            >
              <header class="flex shrink-0 flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 id="sql-results-title" class="font-semibold text-highlighted">Results</h2>
                  <p v-if="dataControlPlane.result.value" class="text-sm text-muted" role="status">
                    First statement:
                    <template v-if="columns.length"
                      >{{ resultRows.length }}
                      {{ resultRows.length === 1 ? "row" : "rows" }} returned</template
                    >
                    <template v-else
                      >{{ dataControlPlane.result.value.rows_affected }} rows reported</template
                    >
                    · {{ dataControlPlane.result.value.execution_time_ms.toFixed(2) }} ms total
                    <span v-if="query !== executedSource">· Query changed since this run</span>
                  </p>
                </div>
                <div v-if="dataControlPlane.result.value" class="flex flex-wrap items-center gap-3">
                  <UButton
                    label="Executed SQL"
                    color="neutral"
                    variant="outline"
                    @click="executedSqlOpen = true"
                  />
                  <UDropdownMenu
                    :items="[
                      {
                        label: 'Download JSON',
                        icon: 'i-lucide-download',
                        onSelect: downloadResults,
                      },
                      {
                        label: 'Copy JSON',
                        icon: 'i-lucide-copy',
                        onSelect: () => dataWorkspace.copy(JSON.stringify(resultRows, null, 2)),
                      },
                    ]"
                    :content="{ align: 'end' }"
                  >
                    <UButton
                      label="Export"
                      trailing-icon="i-lucide-chevron-down"
                      color="neutral"
                      variant="outline"
                    />
                  </UDropdownMenu>
                </div>
              </header>
              <UTable
                v-if="dataControlPlane.result.value"
                :data="resultRows"
                :columns="[...columns, { id: 'inspect' }]"
                caption="Rows returned by the first statement. Select a row to inspect its values."
                sticky
                class="min-h-0 flex-1"
                :ui="{ td: 'max-w-96 truncate' }"
                @select="(_event, row) => (inspectedResult = row.original)"
              >
                <template #inspect-cell="{ row }">
                  <div class="flex justify-end">
                    <UButton
                      label="Inspect"
                      color="neutral"
                      variant="ghost"
                      @click="inspectedResult = row.original"
                    />
                  </div>
                </template>
                <template #empty>
                  <UEmpty
                    title="No Rows Returned"
                    description="SQL committed. The displayed statement returned no rows."
                    icon="i-lucide-table-2"
                  />
                </template>
              </UTable>
              <UEmpty
                v-else
                icon="i-lucide-table-2"
                title="No Results Yet"
                description="Run the full script or highlight SQL and choose Run Selection. Add a LIMIT when exploring large tables."
                class="min-h-0 flex-1 overflow-auto"
              />
            </section>
          </template>
        </USplitter>
      </div>
    </UCard>
    <UModal
      v-model:open="executedSqlOpen"
      title="Executed SQL"
      description="The exact SQL sent for the displayed result. For scripts, only the first statement’s result is shown."
      :ui="{ content: 'max-w-3xl', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{ executedQuery }}</pre>
      </template>
      <template #footer>
        <UButton label="Close" color="neutral" variant="outline" @click="executedSqlOpen = false" />
      </template>
    </UModal>
    <UModal
      v-model:open="resultOpen"
      title="Result Row"
      description="Values returned by the displayed SQL statement."
      :ui="{ content: 'max-w-3xl', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{
          JSON.stringify(inspectedResult, null, 2)
        }}</pre>
      </template>
      <template #footer>
        <UButton label="Close" color="neutral" variant="outline" @click="resultOpen = false" />
      </template>
    </UModal>
    <UModal
      v-model:open="confirmOpen"
      title="Run Administrative SQL?"
      description="Review the SQL before submitting it to PostgreSQL."
      :ui="{ content: 'max-w-3xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          title="Administrative Access"
          description="This SQL can bypass row-level security and modify or delete data. The server runs the script in one transaction and rejects explicit transaction commands such as BEGIN or COMMIT. A rollback does not undo nontransactional effects such as sequence increments. The console cannot undo or cancel a run."
          color="warning"
        />
        <USeparator label="SQL to Execute" position="start" />
        <pre class="max-h-48 overflow-auto whitespace-pre-wrap break-all text-sm">{{
          pendingQuery
        }}</pre>
      </template>
      <template #footer>
        <UButton label="Cancel" color="neutral" variant="outline" @click="confirmOpen = false" />
        <UButton label="Execute SQL" color="warning" @click="execute" />
      </template>
    </UModal>
  </div>
</template>
