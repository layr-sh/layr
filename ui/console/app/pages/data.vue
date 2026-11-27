<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";
const auth = useAuth();
const route = useRoute();
const sections = [
  {
    label: "Tables",
    icon: "i-lucide-table-2",
    to: "/data",
    exact: true,
    scope: "data:schema.read",
  },
  {
    label: "SQL Editor",
    icon: "i-lucide-square-terminal",
    to: "/data/sql",
    scope: "data:schema.write",
  },
  {
    label: "Database Functions",
    icon: "i-lucide-square-function",
    to: "/data/functions",
    scope: "data:schema.write",
  },
  { label: "API Explorer", icon: "i-lucide-braces", to: "/data/api", scope: "data:query.read" },
  {
    label: "Key–Value",
    icon: "i-lucide-key-round",
    to: "/data/key-value",
    scope: "data:query.read",
  },
  { label: "Realtime", icon: "i-lucide-radio", to: "/data/realtime", scope: "data:query.read" },
  {
    label: "Configurations",
    icon: "i-lucide-settings",
    to: "/data/configurations",
    scope: "data:config.read",
  },
  { label: "Cache", icon: "i-lucide-database-zap", to: "/data/cache", scope: "data:cache.write" },
];
const items = computed<NavigationMenuItem[]>(() =>
  sections.filter((section) => auth.hasScope(section.scope)),
);
const canView = computed(() =>
  sections.some((section) => section.to === route.path && auth.hasScope(section.scope)),
);
</script>
<template>
  <div class="space-y-8">
    <header class="border-b border-default pb-6">
      <h1 class="text-2xl font-bold tracking-tight text-highlighted">Data Service</h1>
      <p class="mt-1 text-sm text-muted">
        Manage PostgreSQL tables, indexes, row-level security, and queries.
      </p>
    </header>
    <div class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div class="-mx-2.5">
        <UNavigationMenu
          orientation="vertical"
          :items="items"
          color="neutral"
          aria-label="Data Service Settings"
          class="w-full self-start lg:sticky lg:top-32"
        />
      </div>
      <main class="min-w-0 space-y-6">
        <NuxtPage v-if="canView" />
        <UEmpty
          v-else
          icon="i-lucide-lock-keyhole"
          title="Access Restricted"
          description="Your Service Account does not grant access to this section."
        />
      </main>
    </div>
  </div>
</template>
