<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { TasksControlPlaneModels } from "layr-client";
const auth = useAuth();
const client = useClient();
const tasksControlPlane = useTasksControlPlane(client.value.tasks.cp);
const selectedExecution = ref<TasksControlPlaneModels.DLQExecution | null>(null);
const confirmingPurge = ref(false);
const showingDetails = ref(false);
const columns: TableColumn<TasksControlPlaneModels.DLQExecution>[] = [
  { accessorKey: "id", header: "Execution ID" },
  { accessorKey: "jobId", header: "Job ID" },
  { accessorKey: "attempts", header: "Attempts" },
  { accessorKey: "maxAttempts", header: "Maximum Attempts" },
  { accessorKey: "lastError", header: "Last Error" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions", header: "" },
];
async function retry() {
  if (!selectedExecution.value) return;
  if (await tasksControlPlane.retryDLQ(selectedExecution.value.id)) selectedExecution.value = null;
}
async function purge() {
  if (!selectedExecution.value) return;
  if (await tasksControlPlane.purgeDLQ(selectedExecution.value.id)) {
    selectedExecution.value = null;
    confirmingPurge.value = false;
  }
}
onMounted(() => {
  if (auth.hasScope("tasks:execution.read")) return tasksControlPlane.listDLQ();
});
</script>

<template>
  <div class="space-y-6" :aria-busy="tasksControlPlane.busy.value">
    <UAlert
      v-if="tasksControlPlane.error.value"
      title="Request Failed"
      :description="tasksControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="tasksControlPlane.notice.value"
      :title="tasksControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="Dead-Letter Queue"
      description="Executions isolated after a permanent failure or exhausted retries. Inspect, retry, or purge each execution using its row actions."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="tasksControlPlane.dlq.value?.data || []"
        :columns="columns"
        :loading="tasksControlPlane.busy.value"
        :get-row-id="(execution) => execution.id"
        :ui="{ td: 'whitespace-normal align-top' }"
        caption="Dead-lettered task executions. Execution details, retry, and purge actions."
        @select="
          (_event, row) => {
            selectedExecution = row.original;
            showingDetails = true;
          }
        "
      >
        <template #jobId-cell="{ row }">{{ row.original.jobId || "Standalone" }}</template>
        <template #lastError-cell="{ row }"
          ><span class="break-all">{{ row.original.lastError || "Not Recorded" }}</span></template
        >
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #actions-cell="{ row }"
          ><div class="flex justify-end">
            <UDropdownMenu
              :items="[
                {
                  label: 'Details',
                  icon: 'i-lucide-info',
                  color: 'neutral' as const,
                  disabled: tasksControlPlane.busy.value,
                  onSelect: () => {
                    selectedExecution = row.original;
                    showingDetails = true;
                  },
                },
                ...(auth.hasScope('tasks:execution.write')
                  ? [
                      {
                        label: 'Retry Execution',
                        icon: 'i-lucide-rotate-ccw',
                        disabled: tasksControlPlane.busy.value,
                        onSelect: () => {
                          showingDetails = false;
                          selectedExecution = row.original;
                          retry();
                        },
                      },
                    ]
                  : []),
                ...(auth.hasScope('tasks:execution.write')
                  ? [
                      { type: 'separator' as const },
                      {
                        label: 'Purge Execution',
                        icon: 'i-lucide-trash-2',
                        color: 'error' as const,
                        disabled: tasksControlPlane.busy.value,
                        onSelect: () => {
                          showingDetails = false;
                          selectedExecution = row.original;
                          confirmingPurge = true;
                        },
                      },
                    ]
                  : []),
              ]"
              :content="{ align: 'end' }"
            >
              <UButton
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                :aria-label="`Actions for ${row.original.id}`"
              />
            </UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            title="No Dead-Lettered Executions"
            description="Failed tasks requiring attention will appear here."
            icon="i-lucide-inbox"
        /></template>
        <template #loading
          ><UEmpty title="Loading Dead-Letter Queue…" loading icon="i-lucide-inbox"
        /></template>
      </UTable>
      <template #footer>
        <UButton
          label="Previous"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value || tasksControlPlane.dlqOffset.value === 0"
          @click="
            tasksControlPlane.listDLQ(tasksControlPlane.dlqOffset.value - tasksControlPlane.limit)
          "
        />
        <UButton
          label="Next"
          color="neutral"
          variant="outline"
          :disabled="
            tasksControlPlane.busy.value ||
            tasksControlPlane.dlqOffset.value + tasksControlPlane.limit >=
              (tasksControlPlane.dlq.value?.total || 0)
          "
          @click="
            tasksControlPlane.listDLQ(tasksControlPlane.dlqOffset.value + tasksControlPlane.limit)
          "
        />
      </template>
    </UCard>
    <UModal
      v-if="selectedExecution"
      :open="!!selectedExecution && showingDetails"
      :dismissible="!tasksControlPlane.busy.value"
      title="Failed Execution Details"
      description="Inspect the payload and most recent error before retrying. Retrying can repeat side effects."
      :ui="{ body: 'space-y-6', content: 'max-w-4xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: tasksControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
            showingDetails = false;
            selectedExecution = null;
          }
        }
      "
      ><template #body
        ><UAlert
          v-if="tasksControlPlane.error.value"
          title="Request Failed"
          :description="tasksControlPlane.error.value"
          color="error"
          role="alert"
        />
        <dl class="space-y-4 text-sm">
          <div>
            <dt class="text-muted">Execution ID</dt>
            <dd class="break-all font-mono">{{ selectedExecution.id }}</dd>
          </div>
          <div>
            <dt class="text-muted">Status</dt>
            <dd><UBadge :label="selectedExecution.status" color="error" variant="subtle" /></dd>
          </div>
          <div>
            <dt class="text-muted">Last Error</dt>
            <dd class="whitespace-pre-wrap break-all">
              {{ selectedExecution.lastError || "Not Recorded" }}
            </dd>
          </div>
          <div>
            <dt class="text-muted">Payload</dt>
            <dd class="whitespace-pre-wrap break-all font-mono">
              {{ JSON.stringify(selectedExecution.payload, null, 2) }}
            </dd>
          </div>
        </dl> </template
      ><template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="selectedExecution = null"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmingPurge"
      title="Purge Failed Execution?"
      description="Permanently delete this dead-lettered execution and its payload. This cannot be undone."
      :dismissible="!tasksControlPlane.busy.value"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: tasksControlPlane.busy.value }"
      ><template v-if="tasksControlPlane.error.value" #body
        ><UAlert
          title="Request Failed"
          :description="tasksControlPlane.error.value"
          color="error"
          role="alert"
      /></template>
      <template #footer>
        <UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value"
          @click="confirmingPurge = false"
        />
        <UButton
          label="Purge Execution"
          color="error"
          :loading="tasksControlPlane.busy.value"
          @click="purge"
        />
      </template>
    </UModal>
  </div>
</template>
