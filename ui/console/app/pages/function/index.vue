<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";
const auth = useAuth();
const client = useClient();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const runtimes = computed(() =>
  Object.entries(functionControlPlane.stats.value?.runtimes || {}).map(([name, health]) => ({
    name,
    health,
  })),
);
const summary = computed(() => {
  const stats = functionControlPlane.stats.value;
  if (!stats) return [];
  return [
    { label: "Total Endpoints", value: stats.totalEndpoints.toLocaleString() },
    { label: "Public Endpoints", value: stats.publicEndpoints.toLocaleString() },
    { label: "Private Endpoints", value: stats.privateEndpoints.toLocaleString() },
    { label: "Deployed Endpoints", value: stats.deployedEndpoints.toLocaleString() },
    {
      label: "Without a Deployment",
      value: (stats.totalEndpoints - stats.deployedEndpoints).toLocaleString(),
    },
    { label: "Stored Deployments", value: stats.totalDeployments.toLocaleString() },
    {
      label: "Stored Bundle Payload",
      value: `${(stats.bundleSizeBytes / 1048576).toLocaleString(undefined, { maximumFractionDigits: 2 })} MiB`,
    },
    { label: "Custom Domains", value: stats.totalDomains.toLocaleString() },
    { label: "Domain Routes", value: stats.domainRoutes.toLocaleString() },
    { label: "Domains Needing TLS Attention", value: stats.domainsNeedingTls.toLocaleString() },
    { label: "Retained Execution Logs", value: stats.totalExecutions.toLocaleString() },
  ];
});
const traffic = computed(() => {
  const stats = functionControlPlane.stats.value;
  if (!stats) return [];
  return [
    { label: "Invocations", value: stats.executionsLast24Hours.toLocaleString() },
    { label: "Failed Invocations", value: stats.failedExecutionsLast24Hours.toLocaleString() },
    {
      label: "Failure Rate",
      value: stats.executionsLast24Hours
        ? `${((100 * stats.failedExecutionsLast24Hours) / stats.executionsLast24Hours).toLocaleString(undefined, { maximumFractionDigits: 1 })}%`
        : "No Invocations",
    },
    { label: "HTTP 4xx Responses", value: stats.clientErrorsLast24Hours.toLocaleString() },
    { label: "HTTP 5xx Responses", value: stats.serverErrorsLast24Hours.toLocaleString() },
    {
      label: "Average Duration",
      value:
        stats.averageDurationMs == null
          ? "No Invocations"
          : `${stats.averageDurationMs.toLocaleString(undefined, { maximumFractionDigits: 1 })} ms`,
    },
    {
      label: "P95 Duration",
      value:
        stats.p95DurationMs == null
          ? "No Invocations"
          : `${stats.p95DurationMs.toLocaleString(undefined, { maximumFractionDigits: 1 })} ms`,
    },
  ];
});
const endpointColumns: TableColumn<FunctionControlPlaneModels.EndpointUsage>[] = [
  { accessorKey: "name", header: "Endpoint" },
  { accessorKey: "runtime", header: "Runtime" },
  { accessorKey: "isPublic", header: "Access" },
  { accessorKey: "isDeployed", header: "Deployment" },
  { accessorKey: "executions", header: "Invocations" },
  { accessorKey: "failedExecutions", header: "Failures" },
  { accessorKey: "averageDurationMs", header: "Average Duration" },
  { accessorKey: "p95DurationMs", header: "P95 Duration" },
  { accessorKey: "lastExecutedAt", header: "Last Invocation" },
];
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
      description="Current endpoint, deployment, and domain inventory. Execution totals reflect retained logs, not lifetime traffic; deleted endpoints and their logs are excluded. Bundle bytes describe stored payloads, not physical disk usage."
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <dl v-if="functionControlPlane.stats.value" class="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <div v-for="item in summary" :key="item.label">
          <dt class="text-sm text-muted">{{ item.label }}</dt>
          <dd class="text-3xl font-semibold tabular-nums text-highlighted">{{ item.value }}</dd>
        </div>
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
      <template #footer>
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value || !auth.hasScope('function:stats.read')"
          @click="functionControlPlane.load('statistics')"
        />
      </template>
    </UCard>
    <template v-if="functionControlPlane.stats.value">
      <UCard
        title="Last 24 Hours"
        description="A rolling 24-hour snapshot of retained execution logs. Failures include HTTP 4xx/5xx responses or recorded runtime errors. Durations are wall-clock execution times, not CPU time."
        :ui="{ body: 'space-y-6' }"
      >
        <dl class="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="item in traffic" :key="item.label">
            <dt class="text-sm text-muted">{{ item.label }}</dt>
            <dd class="text-3xl font-semibold tabular-nums text-highlighted">{{ item.value }}</dd>
          </div>
        </dl>
        <USeparator label="Observation Window" position="start" />
        <p class="text-sm text-muted">
          {{ functionControlPlane.stats.value.windowStartedAt.toLocaleString() }} –
          {{ functionControlPlane.stats.value.generatedAt.toLocaleString() }}
        </p>
        <p class="text-sm text-muted">
          Last recorded invocation:
          {{
            functionControlPlane.stats.value.lastExecutedAt?.toLocaleString() || "No Invocations"
          }}
        </p>
      </UCard>
      <UCard
        title="Endpoint Activity"
        description="Per-endpoint invocations, failures, and latency for the same rolling 24-hour window. Endpoints without invocations have no latency measurements."
      >
        <UTable
          :data="functionControlPlane.stats.value.endpoints || []"
          :columns="endpointColumns"
          empty="No Function endpoints"
          caption="Function endpoint activity over the last 24 hours."
        >
          <template #isPublic-cell="{ row }"
            ><UBadge
              :label="row.original.isPublic ? 'Public' : 'Private'"
              color="neutral"
              variant="subtle"
          /></template>
          <template #isDeployed-cell="{ row }">{{
            row.original.isDeployed ? "Deployed" : "No Deployment"
          }}</template>
          <template #averageDurationMs-cell="{ row }">{{
            row.original.averageDurationMs == null
              ? "No Invocations"
              : `${row.original.averageDurationMs.toLocaleString(undefined, { maximumFractionDigits: 1 })} ms`
          }}</template>
          <template #p95DurationMs-cell="{ row }">{{
            row.original.p95DurationMs == null
              ? "No Invocations"
              : `${row.original.p95DurationMs.toLocaleString(undefined, { maximumFractionDigits: 1 })} ms`
          }}</template>
          <template #lastExecutedAt-cell="{ row }">{{
            row.original.lastExecutedAt?.toLocaleString() || "No Invocations"
          }}</template>
        </UTable>
      </UCard>
    </template>
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
