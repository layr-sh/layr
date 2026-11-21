<script setup lang="ts">
import type { NavigationMenuItem } from "@nuxt/ui";
const auth = useAuth();
const route = useRoute();
const sections = [
  {
    label: "Overview",
    icon: "i-lucide-chart-no-axes-combined",
    to: "/auth",
    exact: true,
    scope: "auth:stats.read",
  },
  { label: "Users", icon: "i-lucide-users", to: "/auth/users", scope: "auth:user.read" },
  { label: "Sessions", icon: "i-lucide-monitor", to: "/auth/sessions", scope: "auth:user.read" },
  {
    label: "Identity Providers",
    icon: "i-lucide-log-in",
    to: "/auth/providers",
    scope: "auth:config.read",
  },
  {
    label: "Applications",
    icon: "i-lucide-app-window",
    to: "/auth/applications",
    scope: "auth:config.read",
  },
  {
    label: "APIs",
    icon: "i-lucide-server",
    to: "/auth/resource-servers",
    scope: "auth:config.read",
  },
  {
    label: "Sign-In Methods",
    icon: "i-lucide-key-round",
    to: "/auth/sign-in-methods",
    scope: "auth:config.read",
  },
  {
    label: "Security",
    icon: "i-lucide-shield-check",
    to: "/auth/security",
    scope: "auth:config.read",
  },
  {
    label: "Hosted Sign-In",
    icon: "i-lucide-panels-top-left",
    to: "/auth/hosted-sign-in",
    scope: "auth:config.read",
  },
  { label: "Email", icon: "i-lucide-mail", to: "/auth/email", scope: "auth:config.read" },
  { label: "SMS", icon: "i-lucide-message-square", to: "/auth/sms", scope: "auth:config.read" },
  {
    label: "Session Settings",
    icon: "i-lucide-settings",
    to: "/auth/configurations",
    scope: "auth:config.read",
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
      <h1 class="text-2xl font-bold tracking-tight text-highlighted">Auth Service</h1>
      <p class="mt-1 text-sm text-muted">
        Manage application users, connected applications, sign-in experiences, and security. Console
        administrator accounts are managed separately in the control plane.
      </p>
    </header>
    <div class="grid gap-8 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div class="-mx-2.5">
        <UNavigationMenu
          orientation="vertical"
          :items="items"
          color="neutral"
          aria-label="Auth Service Settings"
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
