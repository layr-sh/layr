<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";
const auth = useAuth();
const client = useClient();
const route = useRoute();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const filterForm = reactive({ endpoint: "all" });
const selectedExecution = ref<FunctionControlPlaneModels.Execution | null>(null);
const endpointOptions = computed(() => [
  { id: "all", name: "All Endpoints" },
  ...(functionControlPlane.endpoints.value?.data || []).map((endpoint) => ({
    id: endpoint.id,
    name: endpoint.name,
  })),
]);
const columns: TableColumn<FunctionControlPlaneModels.Execution>[] = [
  { accessorKey: "endpointId", header: "Endpoint ID" },
  { accessorKey: "method", header: "Method" },
  { accessorKey: "path", header: "Path" },
  { accessorKey: "statusCode", header: "Status" },
  { accessorKey: "durationMs", header: "Duration (ms)" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions", header: "" },
];
async function applyFilter() {
  functionControlPlane.executionEndpointId.value =
    filterForm.endpoint === "all" ? "" : filterForm.endpoint;
  await functionControlPlane.listExecutions();
}
onMounted(async () => {
  if (!auth.hasScope("function:endpoint.read")) return;
  await functionControlPlane.load("endpoints");
  if (typeof route.query.endpoint === "string") filterForm.endpoint = route.query.endpoint;
  await applyFilter();
});
</script>
<template>
  <div class="space-y-6" :aria-busy="functionControlPlane.busy.value">
    <UAlert
      v-if="functionControlPlane.error.value"
      title="Request Failed"
      :description="functionControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UCard
      title="Function Execution History"
      description="Invocation audit records with response status, duration, stdout, and stderr. Open invocation details to inspect its logs."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="function-execution-filters"
        :state="filterForm"
        :disabled="functionControlPlane.busy.value"
        class="mb-5"
        @submit="applyFilter"
      >
        <UFormField label="Endpoint" description="Search for an endpoint or choose all endpoints."
          ><USelectMenu
            v-model="filterForm.endpoint"
            :items="endpointOptions"
            value-key="id"
            label-key="name"
            class="w-full"
        /></UFormField>
      </UForm>
      <UTable
        :data="functionControlPlane.executions.value?.data || []"
        :columns="columns"
        :loading="functionControlPlane.busy.value"
        caption="Function invocations. Open invocation details to view recorded output."
        @select="
          (_event, row) => {
            selectedExecution = row.original;
          }
        "
      >
        <template #statusCode-cell="{ row }"
          ><UBadge
            :label="String(row.original.statusCode)"
            :color="
              row.original.statusCode >= 200 && row.original.statusCode < 400 ? 'success' : 'error'
            "
            variant="subtle"
        /></template>
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #actions-cell="{ row }"
          ><div>
            <UButton
              label="Details"
              color="neutral"
              variant="ghost"
              :disabled="functionControlPlane.busy.value"
              @click="selectedExecution = row.original"
            /></div
        ></template>
        <template #empty
          ><UEmpty
            title="No Function Executions"
            description="Invocation logs appear after an endpoint is called."
            icon="i-lucide-list-checks"
        /></template>
        <template #loading
          ><UEmpty title="Loading Executions…" loading icon="i-lucide-list-checks"
        /></template>
      </UTable>
      <template #footer>
        <UButton
          type="submit"
          form="function-execution-filters"
          label="Apply Filter"
          :disabled="functionControlPlane.busy.value"
        />
        <UButton
          label="Previous"
          color="neutral"
          variant="outline"
          :disabled="
            functionControlPlane.busy.value || functionControlPlane.executionOffset.value === 0
          "
          @click="
            functionControlPlane.listExecutions(
              functionControlPlane.executionOffset.value - functionControlPlane.limit,
            )
          "
        />
        <UButton
          label="Next"
          color="neutral"
          variant="outline"
          :disabled="
            functionControlPlane.busy.value ||
            functionControlPlane.executionOffset.value + functionControlPlane.limit >=
              (functionControlPlane.executions.value?.total || 0)
          "
          @click="
            functionControlPlane.listExecutions(
              functionControlPlane.executionOffset.value + functionControlPlane.limit,
            )
          "
        />
      </template>
    </UCard>
    <UModal
      v-if="selectedExecution"
      :open="!!selectedExecution"
      :dismissible="!functionControlPlane.busy.value"
      title="Invocation Details"
      description="Recorded output from the selected invocation. Output is shown as plain text."
      :ui="{ content: 'max-w-5xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
            selectedExecution = null;
          }
        }
      "
      ><template #body
        ><UAlert
          v-if="functionControlPlane.error.value"
          title="Request Failed"
          :description="functionControlPlane.error.value"
          color="error"
          role="alert"
        />
        <dl class="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-muted">Execution ID</dt>
            <dd class="break-all font-mono">{{ selectedExecution.id }}</dd>
          </div>
          <div>
            <dt class="text-muted">Deployment ID</dt>
            <dd class="break-all font-mono">
              {{ selectedExecution.deploymentId || "Not Recorded" }}
            </dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-muted">Error</dt>
            <dd class="whitespace-pre-wrap break-all">
              {{ selectedExecution.errorMessage || "None" }}
            </dd>
          </div>
        </dl>
        <UFormField label="Standard Output" description="stdout captured for this invocation.">
          <pre class="whitespace-pre-wrap break-all text-sm">{{
            selectedExecution.stdout || "No standard output"
          }}</pre>
        </UFormField>
        <UFormField label="Standard Error" description="stderr captured for this invocation.">
          <pre class="whitespace-pre-wrap break-all text-sm">{{
            selectedExecution.stderr || "No standard error"
          }}</pre>
        </UFormField> </template
      ><template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="selectedExecution = null"
      /></template>
    </UModal>
  </div>
</template>
