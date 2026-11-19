<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const runtimes = computed(() =>
  Object.entries(functionControlPlane.stats.value?.runtimes || {}).map(([name, health]) => ({
    name,
    health,
  })),
);
onMounted(() => {
  if (auth.hasScope("function:stats.read")) return functionControlPlane.load("statistics");
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
      title="Function Statistics"
      description="Current endpoint count and runtime runner health. This snapshot does not include historical traffic or error-rate trends."
    >
      <dl v-if="functionControlPlane.stats.value" class="space-y-2">
        <dt class="text-sm text-muted">Total Endpoints</dt>
        <dd class="text-3xl font-semibold tabular-nums text-highlighted">
          {{ functionControlPlane.stats.value.totalEndpoints.toLocaleString() }}
        </dd>
      </dl>
      <UEmpty
        v-else
        :title="
          functionControlPlane.busy.value
            ? 'Loading Function Statistics…'
            : 'Statistics Unavailable'
        "
        :loading="functionControlPlane.busy.value"
        icon="i-lucide-chart-no-axes-combined"
      />
    </UCard>
    <UCard
      v-for="runtime in runtimes"
      :key="runtime.name"
      :title="`${runtime.name} Runtime`"
      description="Readiness, active isolates, and memory reported by this runner."
      :ui="{ body: 'space-y-5' }"
    >
      <UBadge
        :label="runtime.health?.available ? 'Available' : 'Unavailable'"
        :color="runtime.health?.available ? 'success' : 'warning'"
        variant="subtle"
      />
      <dl v-if="runtime.health" class="grid gap-6 sm:grid-cols-3">
        <div>
          <dt class="text-sm text-muted">Active Isolates</dt>
          <dd class="text-2xl font-semibold tabular-nums text-highlighted">
            {{ runtime.health.activeIsolates.toLocaleString() }}
          </dd>
        </div>
        <div>
          <dt class="text-sm text-muted">Memory Usage</dt>
          <dd class="text-2xl font-semibold tabular-nums text-highlighted">
            {{ runtime.health.memoryUsageMb.toLocaleString() }}
            <span class="text-sm font-normal text-muted">MB</span>
          </dd>
        </div>
        <div>
          <dt class="text-sm text-muted">Process ID</dt>
          <dd class="text-2xl font-semibold tabular-nums text-highlighted">
            {{ runtime.health.processPid || "Not Reported" }}
          </dd>
        </div>
      </dl>
      <UAlert
        v-if="runtime.health?.errorMessage"
        title="Runner Needs Attention"
        :description="runtime.health.errorMessage"
        color="warning"
      />
    </UCard>
    <UEmpty
      v-if="functionControlPlane.stats.value && !runtimes.length"
      title="No Runtime Health Reported"
      description="The service returned no registered runtime health entries."
      icon="i-lucide-square-terminal"
    />
  </div>
</template>
