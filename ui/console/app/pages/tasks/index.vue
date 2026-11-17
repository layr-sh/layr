<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const tasksControlPlane = useTasksControlPlane(client.value.tasks.cp);
const metrics = computed(() => {
  if (!tasksControlPlane.stats.value) return [];
  const stats = tasksControlPlane.stats.value;
  return [
    { label: "Total Jobs", value: stats.totalJobs, description: "Registered recurring jobs" },
    { label: "Active Jobs", value: stats.activeJobs, description: "Enabled scheduled jobs" },
    { label: "Paused Jobs", value: stats.pausedJobs, description: "Jobs with scheduling disabled" },
    {
      label: "Pending Executions",
      value: stats.pendingExecutions,
      description: "Executions waiting for a worker",
    },
    {
      label: "Running Executions",
      value: stats.runningExecutions,
      description: "Executions currently in progress",
    },
    {
      label: "Completed Executions",
      value: stats.completedExecutions,
      description: "Successfully completed executions",
    },
    {
      label: "Failed Executions",
      value: stats.failedExecutions,
      description: "Executions that failed",
    },
  ];
});
onMounted(() => {
  if (auth.hasScope("tasks:stats.read")) return tasksControlPlane.load("statistics");
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
    <UCard
      title="Tasks Statistics"
      description="A current snapshot of scheduled jobs and execution queue activity."
    >
      <dl v-if="tasksControlPlane.stats.value" class="grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
        <div v-for="metric in metrics" :key="metric.label" class="space-y-2">
          <dt class="text-sm text-muted">{{ metric.label }}</dt>
          <dd class="text-3xl font-semibold tracking-tight tabular-nums text-highlighted">
            {{ metric.value.toLocaleString() }}
          </dd>
          <dd class="text-sm text-muted">{{ metric.description }}</dd>
        </div>
      </dl>
      <UEmpty
        v-else
        :title="
          tasksControlPlane.busy.value ? 'Loading Tasks Statistics…' : 'Statistics Unavailable'
        "
        :loading="tasksControlPlane.busy.value"
        icon="i-lucide-chart-no-axes-combined"
      />
    </UCard>
    <UCard
      v-if="tasksControlPlane.stats.value"
      title="Execution Success Rate"
      description="Completed executions as a share of completed and failed executions. The service reports 100% when none have finished."
      :ui="{ body: 'space-y-4' }"
    >
      <p class="text-3xl font-semibold tabular-nums text-highlighted">
        {{ tasksControlPlane.stats.value.successRate.toFixed(2) }}%
      </p>
      <UProgress
        :model-value="tasksControlPlane.stats.value.successRate"
        :max="100"
        :get-value-label="(value) => `Execution Success Rate: ${value}%`"
      />
    </UCard>
  </div>
</template>
