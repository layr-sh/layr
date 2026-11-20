<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";
const auth = useAuth();
const route = useRoute();
const sections = [
  {
    label: "Statistics",
    icon: "i-lucide-chart-no-axes-combined",
    to: "/file-storage",
    exact: true,
    scope: "storage:bucket.read",
  },
  {
    label: "Buckets",
    icon: "i-lucide-folder",
    to: "/file-storage/buckets",
    scope: "storage:bucket.read",
  },
  {
    label: "Objects",
    icon: "i-lucide-files",
    to: "/file-storage/objects",
    scope: "storage:bucket.read",
  },
  {
    label: "URL Signing",
    icon: "i-lucide-link",
    to: "/file-storage/signing",
    scope: "storage:object.read",
  },
  {
    label: "Configurations",
    icon: "i-lucide-settings",
    to: "/file-storage/configurations",
    scope: "storage:config.read",
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
      <h1 class="text-2xl font-bold tracking-tight text-highlighted">File Storage Service</h1>
      <p class="mt-1 text-sm text-muted">
        Manage buckets, stored files, access policies, and temporary sharing links.
      </p>
    </header>
    <div class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div class="-mx-2.5">
        <UNavigationMenu
          orientation="vertical"
          :items="items"
          color="neutral"
          aria-label="File Storage Service Settings"
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
