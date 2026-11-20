<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";
const auth = useAuth();
const route = useRoute();
const sections = [
  {
    label: "Statistics",
    icon: "i-lucide-chart-no-axes-combined",
    to: "/function",
    exact: true,
    scope: "function:stats.read",
  },
  {
    label: "Endpoints",
    icon: "i-lucide-square-terminal",
    to: "/function/endpoints",
    scope: "function:endpoint.read",
  },
  {
    label: "Deployments",
    icon: "i-lucide-rocket",
    to: "/function/deployments",
    scope: "function:endpoint.read",
  },
  {
    label: "Executions",
    icon: "i-lucide-list-checks",
    to: "/function/executions",
    scope: "function:endpoint.read",
  },
  {
    label: "Domains",
    icon: "i-lucide-globe",
    to: "/function/domains",
    scope: "function:endpoint.read",
  },
  {
    label: "Configurations",
    icon: "i-lucide-settings",
    to: "/function/configurations",
    scope: "function:config.read",
  },
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
      <h1 class="text-2xl font-bold tracking-tight text-highlighted">Function Service</h1>
      <p class="mt-1 text-sm text-muted">
        Manage serverless endpoints, code deployments, execution logs, and custom domains.
      </p>
    </header>
    <div class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div class="-mx-2.5">
        <UNavigationMenu
          orientation="vertical"
          :items="items"
          color="neutral"
          aria-label="Function Service Settings"
          class="w-full self-start lg:sticky lg:top-32"
        />
      </div>
      <main class="min-w-0 space-y-6">
        <NuxtPage v-if="canView" /><UEmpty
          v-else
          icon="i-lucide-lock-keyhole"
          title="Access Restricted"
          description="Your Service Account does not grant access to this section."
        />
      </main>
    </div>
  </div>
</template>
