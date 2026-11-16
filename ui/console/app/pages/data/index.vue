<script setup lang="ts">
import type { DataControlPlaneModels } from "layr-client";

const client = useClient();

const defaultTables = [
  { name: "core.service_accounts", type: "system table", rows: "Active" },
  { name: "core.events", type: "system table", rows: "Active" },
  { name: "core.event_hooks", type: "system table", rows: "Active" },
  { name: "console.users", type: "control plane table", rows: "Active" },
  { name: "console.sessions", type: "control plane table", rows: "Active" },
];

const { data: tablesData } = await useAsyncData("data:tables", () =>
  client.value.data.cp.tables.list().catch(() => null),
);

const tables = computed(() => {
  const fetched = tablesData.value?.data;
  if (fetched && fetched.length > 0) {
    return fetched.map((item: DataControlPlaneModels.Table) => ({
      name: `${item.schema}.${item.name}`,
      type: item.schema === "console" ? "control plane table" : "table",
      rows: "Active",
    }));
  }
  return defaultTables;
});
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Data</h1>
        <p class="text-sm text-muted mt-1">
          Schema browser, SQL query runner, and table inspector.
        </p>
      </div>

      <div class="flex items-center gap-3">
        <UButton label="New Query" icon="i-lucide-play" color="neutral" variant="solid" size="sm" />
      </div>
    </div>

    <!-- Table Explorer -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-table" class="size-4 text-muted" />
          <span class="text-sm font-semibold text-highlighted">Tables</span>
        </div>
        <span class="text-xs text-muted font-mono">{{ tables.length }} tables</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="table in tables"
          :key="table.name"
          class="px-4 py-3.5 flex items-center justify-between hover:bg-accented/30 transition text-sm"
        >
          <div class="flex items-center gap-3">
            <UIcon name="i-lucide-file-code-2" class="size-4 text-dimmed" />
            <span class="font-mono text-xs text-highlighted">{{ table.name }}</span>
            <span class="text-[11px] text-muted">{{ table.type }}</span>
          </div>
          <UBadge color="neutral" variant="outline" size="xs" class="text-[10px]">
            {{ table.rows }}
          </UBadge>
        </div>
      </div>
    </div>
  </div>
</template>
