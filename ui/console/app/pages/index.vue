<script setup lang="ts">
import { h, resolveComponent } from "vue";
import type { TableColumn } from "@nuxt/ui";

const UButton = resolveComponent("UButton");

const client = useClient();
const manifest = useManifest();

const { copy: copyKey, copied: isCopiedKey } = useClipboard({ copiedDuring: 2000 });
const { copy: copyUrl, copied: isCopiedUrl } = useClipboard({ copiedDuring: 2000 });
const { copy: copySnippet, copied: isCopiedSnippet } = useClipboard({ copiedDuring: 2000 });
const { copy: copyInstall, copied: isCopiedInstall } = useClipboard({ copiedDuring: 2000 });

const projectName = computed(() => manifest.state.value?.project?.name || "Layr App");
const projectDescription = computed(
  () =>
    manifest.state.value?.project?.description || "Unified application backend and control plane.",
);
const baseUrl = computed(
  () => client.value.endpoint || manifest.state.value?.server?.baseUrl || "http://localhost:5297",
);
const publishableKey = computed(() => manifest.state.value?.publishableKey || "");
const enabledServices = computed(() => new Set(manifest.state.value?.enabledServices || []));

// Fetch real telemetry and metrics across services
const { data: tablesData } = await useAsyncData("overview:tables", () =>
  client.value.data.cp.tables.list().catch(() => null),
);
const { data: bucketsData } = await useAsyncData("overview:buckets", () =>
  client.value.file_storage.cp.buckets.list().catch(() => null),
);
const { data: authStatsData } = await useAsyncData("overview:auth-stats", () =>
  client.value.auth.cp.stats.get().catch(() => null),
);
const { data: endpointsData } = await useAsyncData("overview:endpoints", () =>
  client.value.function.cp.endpoints.list().catch(() => null),
);
const { data: jobsData } = await useAsyncData("overview:jobs", () =>
  client.value.tasks.cp.jobs.list().catch(() => null),
);
const { data: presetsData } = await useAsyncData("overview:presets", () =>
  client.value.image.cp.presets.list().catch(() => null),
);
const { data: eventsData } = await useAsyncData("overview:events", () =>
  client.value.core.cp.events.list({ limit: 10 }).catch(() => null),
);

// Metrics calculations
const tablesCount = computed(() => tablesData.value?.count ?? 0);
const bucketsCount = computed(() => bucketsData.value?.count ?? 0);

const endpointsCount = computed(() => endpointsData.value?.count ?? 0);
const jobsCount = computed(() => jobsData.value?.count ?? 0);
const presetsCount = computed(() => presetsData.value?.count ?? 0);

const overviewServices = computed(() => [
  {
    title: "Data",
    to: "/data",
    icon: "i-lucide-database",
    serviceKey: "data",
    metric: `${tablesCount.value} ${tablesCount.value === 1 ? "Table" : "Tables"}`,
    description: "PostgreSQL tables, schemas, relations, and SQL query runner.",
    quickAction: "Explore Data",
  },
  {
    title: "Auth",
    to: "/auth/users",
    icon: "i-lucide-users",
    serviceKey: "auth",
    metric: authStatsData.value ? `${authStatsData.value.totalUsers} Users` : "Unavailable",
    description: "Application users, session records, and authentication methods.",
    quickAction: "Manage Users",
  },
  {
    title: "File Storage",
    to: "/file-storage",
    icon: "i-lucide-hard-drive",
    serviceKey: "file_storage",
    metric: `${bucketsCount.value} ${bucketsCount.value === 1 ? "Bucket" : "Buckets"}`,
    description: "Object storage buckets, uploaded media, and asset policies.",
    quickAction: "Browse Buckets",
  },
  {
    title: "Function",
    to: "/function",
    icon: "i-lucide-square-terminal",
    serviceKey: "function",
    metric: `${endpointsCount.value} ${endpointsCount.value === 1 ? "Endpoint" : "Endpoints"}`,
    description: "Serverless function runtime, HTTP routing, and execution logs.",
    quickAction: "View Endpoints",
  },
  {
    title: "Tasks",
    to: "/tasks",
    icon: "i-lucide-clock",
    serviceKey: "tasks",
    metric: `${jobsCount.value} ${jobsCount.value === 1 ? "Job" : "Jobs"}`,
    description: "Background queues, delayed workers, retries, and cron workers.",
    quickAction: "Inspect Queues",
  },
  {
    title: "Image",
    to: "/image/presets",
    icon: "i-lucide-image",
    serviceKey: "image",
    metric: `${presetsCount.value} ${presetsCount.value === 1 ? "Preset" : "Presets"}`,
    description: "Dynamic image transformations, optimization, and cached presets.",
    quickAction: "View Presets",
  },
]);

function formatRelativeTime(date?: Date | null): string {
  if (!date) return "Recent";
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
  if (isNaN(seconds) || seconds < 10) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

interface DisplayEvent {
  id: string;
  type: string;
  details: string;
  timestamp: string;
  payload: Record<string, unknown> | null;
}

const expanded = ref<Record<string, boolean>>({});

function getEventDetails(eventItem: {
  data?: Record<string, unknown> | null;
  action?: string;
  resourceId?: string;
}): string {
  if (
    eventItem.data &&
    typeof eventItem.data === "object" &&
    Object.keys(eventItem.data).length > 0
  ) {
    return Object.entries(eventItem.data)
      .map(([entryKey, entryValue]) => {
        if (Array.isArray(entryValue)) {
          return `${entryKey}: [${entryValue.join(", ")}]`;
        }
        if (entryValue && typeof entryValue === "object") {
          return `${entryKey}: ${JSON.stringify(entryValue)}`;
        }
        return `${entryKey}: ${String(entryValue)}`;
      })
      .join(" · ");
  }
  if (eventItem.resourceId) {
    return `ID: ${eventItem.resourceId}`;
  }
  return eventItem.action || "—";
}

const eventColumns: TableColumn<DisplayEvent>[] = [
  { id: "expand", header: "" },
  {
    accessorKey: "type",
    header: "Event",
    cell: ({ row }) =>
      h("div", { class: "flex items-center gap-2.5 min-w-0" }, [
        h(
          "span",
          { class: "font-mono text-highlighted text-xs font-medium shrink-0" },
          String(row.getValue("type") || ""),
        ),
        row.original.details && row.original.details !== "—"
          ? h(
              "span",
              { class: "font-mono text-muted text-xs truncate max-w-4xl" },
              row.original.details,
            )
          : null,
      ]),
  },
  {
    accessorKey: "timestamp",
    header: "Time",
    meta: {
      class: {
        th: "text-right",
        td: "text-right font-mono text-muted text-[11px] whitespace-nowrap",
      },
    },
    cell: ({ row }) => String(row.getValue("timestamp") || ""),
  },
];

const liveEvents = computed<DisplayEvent[]>(() => {
  const eventsList = eventsData.value?.data;
  if (eventsList && eventsList.length > 0) {
    return eventsList.map((eventItem) => ({
      id: String(eventItem.id || Math.random()),
      type: eventItem.type || "core.event",
      details: getEventDetails(eventItem),
      timestamp: formatRelativeTime(eventItem.createdAt),
      payload: (eventItem.data as Record<string, unknown> | null) || null,
    }));
  }
  return [];
});

const sdkSnippet = computed(
  () => `import { createClient } from "layr-client";

export const client = createClient({
  endpoint: "${baseUrl.value}",
  publishableKey: "${publishableKey.value}",
});`,
);
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">
          {{ projectName }}
        </h1>
        <p class="text-sm text-muted mt-1 max-w-2xl">
          {{ projectDescription }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <UButton
          :to="`${baseUrl}/v1/manifest`"
          target="_blank"
          color="neutral"
          variant="outline"
          size="xs"
          icon="i-lucide-layers"
          trailing-icon="i-lucide-external-link"
          label="Manifest"
        />
      </div>
    </div>

    <!-- Project Connection & Client SDK Banner -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-cable" class="size-4 text-muted" />
          <h2 class="text-sm font-semibold text-highlighted">Connect to Project</h2>
        </div>
        <div class="flex items-center gap-2 text-xs font-mono text-muted">
          <span class="bg-muted px-2.5 py-1 rounded border border-default">
            npm i layr-client
          </span>
          <UButton
            square
            :icon="isCopiedInstall ? 'i-lucide-check' : 'i-lucide-copy'"
            color="neutral"
            variant="ghost"
            size="xs"
            @click="copyInstall('npm i layr-client')"
          />
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <!-- Left: API Credentials -->
        <div class="space-y-4 flex flex-col justify-between">
          <div class="space-y-4">
            <!-- API Endpoint -->
            <div>
              <span class="text-xs font-medium text-muted block mb-1.5">API Endpoint</span>
              <div
                class="flex items-center justify-between gap-3 px-4 py-2.5 rounded-md border border-default"
              >
                <span class="font-mono text-xs text-highlighted truncate">
                  {{ baseUrl }}
                </span>
                <UButton
                  :icon="isCopiedUrl ? 'i-lucide-check' : 'i-lucide-copy'"
                  :label="isCopiedUrl ? 'Copied' : 'Copy'"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  class="-mr-2"
                  @click="copyUrl(baseUrl)"
                />
              </div>
            </div>

            <!-- Publishable Key -->
            <div>
              <span class="text-xs font-medium text-muted block mb-1.5">Publishable Key</span>
              <div
                class="flex items-center justify-between gap-3 px-4 py-2.5 rounded-md border border-default"
              >
                <span class="font-mono text-xs text-highlighted truncate">
                  {{ publishableKey }}
                </span>
                <UButton
                  :icon="isCopiedKey ? 'i-lucide-check' : 'i-lucide-copy'"
                  :label="isCopiedKey ? 'Copied' : 'Copy'"
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  :disabled="!publishableKey"
                  @click="copyKey(publishableKey)"
                />
              </div>
            </div>
          </div>

          <p class="text-xs text-dimmed">
            Use the endpoint and publishable key to initialize the Layr client SDK.
          </p>
        </div>

        <!-- Right: SDK Code Snippet -->
        <div class="flex flex-col justify-between bg-muted/30 rounded-md">
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <span class="text-xs font-medium text-muted">Client Initialization</span>
              <UButton
                :icon="isCopiedSnippet ? 'i-lucide-check' : 'i-lucide-copy'"
                :label="isCopiedSnippet ? 'Copied' : 'Copy Code'"
                color="neutral"
                variant="link"
                size="xs"
                class="p-0!"
                @click="copySnippet(sdkSnippet)"
              />
            </div>
            <pre
              class="py-3 px-4 rounded-md bg-default border border-default text-xs font-mono text-default overflow-x-auto whitespace-pre leading-relaxed"
            ><code>{{ sdkSnippet }}</code></pre>
          </div>
        </div>
      </div>
    </div>

    <!-- Services Cards (2x3 Grid) -->
    <div class="space-y-4">
      <div class="flex items-center gap-2">
        <UIcon name="i-lucide-blocks" class="size-4 text-muted" />
        <h2 class="text-sm font-semibold text-highlighted">Services</h2>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="service in overviewServices"
          :key="service.to"
          class="p-5 rounded-lg border transition flex flex-col justify-between"
          :class="[
            enabledServices.has(service.serviceKey)
              ? 'border-default bg-muted/20 hover:border-accented'
              : 'border-default/40 bg-muted/10 opacity-60 cursor-not-allowed select-none',
          ]"
        >
          <div>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2.5">
                <div
                  class="size-8 rounded border flex items-center justify-center"
                  :class="[
                    enabledServices.has(service.serviceKey)
                      ? 'bg-accented/50 border-default text-highlighted'
                      : 'bg-muted/40 border-default/40 text-dimmed',
                  ]"
                >
                  <UIcon :name="service.icon" class="size-4" />
                </div>
                <h3
                  class="text-sm font-semibold"
                  :class="[
                    enabledServices.has(service.serviceKey) ? 'text-highlighted' : 'text-muted',
                  ]"
                >
                  {{ service.title }}
                </h3>
              </div>
              <UBadge
                color="neutral"
                variant="subtle"
                size="xs"
                class="font-mono text-[11px]"
                :class="{ 'opacity-50': !enabledServices.has(service.serviceKey) }"
              >
                {{ service.metric }}
              </UBadge>
            </div>

            <p class="text-xs text-muted mt-3 leading-relaxed">
              {{ service.description }}
            </p>
          </div>

          <div
            class="mt-5 pt-3 border-t border-default/50 flex items-center justify-between text-xs"
          >
            <span class="text-muted flex items-center gap-1.5">
              <span
                class="size-1.5 rounded-full"
                :class="enabledServices.has(service.serviceKey) ? 'bg-success' : 'bg-dimmed'"
              />
              <span>{{ enabledServices.has(service.serviceKey) ? "Enabled" : "Disabled" }}</span>
            </span>

            <NuxtLink
              v-if="enabledServices.has(service.serviceKey)"
              :to="service.to"
              class="font-medium text-toned hover:text-highlighted transition flex items-center gap-1"
            >
              <span>{{ service.quickAction }}</span>
              <UIcon name="i-lucide-arrow-right" class="size-3" />
            </NuxtLink>
            <span
              v-else
              class="font-medium text-dimmed cursor-not-allowed flex items-center gap-1 select-none"
              :title="`${service.title} is disabled`"
            >
              <span>{{ service.quickAction }}</span>
              <UIcon name="i-lucide-arrow-right" class="size-3" />
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Events (Full-Width Clean Audit Log) -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-activity" class="size-4 text-muted" />
          <h2 class="text-sm font-semibold text-highlighted">Events</h2>
        </div>
        <div class="flex flex-row items-center gap-2">
          <NuxtLink
            to="/control-plane/events"
            class="text-muted hover:text-highlighted transition flex items-center gap-1 text-sm"
          >
            <span>Show All</span>
            <UIcon name="i-lucide-arrow-right" class="size-3" />
          </NuxtLink>
        </div>
      </div>

      <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
        <!-- Empty State -->
        <div
          v-if="liveEvents.length === 0"
          class="p-12 text-center flex flex-col items-center justify-center"
        >
          <div
            class="size-10 rounded-full bg-accented/50 border border-default flex items-center justify-center text-muted mb-3"
          >
            <UIcon name="i-lucide-inbox" class="size-5" />
          </div>
          <p class="text-sm font-medium text-highlighted">No recent events</p>
          <p class="text-xs text-muted mt-1 max-w-sm">
            Audit events from Data, Auth, File Storage, and Control Plane will appear here as
            activity occurs.
          </p>
        </div>

        <!-- Live Event List via UTable with expandable rows -->
        <UTable
          v-else
          v-model:expanded="expanded"
          :data="liveEvents"
          :columns="eventColumns"
          :column-pinning="{
            left: ['expand'],
            right: ['timestamp'],
          }"
          :ui="{
            tr: 'data-[expanded=true]:bg-accented/50',
            td: '[&:has([aria-label=Expand])]:px-3',
            thead: 'hidden',
          }"
        >
          <template #expand-cell="{ row }">
            <UButton
              :label="row.getIsExpanded() ? 'Hide Payload' : 'View Payload'"
              color="neutral"
              variant="ghost"
              @click="row.toggleExpanded()"
            />
          </template>
          <template #expanded="{ row }">
            <div class="p-3.5 bg-default border border-default rounded-md m-2">
              <div class="flex items-center justify-between mb-2">
                <span class="text-[10px] font-mono text-muted uppercase tracking-wider">
                  Event Payload
                </span>
                <span class="text-[10px] font-mono text-dimmed">ID: {{ row.original.id }}</span>
              </div>
              <pre
                class="text-[11px] font-mono text-default overflow-x-auto whitespace-pre leading-relaxed"
              ><code>{{ row.original.payload ? JSON.stringify(row.original.payload, null, 2) : "{}" }}</code></pre>
            </div>
          </template>
        </UTable>
      </div>
    </div>
  </div>
</template>
