<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);
const allowedCidrs = ref("");

async function reload() {
  if (auth.hasScope("core:config.read")) {
    await controlPlane.load("platform");
    allowedCidrs.value = (controlPlane.coreConfig.value?.allowedCidrs || []).join("\n");
  }
  if (auth.hasScope("console:config.read")) await controlPlane.load("settings");
}
async function savePlatformSettings() {
  if (!controlPlane.coreConfig.value) return;
  await controlPlane.saveCoreConfig({
    ...controlPlane.coreConfig.value,
    allowedCidrs: allowedCidrs.value
      .split(/[\n,]/)
      .map((entry) => entry.trim())
      .filter(Boolean),
  });
}
async function saveSettings() {
  if (!controlPlane.config.value) return;
  await controlPlane.saveConfig(controlPlane.config.value);
}
onMounted(reload);
</script>

<template>
  <div class="space-y-6" :aria-busy="controlPlane.busy.value">
    <UAlert
      v-if="controlPlane.error.value"
      color="error"
      title="Request Failed"
      :description="controlPlane.error.value"
      role="alert"
    />
    <UAlert
      v-if="controlPlane.notice.value"
      color="success"
      :title="controlPlane.notice.value"
      role="status"
    />
    <UCard
      v-if="auth.hasScope('core:config.read')"
      title="Platform"
      description="Core runtime policies shared across all services."
      :ui="{ body: 'space-y-5' }"
    >
      <UForm
        v-if="controlPlane.coreConfig.value"
        id="platform-settings"
        :state="controlPlane.coreConfig.value"
        :disabled="controlPlane.busy.value || !auth.hasScope('core:config.write')"
        class="space-y-5"
        @submit="savePlatformSettings"
      >
        <UFormField
          label="Allowed IP Addresses and CIDRs"
          description="One address or CIDR per line. Empty allows all networks. This restricts requests across every service; excluding your network will block your access."
        >
          <UTextarea v-model="allowedCidrs" placeholder="192.168.1.0/24" class="w-full" />
        </UFormField>
        <UFormField
          label="Audit Retention (Days)"
          description="Expired events from all services and their delivery history are permanently deleted at startup and hourly. Changes apply at the next cleanup."
          required
        >
          <UInput
            v-model.number="controlPlane.coreConfig.value.auditRetentionDays"
            type="number"
            :min="1"
            required
            class="w-full sm:max-w-xs"
          />
        </UFormField>
      </UForm>
      <UEmpty
        v-else
        :title="controlPlane.busy.value ? 'Loading Platform Settings…' : 'Settings Unavailable'"
        icon="i-lucide-sliders-horizontal"
      />
      <template v-if="auth.hasScope('core:config.write')" #footer>
        <div class="flex justify-end">
          <UButton
            type="submit"
            form="platform-settings"
            label="Save Platform Config"
            :disabled="!controlPlane.coreConfig.value || controlPlane.busy.value"
          />
        </div>
      </template>
    </UCard>
    <UCard
      v-if="auth.hasScope('console:config.read')"
      title="Console"
      description="Session and MFA configuration for Console Users."
      :ui="{ body: 'space-y-5' }"
    >
      <UForm
        v-if="controlPlane.config.value"
        id="console-settings"
        :state="controlPlane.config.value"
        :disabled="controlPlane.busy.value || !auth.hasScope('console:config.write')"
        class="space-y-5"
        @submit="saveSettings"
      >
        <UFormField
          label="Access Token Lifetime (Seconds)"
          description="Lifetime of newly issued Console access tokens. This is not a sliding inactivity timer."
          required
        >
          <UInput
            v-model.number="controlPlane.config.value.sessionIdleTimeoutSeconds"
            type="number"
            :min="1"
            :max="controlPlane.config.value.sessionMaxLifetimeSeconds"
            required
            class="w-full sm:max-w-xs"
          />
        </UFormField>
        <UFormField label="Maximum Session Lifetime (Seconds)" required>
          <UInput
            v-model.number="controlPlane.config.value.sessionMaxLifetimeSeconds"
            type="number"
            :min="controlPlane.config.value.sessionIdleTimeoutSeconds"
            required
            class="w-full sm:max-w-xs"
          />
        </UFormField>
        <USwitch
          v-model="controlPlane.config.value.mfaEnforced"
          label="Require Multi-Factor Authentication"
          description="Require MFA verification before Console sessions can access the platform. Users without MFA can enroll through Account Security."
        />
      </UForm>
      <UEmpty
        v-else
        :title="controlPlane.busy.value ? 'Loading Console Settings…' : 'Settings Unavailable'"
        icon="i-lucide-settings"
      />
      <template v-if="auth.hasScope('console:config.write')" #footer>
        <div class="flex justify-end">
          <UButton
            type="submit"
            form="console-settings"
            label="Save Console Config"
            :disabled="!controlPlane.config.value || controlPlane.busy.value"
          />
        </div>
      </template>
    </UCard>
  </div>
</template>
