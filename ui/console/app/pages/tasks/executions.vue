<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import { createTaskTarget, loadTaskTarget, resolveTaskTarget } from "../../utils/task-forms";
import type { TasksControlPlaneModels } from "layr-client";
const auth = useAuth();
const triggerOpen = ref(false);
const client = useClient();
const tasksControlPlane = useTasksControlPlane(client.value.tasks.cp);
const triggerForm = reactive({
  mode: "job",
  jobId: "",
  override: false,
  targetType: "http",
  target: createTaskTarget(),
});
const triggerFormTemplateRef = useTemplateRef("triggerFormRef");
const jobChoices = computed(() => tasksControlPlane.jobs.value?.data || []);
const selectedJob = computed(() => jobChoices.value.find((job) => job.id === triggerForm.jobId));
const needsPayload = computed(() => triggerForm.mode === "standalone" || triggerForm.override);
function chooseJob(id: string) {
  triggerForm.jobId = id;
  const job = jobChoices.value.find((item) => item.id === id);
  if (job) {
    triggerForm.targetType = job.targetType;
    loadTaskTarget(triggerForm.target, job.targetPayload);
  }
}
function validateExecution(): FormError[] {
  const errors: FormError[] = [];
  if (
    triggerForm.mode === "job" &&
    !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(triggerForm.jobId)
  )
    errors.push({ name: "jobId", message: "Choose a job or enter a valid job UUID." });
  if (needsPayload.value)
    errors.push(...resolveTaskTarget(triggerForm.target, triggerForm.targetType).errors);
  return errors;
}
async function queueExecution() {
  const errors = validateExecution();
  if (errors.length) {
    triggerFormTemplateRef.value?.setErrors(errors);
    return;
  }
  const jobId = triggerForm.mode === "job" ? triggerForm.jobId : "";
  const payload = needsPayload.value
    ? JSON.stringify(resolveTaskTarget(triggerForm.target, triggerForm.targetType).payload)
    : "";
  await tasksControlPlane.trigger(jobId, payload);
}
const selectedExecution = ref<TasksControlPlaneModels.ExecutionLog | null>(null);
const columns: TableColumn<TasksControlPlaneModels.ExecutionLog>[] = [
  { accessorKey: "executionId", header: "Execution ID" },
  { accessorKey: "jobId", header: "Job ID" },
  { accessorKey: "status", header: "Status" },
  { accessorKey: "executionDurationMs", header: "Duration (ms)" },
  { accessorKey: "executedAt", header: "Executed" },
  { id: "actions", header: "" },
];
onMounted(async () => {
  if (auth.hasScope("tasks:execution.read")) await tasksControlPlane.listExecutions();
  if (auth.hasScope("tasks:job.read") && auth.hasScope("tasks:execution.write"))
    await tasksControlPlane.load("jobs");
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
    <div v-if="auth.hasScope('tasks:execution.write')" class="flex justify-end">
      <UButton label="New Execution" icon="i-lucide-plus" @click="triggerOpen = true" />
    </div>
    <UModal
      :open="!!triggerOpen"
      :dismissible="!tasksControlPlane.busy.value"
      title="Trigger Execution"
      description="Run a saved job once or create a standalone HTTP or SQL task. Queued means accepted for processing, not completed."
      :ui="{ body: 'space-y-6', content: 'max-w-4xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: tasksControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
            triggerOpen = false;
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
        <UForm
          id="tasks-trigger"
          ref="triggerFormRef"
          :state="triggerForm"
          :validate="validateExecution"
          :disabled="tasksControlPlane.busy.value"
          class="space-y-5"
          @submit="queueExecution"
        >
          <UFormField
            label="Execution Mode"
            name="mode"
            description="Saved jobs use their configured target. Standalone tasks do not create a schedule."
            required
          >
            <USelect
              v-model="triggerForm.mode"
              :items="[
                { label: 'Run a Saved Job', value: 'job' },
                { label: 'Run a Standalone Task', value: 'standalone' },
              ]"
              class="w-full"
            />
          </UFormField>
          <template v-if="triggerForm.mode === 'job'">
            <UFormField
              v-if="auth.hasScope('tasks:job.read')"
              label="Job"
              name="jobId"
              description="Search by job name. Paused jobs can also be run manually without enabling their schedule."
              required
            >
              <USelectMenu
                :model-value="triggerForm.jobId"
                :items="jobChoices"
                value-key="id"
                label-key="name"
                placeholder="Choose a job"
                :search-input="{ placeholder: 'Search jobs…' }"
                class="w-full"
                @update:model-value="chooseJob"
              />
            </UFormField>
            <UFormField
              v-else
              label="Job ID"
              name="jobId"
              description="Your Service Account cannot list jobs. Enter a known job UUID, or choose a standalone task."
              required
              ><UInput v-model="triggerForm.jobId" class="w-full"
            /></UFormField>
            <UAlert
              v-if="selectedJob"
              :title="selectedJob.name"
              :description="`${selectedJob.targetType.toUpperCase()} target · ${selectedJob.isEnabled ? 'Active schedule' : 'Paused schedule'}. This execution uses saved settings unless you explicitly override the payload.`"
              color="info"
            />
            <USwitch
              v-model="triggerForm.override"
              label="Override Saved Payload"
              description="Off by default. When enabled, your payload REPLACES the saved payload for this run; it is not merged. The saved job is not changed."
            />
          </template>
          <template v-if="needsPayload">
            <UFormField
              label="Target Type"
              name="targetType"
              description="HTTP sends a POST request; SQL runs a query. A saved job always uses its existing target type."
              required
            >
              <USelect
                v-model="triggerForm.targetType"
                :disabled="triggerForm.mode === 'job' && !!selectedJob"
                :items="[
                  { label: 'HTTP Webhook', value: 'http' },
                  { label: 'SQL Query', value: 'sql' },
                ]"
                class="w-full"
              />
            </UFormField>
            <TasksTargetFields v-model="triggerForm.target" :target-type="triggerForm.targetType" />
          </template>
        </UForm>
        <dl v-if="tasksControlPlane.triggered.value" class="mt-5 space-y-2 text-sm">
          <div>
            <dt class="text-muted">Last Queued Execution ID</dt>
            <dd class="break-all font-mono">{{ tasksControlPlane.triggered.value.executionId }}</dd>
          </div>
          <div>
            <dt class="text-muted">Queue Status — Check History for the Outcome</dt>
            <dd>{{ tasksControlPlane.triggered.value.status }}</dd>
          </div>
        </dl> </template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value"
          @click="triggerOpen = false" /><UButton
          type="submit"
          form="tasks-trigger"
          label="Queue Execution"
          :disabled="tasksControlPlane.busy.value"
      /></template>
    </UModal>
    <UCard
      title="Execution History"
      description="Attempt-level audit logs, not the live pending queue. Open execution details to inspect the response or error."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="tasks-execution-filters"
        :state="{ jobId: tasksControlPlane.jobFilter.value }"
        :disabled="tasksControlPlane.busy.value"
        class="mb-5"
        @submit="tasksControlPlane.listExecutions()"
      >
        <UFormField
          label="Filter by Job ID"
          description="Leave blank to show execution logs for all jobs and standalone tasks."
          ><UInput v-model="tasksControlPlane.jobFilter.value" class="w-full"
        /></UFormField>
      </UForm>
      <UTable
        :data="tasksControlPlane.executions.value?.data || []"
        :columns="columns"
        :loading="tasksControlPlane.busy.value"
        caption="Task execution attempt history. Open execution details to inspect recorded output."
        @select="
          (_event, row) => {
            selectedExecution = row.original;
          }
        "
      >
        <template #jobId-cell="{ row }">{{ row.original.jobId || "Standalone" }}</template>
        <template #status-cell="{ row }"
          ><UBadge
            :label="row.original.status"
            :color="row.original.status === 'success' ? 'success' : 'error'"
            variant="subtle"
        /></template>
        <template #executedAt-cell="{ row }">{{
          row.original.executedAt.toLocaleString()
        }}</template>
        <template #actions-cell="{ row }"
          ><div>
            <UButton
              label="Details"
              color="neutral"
              variant="ghost"
              :disabled="tasksControlPlane.busy.value"
              @click="selectedExecution = row.original"
            /></div
        ></template>
        <template #empty
          ><UEmpty
            title="No Execution Logs"
            description="Finished attempts appear here after a worker processes a task."
            icon="i-lucide-list-checks"
        /></template>
        <template #loading
          ><UEmpty title="Loading Executions…" loading icon="i-lucide-list-checks"
        /></template>
      </UTable>
      <template #footer>
        <UButton
          type="submit"
          form="tasks-execution-filters"
          label="Apply Filter"
          :disabled="tasksControlPlane.busy.value"
        />
        <UButton
          label="Previous"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value || tasksControlPlane.executionOffset.value === 0"
          @click="
            tasksControlPlane.listExecutions(
              tasksControlPlane.executionOffset.value - tasksControlPlane.limit,
            )
          "
        />
        <UButton
          label="Next"
          color="neutral"
          variant="outline"
          :disabled="
            tasksControlPlane.busy.value ||
            tasksControlPlane.executionOffset.value + tasksControlPlane.limit >=
              (tasksControlPlane.executions.value?.total || 0)
          "
          @click="
            tasksControlPlane.listExecutions(
              tasksControlPlane.executionOffset.value + tasksControlPlane.limit,
            )
          "
        />
      </template>
    </UCard>
    <UModal
      v-if="selectedExecution"
      :open="!!selectedExecution"
      :dismissible="!tasksControlPlane.busy.value"
      title="Execution Details"
      description="The recorded response and error for this execution attempt."
      :ui="{ body: 'space-y-6', content: 'max-w-4xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: tasksControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
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
        <dl class="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-muted">Execution ID</dt>
            <dd class="break-all font-mono">{{ selectedExecution.executionId }}</dd>
          </div>
          <div>
            <dt class="text-muted">Response Status Code</dt>
            <dd>{{ selectedExecution.responseStatusCode ?? "Not Recorded" }}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-muted">Error Message</dt>
            <dd class="whitespace-pre-wrap break-all">
              {{ selectedExecution.errorMessage || "None" }}
            </dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-muted">Response Body</dt>
            <dd class="whitespace-pre-wrap break-all font-mono">
              {{ selectedExecution.responseBody || "Not Recorded" }}
            </dd>
          </div>
        </dl> </template
      ><template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="selectedExecution = null"
      /></template>
    </UModal>
  </div>
</template>
