<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { TasksControlPlaneModels } from "layr-client";
import {
  createTaskTarget,
  loadTaskTarget,
  resolveTaskTarget,
  taskSchedules,
  taskTimezones,
  validateTaskCron,
} from "../../utils/task-forms";
const auth = useAuth();
const client = useClient();
const tasksControlPlane = useTasksControlPlane(client.value.tasks.cp);
const selectedJobId = ref("");
const selectedJobName = computed(
  () =>
    tasksControlPlane.jobs.value?.data?.find((job) => job.id === selectedJobId.value)?.name ||
    "No job selected",
);
const editing = ref(false);
const confirmingDelete = ref(false);
const confirmingRun = ref(false);
const schedule = ref("0 0 * * *");
const jobFormTemplateRef = useTemplateRef("jobFormRef");
const jobForm = reactive({
  name: "",
  cronExpression: "0 0 * * *",
  timezone: "UTC",
  targetType: "http",
  target: createTaskTarget(),
  isEnabled: false,
});
const timezoneOptions = computed(() => [...new Set([jobForm.timezone, ...taskTimezones])]);
const scheduleSummary = computed(() => {
  const preset = taskSchedules.find((item) => item.value === jobForm.cronExpression);
  const summary = preset ? preset.summary : `Custom cron: ${jobForm.cronExpression}`;
  return `${summary} in ${jobForm.timezone}. ${jobForm.isEnabled ? "The scheduler will queue future runs." : "Saved paused: no automatic runs until you enable the schedule."} Next run is calculated by the service when saved.`;
});
const columns: TableColumn<TasksControlPlaneModels.GetJobResponse>[] = [
  { id: "select", header: "Select" },
  { accessorKey: "name", header: "Name" },
  { accessorKey: "cronExpression", header: "Schedule" },
  { accessorKey: "timezone", header: "Timezone" },
  { accessorKey: "targetType", header: "Target" },
  { accessorKey: "isEnabled", header: "Status" },
  { accessorKey: "nextRunAt", header: "Next Run" },
];
function setSchedule(value: string) {
  if (value !== "custom") jobForm.cronExpression = value;
}
function validateJob(): FormError[] {
  const errors = resolveTaskTarget(jobForm.target, jobForm.targetType).errors;
  if (!jobForm.name.trim()) errors.push({ name: "name", message: "Enter a job name." });
  const cronError = validateTaskCron(jobForm.cronExpression);
  if (cronError) errors.push({ name: "cronExpression", message: cronError });
  try {
    if (!jobForm.timezone.trim()) throw new Error("empty timezone");
    new Intl.DateTimeFormat("en", { timeZone: jobForm.timezone }).format();
  } catch {
    errors.push({ name: "timezone", message: "Choose a valid IANA timezone." });
  }
  return errors;
}
function newJob() {
  selectedJobId.value = "";
  tasksControlPlane.selectedJob.value = null;
  schedule.value = "0 0 * * *";
  Object.assign(jobForm, {
    name: "",
    cronExpression: "0 0 * * *",
    timezone: "UTC",
    targetType: "http",
    target: createTaskTarget(),
    isEnabled: false,
  });
  editing.value = true;
}
async function editJob() {
  if (!selectedJobId.value) return;
  if (await tasksControlPlane.selectJob(selectedJobId.value)) {
    const job = tasksControlPlane.selectedJob.value!;
    Object.assign(jobForm, {
      name: job.name,
      cronExpression: job.cronExpression,
      timezone: job.timezone,
      targetType: job.targetType,
      target: createTaskTarget(),
      isEnabled: job.isEnabled,
    });
    loadTaskTarget(jobForm.target, job.targetPayload);
    schedule.value = taskSchedules.some((item) => item.value === job.cronExpression)
      ? job.cronExpression
      : "custom";
    editing.value = true;
  }
}
async function saveJob() {
  const errors = validateJob();
  if (errors.length) {
    jobFormTemplateRef.value?.setErrors(errors);
    return;
  }
  const { target, ...fields } = jobForm;
  const payload = resolveTaskTarget(target, jobForm.targetType).payload;
  if (await tasksControlPlane.saveJob({ ...fields, targetPayload: JSON.stringify(payload) })) {
    tasksControlPlane.notice.value += jobForm.isEnabled ? " — Schedule Enabled" : " — Saved Paused";
    editing.value = false;
  }
}
async function deleteJob() {
  if (!selectedJobId.value) return;
  if (await tasksControlPlane.deleteJob(selectedJobId.value)) {
    selectedJobId.value = "";
    confirmingDelete.value = false;
    editing.value = false;
  }
}
async function runJob() {
  if (!selectedJobId.value) return;
  if (await tasksControlPlane.trigger(selectedJobId.value, "")) confirmingRun.value = false;
}
onMounted(() => {
  if (auth.hasScope("tasks:job.read")) return tasksControlPlane.load("jobs");
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
    <UAlert
      v-if="tasksControlPlane.triggered.value"
      title="Execution Queued, Not Yet Completed"
      :description="`Execution ${tasksControlPlane.triggered.value.executionId} is queued. Check Execution History for its outcome. Running a paused job once does not enable its schedule.`"
      color="info"
    />
    <UCard
      title="Scheduled Jobs"
      description="Select a job using its checkbox or row, then use the actions below. New jobs start paused."
      :ui="{ footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UTable
        :data="tasksControlPlane.jobs.value?.data || []"
        :columns="columns"
        :loading="tasksControlPlane.busy.value"
        :inert="editing || tasksControlPlane.busy.value"
        :get-row-id="(job) => job.id"
        :row-selection="{ [selectedJobId]: true }"
        caption="Scheduled jobs. Select one job to manage it."
        @select="(_event, row) => (selectedJobId = row.original.id)"
      >
        <template #select-cell="{ row }"
          ><UCheckbox
            :model-value="selectedJobId === row.original.id"
            :aria-label="`Select ${row.original.name}`"
            @click.stop
            @update:model-value="(checked) => (selectedJobId = checked ? row.original.id : '')"
        /></template>
        <template #name-cell="{ row }"
          ><span class="font-medium text-highlighted">{{ row.original.name }}</span></template
        >
        <template #cronExpression-cell="{ row }"
          ><code>{{ row.original.cronExpression }}</code></template
        >
        <template #isEnabled-cell="{ row }"
          ><UBadge
            :label="row.original.isEnabled ? 'Active' : 'Paused'"
            :color="row.original.isEnabled ? 'success' : 'neutral'"
            variant="subtle"
        /></template>
        <template #nextRunAt-cell="{ row }">{{
          row.original.isEnabled ? row.original.nextRunAt.toLocaleString() : "Paused"
        }}</template>
        <template #empty
          ><UEmpty
            title="No Scheduled Jobs"
            description="Create a job to schedule recurring HTTP or SQL tasks."
            icon="i-lucide-calendar-clock"
        /></template>
        <template #loading
          ><UEmpty title="Loading Jobs…" loading icon="i-lucide-calendar-clock"
        /></template>
      </UTable>
      <template #footer>
        <p class="mr-auto text-sm text-muted" aria-live="polite">{{ selectedJobName }}</p>
        <UButton
          v-if="tasksControlPlane.triggered.value && auth.hasScope('tasks:execution.read')"
          to="/tasks/executions"
          label="Execution History"
          color="neutral"
          variant="outline"
        />
        <UButton
          v-if="auth.hasScope('tasks:job.write')"
          label="Delete Job"
          color="error"
          variant="outline"
          :disabled="tasksControlPlane.busy.value || !selectedJobId"
          @click="confirmingDelete = true"
        />
        <UButton
          v-if="auth.hasScope('tasks:execution.write')"
          label="Run Now"
          icon="i-lucide-play"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value || !selectedJobId"
          @click="confirmingRun = true"
        />
        <UButton
          label="Job Details"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value || !selectedJobId"
          @click="editJob"
        />
        <UButton
          v-if="auth.hasScope('tasks:job.write')"
          label="New Job"
          icon="i-lucide-plus"
          :disabled="tasksControlPlane.busy.value"
          @click="newJob"
        />
      </template>
    </UCard>
    <UCard
      v-if="editing"
      :title="tasksControlPlane.selectedJob.value ? 'Manage Task Job' : 'Create Task Job'"
      description="Choose when this job runs and what it does. Pausing stops future scheduled runs, not executions already queued."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="tasks-job"
        ref="jobFormRef"
        :state="jobForm"
        :validate="validateJob"
        :disabled="tasksControlPlane.busy.value || !auth.hasScope('tasks:job.write')"
        class="space-y-5"
        @submit="saveJob"
      >
        <UFormField
          label="Name"
          name="name"
          description="A unique, descriptive name for this job."
          required
          ><UInput v-model="jobForm.name" placeholder="Sync product catalog" class="w-full"
        /></UFormField>
        <div class="grid items-end gap-4 sm:grid-cols-2">
          <UFormField
            label="Schedule"
            description="Choose a common schedule or enter a custom cron expression."
            required
            ><USelect
              v-model="schedule"
              :items="taskSchedules"
              class="w-full"
              @update:model-value="setSchedule"
          /></UFormField>
          <UFormField
            label="Timezone"
            name="timezone"
            description="Search for an IANA timezone. Scheduled times use this timezone."
            required
            ><USelectMenu
              v-model="jobForm.timezone"
              :items="timezoneOptions"
              class="w-full"
              :search-input="{ placeholder: 'Search timezones…' }"
          /></UFormField>
        </div>
        <UFormField
          v-if="schedule === 'custom'"
          label="Cron Expression"
          name="cronExpression"
          description="Five numeric fields: minute, hour, day of month, month, day of week. Aliases such as @daily are also supported."
          required
          ><UInput
            v-model="jobForm.cronExpression"
            placeholder="0 9 * * 1-5"
            class="w-full font-mono"
        /></UFormField>
        <UAlert title="Schedule Preview" :description="scheduleSummary" color="info" />
        <UFormField
          label="Target Type"
          name="targetType"
          description="HTTP sends a POST request; SQL runs a query against the project database."
          required
          ><USelect
            v-model="jobForm.targetType"
            :items="[
              { label: 'HTTP Webhook', value: 'http' },
              { label: 'SQL Query', value: 'sql' },
            ]"
            class="w-full"
        /></UFormField>
        <TasksTargetFields v-model="jobForm.target" :target-type="jobForm.targetType" />
        <USwitch
          v-model="jobForm.isEnabled"
          label="Enable Schedule"
          description="Leave off to save the job paused. You can still run a paused job manually."
        />
      </UForm>
      <dl v-if="tasksControlPlane.selectedJob.value" class="mt-6 grid gap-4 text-sm sm:grid-cols-2">
        <div>
          <dt class="text-muted">Job ID</dt>
          <dd class="break-all font-mono">{{ tasksControlPlane.selectedJob.value.id }}</dd>
        </div>
        <div>
          <dt class="text-muted">Last Scheduled Run</dt>
          <dd>{{ tasksControlPlane.selectedJob.value.lastRunAt?.toLocaleString() || "Never" }}</dd>
        </div>
        <div>
          <dt class="text-muted">Next Scheduled Run</dt>
          <dd>
            {{
              tasksControlPlane.selectedJob.value.isEnabled
                ? tasksControlPlane.selectedJob.value.nextRunAt.toLocaleString()
                : "Paused — no automatic run scheduled"
            }}
          </dd>
        </div>
      </dl>
      <template #footer>
        <UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value"
          @click="editing = false"
        />
        <UButton
          v-if="auth.hasScope('tasks:job.write')"
          type="submit"
          form="tasks-job"
          :label="tasksControlPlane.selectedJob.value ? 'Save Job' : 'Create Job'"
          :disabled="tasksControlPlane.busy.value"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="confirmingDelete"
      title="Delete Task Job?"
      description="This removes the schedule and cancels its pending executions. Already running executions are not stopped. This cannot be undone."
      :dismissible="!tasksControlPlane.busy.value"
      :ui="{ footer: 'justify-end' }"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Job"
          color="error"
          :loading="tasksControlPlane.busy.value"
          @click="deleteJob"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmingRun"
      title="Run Job Now?"
      :description="`Queue one execution of ${selectedJobName} using its saved settings, not unsaved form changes. This may change data or send an HTTP request. It does not enable a paused schedule.`"
      :dismissible="!tasksControlPlane.busy.value"
      :ui="{ footer: 'justify-end' }"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="tasksControlPlane.busy.value"
          @click="confirmingRun = false" /><UButton
          label="Queue Execution"
          :loading="tasksControlPlane.busy.value"
          @click="runJob"
      /></template>
    </UModal>
  </div>
</template>
