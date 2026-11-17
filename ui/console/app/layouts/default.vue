<script setup lang="ts">
import type { DropdownMenuItem } from "@nuxt/ui";

const auth = useAuth();
const manifest = useManifest();
const route = useRoute();

const projectName = computed(() => manifest.state.value?.project?.name || "Layr App");
const userEmail = computed(() => auth.state.value?.user?.email || "Console User");

const isSigningOut = ref(false);
const toast = useToast();

async function handleSignOut() {
  if (isSigningOut.value) return;
  isSigningOut.value = true;
  try {
    const result = await auth.signOut();
    if (result.error) {
      toast.add({ title: "Sign-Out Failed", description: result.error.detail, color: "error" });
      return;
    }
    await navigateTo("/sign-in");
  } finally {
    isSigningOut.value = false;
  }
}

const enabledServices = computed(() => new Set(manifest.state.value?.enabledServices || []));

const navItems = [
  { label: "Overview", to: "/", icon: "i-lucide-layout-dashboard" },
  { label: "Data", to: "/data", icon: "i-lucide-database", serviceKey: "data" },
  { label: "Auth", to: "/auth", icon: "i-lucide-users", serviceKey: "auth" },
  { label: "Storage", to: "/storage", icon: "i-lucide-hard-drive", serviceKey: "file_storage" },
  {
    label: "Functions",
    to: "/functions",
    icon: "i-lucide-square-terminal",
    serviceKey: "function",
  },
  { label: "Tasks", to: "/tasks", icon: "i-lucide-clock", serviceKey: "tasks" },
  { label: "Images", to: "/image", icon: "i-lucide-image", serviceKey: "image" },
  { label: "Control Plane", to: "/control-plane", icon: "i-lucide-shield-check" },
];

function isNavEnabled(item: (typeof navItems)[number]) {
  if (!("serviceKey" in item) || !item.serviceKey) return true;
  return enabledServices.value.has(item.serviceKey);
}

function isRouteActive(path: string) {
  if (path === "/") {
    return route.path === "/" || route.path === "";
  }
  return route.path === path || route.path.startsWith(`${path}/`);
}

const userMenuItems = computed<DropdownMenuItem[][]>(() => [
  [
    {
      label: "Account Security",
      icon: "i-lucide-shield-check",
      to: "/control-plane/account-security",
    },
  ],
  [
    {
      label: "Sign Out",
      icon: "i-lucide-log-out",
      color: "error" as const,
      onSelect: () => {
        handleSignOut();
      },
    },
  ],
]);
</script>

<template>
  <div class="min-h-screen flex flex-col font-sans selection:bg-inverted selection:text-inverted">
    <!-- Top Header (Tier 1) -->
    <header
      class="sticky top-0 z-40 w-full border-b border-default bg-elevated/25 backdrop-blur-md"
    >
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between h-14">
          <!-- Left: Brand + Breadcrumb + Project Context -->
          <div class="flex items-center gap-2">
            <NuxtLink
              to="/"
              class="flex items-center gap-2 group transition-opacity hover:opacity-85"
            >
              <img
                src="/icon-glyph-light.png"
                width="32"
                height="32"
                class="w-5 h-5 -ml-px dark:hidden"
              />
              <img
                src="/icon-glyph-dark.png"
                width="32"
                height="32"
                class="w-5 h-5 -ml-px hidden dark:block"
              />
              <span class="font-semibold text-base tracking-tight">Layr</span>
            </NuxtLink>

            <span class="font-semibold text-base tracking-tight text-muted">/</span>

            <!-- Project Pill -->
            <div class="flex items-center gap-2">
              <span class="font-semibold text-base tracking-tight text-muted">{{
                projectName
              }}</span>
            </div>
          </div>

          <!-- Right: User Profile -->
          <div class="flex items-center gap-2">
            <UColorModeButton size="sm" />

            <!-- Console User Dropdown -->
            <UDropdownMenu :items="userMenuItems" :content="{ align: 'end', sideOffset: 8 }">
              <UButton
                :label="userEmail"
                color="neutral"
                variant="outline"
                size="sm"
                trailing-icon="i-lucide-chevron-down"
                :ui="{
                  base: 'gap-0.5',
                  trailingIcon: '-mr-1',
                }"
                class="max-w-32"
              />
            </UDropdownMenu>
          </div>
        </div>
      </div>

      <!-- Sub-navigation Bar (Tier 2) -->
      <div class="">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <nav class="flex items-center gap-6 overflow-x-auto no-scrollbar -mb-px">
            <template v-for="item in navItems" :key="item.to">
              <NuxtLink
                v-if="isNavEnabled(item)"
                :to="item.to"
                class="flex items-center gap-2 py-2.5 text-xs sm:text-sm font-medium transition-all whitespace-nowrap cursor-pointer"
                :class="[
                  isRouteActive(item.to)
                    ? 'text-highlighted'
                    : 'text-muted/75 hover:text-highlighted',
                ]"
              >
                <UIcon :name="item.icon" class="size-4 shrink-0" />
                <span>{{ item.label }}</span>
              </NuxtLink>

              <span
                v-else
                class="flex items-center gap-2 py-2.5 text-xs sm:text-sm font-medium text-dimmed opacity-40 cursor-not-allowed select-none whitespace-nowrap"
                :title="`${item.label} service is disabled`"
              >
                <UIcon :name="item.icon" class="size-4 shrink-0" />
                <span>{{ item.label }}</span>
              </span>
            </template>
          </nav>
        </div>
      </div>
    </header>

    <!-- Content Area -->
    <main class="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <slot />
    </main>

    <!-- Footer -->
    <footer class="py-3 mt-auto">
      <div
        class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs"
      >
        <div class="flex items-center gap-2">
          <span class="font-semibold text-muted"
            >Layr v{{ /* manifest.state.value.version */ "0.1.0" }}</span
          >
        </div>
        <div class="flex items-center gap-4">
          <NuxtLink to="#" class="text-muted hover:text-highlighted transition">Community</NuxtLink>
          <NuxtLink to="#" class="text-muted hover:text-highlighted transition"
            >Documentation</NuxtLink
          >
        </div>
      </div>
    </footer>
  </div>
</template>
