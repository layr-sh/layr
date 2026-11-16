<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";

const auth = useAuth();
const route = useRoute();
const sections = [
  {
    label: "Configuration",
    icon: "i-lucide-settings",
    to: "/control-plane",
    exact: true,
    scope: "console:config.read",
  },
  {
    label: "Console Users",
    icon: "i-lucide-users",
    to: "/control-plane/users",
    scope: "console:user.read",
  },
  {
    label: "Service Accounts",
    icon: "i-lucide-key-round",
    to: "/control-plane/service-accounts",
    scope: "core:service-account.read",
  },
  {
    label: "Events",
    icon: "i-lucide-history",
    to: "/control-plane/events",
    scope: "core:event.read",
  },
  {
    label: "Event Hooks",
    icon: "i-lucide-webhook",
    to: "/control-plane/event-hooks",
    scope: "core:event-hook.read",
  },
  {
    label: "Account Security",
    icon: "i-lucide-shield-check",
    to: "/control-plane/account-security",
    scope: "",
  },
];
const items = computed<NavigationMenuItem[]>(() =>
  sections.filter(
    (section) =>
      !section.scope ||
      auth.hasScope(section.scope) ||
      (section.to === "/control-plane" && auth.hasScope("core:config.read")),
  ),
);
const canView = computed(() => {
  return sections.some(
    (section) =>
      section.to === route.path &&
      (!section.scope ||
        auth.hasScope(section.scope) ||
        (section.to === "/control-plane" && auth.hasScope("core:config.read"))),
  );
});
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Control Plane</h1>
        <p class="text-sm text-muted mt-1">
          Manage your console, access, and platform configuration.
        </p>
      </div>

      <div class="flex items-center gap-3">
        <!--  -->
      </div>
    </div>

    <div class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div class="-mx-2.5">
        <UNavigationMenu
          orientation="vertical"
          :items="items"
          color="neutral"
          aria-label="Control Plane Settings"
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
