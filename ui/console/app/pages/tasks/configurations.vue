<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
const auth = useAuth();
const client = useClient();
const tasksControlPlane = useTasksControlPlane(client.value.tasks.cp);
const fields = [
  {
    key: "concurrencyLimit",
    label: "Worker Concurrency",
    description: "Maximum simultaneous executions, from 1 to 1,000.",
    min: 1,
    max: 1000,
  },
  {
    key: "timeoutSeconds",
    label: "Execution Timeout (Seconds)",
    description: "Time allowed for each attempt, from 1 to 3,600 seconds.",
    min: 1,
    max: 3600,
  },
  {
    key: "pollIntervalMs",
    label: "Poll Interval (Milliseconds)",
    description: "Delay between queue polls, from 100 to 60,000 milliseconds.",
    min: 100,
    max: 60000,
  },
  {
    key: "retryInitialDelaySeconds",
    label: "Initial Retry Delay (Seconds)",
    description: "Starting backoff delay, from 1 to 3,600 seconds.",
    min: 1,
    max: 3600,
  },
  {
    key: "retryMaxDelaySeconds",
    label: "Maximum Retry Delay (Seconds)",
    description: "Backoff ceiling, at least the initial delay and at most 86,400 seconds.",
    min: 1,
    max: 86400,
  },
  {
    key: "retryMaxAttempts",
    label: "Maximum Attempts",
    description:
      "Total attempts before moving an execution to the dead-letter queue, from 1 to 20.",
    min: 1,
    max: 20,
  },
] as const;
const configurationFormTemplateRef = useTemplateRef("configurationFormRef");
function validateConfig(): FormError[] {
  const config = tasksControlPlane.config.value;
  if (!config) return [];
  const errors: FormError[] = [];
  for (const field of fields) {
    const value = config[field.key];
    if (!Number.isInteger(value) || value < field.min || value > field.max)
      errors.push({
        name: field.key,
        message: `Enter a whole number from ${field.min.toLocaleString()} to ${field.max.toLocaleString()}.`,
      });
  }
  if (config.retryMaxDelaySeconds < config.retryInitialDelaySeconds)
    errors.push({
      name: "retryMaxDelaySeconds",
      message: "Maximum retry delay must be at least the initial retry delay.",
    });
  return errors;
}
async function save() {
  if (!tasksControlPlane.config.value) return;
  const errors = validateConfig();
  if (errors.length) {
    configurationFormTemplateRef.value?.setErrors(errors);
    return;
  }
  await tasksControlPlane.saveConfig(tasksControlPlane.config.value);
}
onMounted(() => {
  if (auth.hasScope("tasks:config.read")) return tasksControlPlane.load("configuration");
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
      title="Tasks Configuration"
      description="Configure worker concurrency, execution timeouts, polling, and retry backoff."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UForm
        v-if="tasksControlPlane.config.value"
        id="tasks-configuration"
        ref="configurationFormRef"
        :validate="validateConfig"
        :state="tasksControlPlane.config.value"
        :disabled="tasksControlPlane.busy.value || !auth.hasScope('tasks:config.write')"
        class="grid items-end gap-4 sm:grid-cols-2"
        @submit="save"
      >
        <UFormField
          v-for="field in fields"
          :key="field.key"
          :name="field.key"
          :label="field.label"
          :description="field.description"
          required
        >
          <UInputNumber
            v-model="tasksControlPlane.config.value[field.key]"
            :min="field.min"
            :max="field.max"
            required
            class="w-full"
          />
        </UFormField>
      </UForm>
      <UEmpty
        v-else
        :title="
          tasksControlPlane.busy.value
            ? 'Loading Tasks Configuration…'
            : 'Configuration Unavailable'
        "
        :loading="tasksControlPlane.busy.value"
        icon="i-lucide-settings"
      />
      <template v-if="auth.hasScope('tasks:config.write')" #footer>
        <UButton
          type="submit"
          form="tasks-configuration"
          label="Save Tasks Config"
          :disabled="tasksControlPlane.busy.value || !tasksControlPlane.config.value"
        />
      </template>
    </UCard>
  </div>
</template>
