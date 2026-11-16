<script setup lang="ts">
const authProviders = [
  { name: "Email & Password", status: "Enabled", type: "Native" },
  { name: "Magic Link / OTP", status: "Enabled", type: "Passwordless" },
  { name: "OAuth 2.0 / OIDC", status: "Configurable", type: "Social" },
  { name: "Passkeys / WebAuthn", status: "Configurable", type: "FIDO2" },
];
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Auth</h1>
        <p class="text-sm text-muted mt-1">
          Application end users, authentication methods, and active sessions.
        </p>
      </div>

      <div class="flex items-center gap-3">
        <UButton
          label="Add User"
          icon="i-lucide-user-plus"
          color="neutral"
          variant="solid"
          size="sm"
        />
      </div>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="p-5 rounded-lg border border-default bg-muted/30">
        <div class="flex items-center justify-between text-muted text-xs">
          <span>Registered End Users</span>
          <UIcon name="i-lucide-users" class="size-4" />
        </div>
        <div class="mt-2 text-xl font-bold text-highlighted">0</div>
        <p class="text-xs text-dimmed mt-1">Application accounts</p>
      </div>

      <div class="p-5 rounded-lg border border-default bg-muted/30">
        <div class="flex items-center justify-between text-muted text-xs">
          <span>Active End-User Sessions</span>
          <UIcon name="i-lucide-shield-check" class="size-4" />
        </div>
        <div class="mt-2 text-xl font-bold text-highlighted">0</div>
        <p class="text-xs text-dimmed mt-1">Bearer &amp; cookie sessions</p>
      </div>

      <div class="p-5 rounded-lg border border-default bg-muted/30">
        <div class="flex items-center justify-between text-muted text-xs">
          <span>Auth Providers</span>
          <UIcon name="i-lucide-key" class="size-4" />
        </div>
        <div class="mt-2 text-xl font-bold text-highlighted">4 Available</div>
        <p class="text-xs text-dimmed mt-1">Password, OTP, OAuth, WebAuthn</p>
      </div>
    </div>

    <!-- Provider Configuration Cards -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <span class="text-sm font-semibold text-highlighted">Authentication Methods</span>
        <span class="text-xs text-muted font-mono">Managed by Layr Auth Service</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="provider in authProviders"
          :key="provider.name"
          class="px-4 py-3.5 flex items-center justify-between hover:bg-accented/30 transition text-sm"
        >
          <div class="flex items-center gap-3">
            <UIcon name="i-lucide-check-circle-2" class="size-4 text-success" />
            <span class="font-medium text-highlighted">{{ provider.name }}</span>
            <span class="text-[11px] text-dimmed font-mono">({{ provider.type }})</span>
          </div>
          <UBadge color="neutral" variant="outline" size="xs" class="text-[10px]">
            {{ provider.status }}
          </UBadge>
        </div>
      </div>
    </div>
  </div>
</template>
