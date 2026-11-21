<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { AuthControlPlaneModels as Models } from "layr-client";
import { authUserLabel, authUserLocked } from "../../utils/auth-forms";
import { authRecordRows } from "../../utils/auth-records";
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
const summary = computed(() => {
  const stats = authControlPlane.stats.value;
  return [
    { label: "Total Users", value: stats?.totalUsers ?? 0 },
    { label: "Guest Accounts", value: stats?.anonymousUsers ?? 0 },
    { label: "New Users (24 Hours)", value: stats?.newUsersLast24Hours ?? 0 },
    { label: "Locked Accounts", value: stats?.lockedUsers ?? 0 },
    { label: "MFA Enrolled", value: stats?.mfaUsers ?? 0 },
    { label: "Active Sessions", value: stats?.activeSessions ?? 0 },
    { label: "Users With Active Sessions", value: stats?.activeSessionUsers ?? 0 },
    { label: "Users With Passkeys", value: stats?.passkeyUsers ?? 0 },
  ];
});
const accountHealth = computed(() => {
  const stats = authControlPlane.stats.value;
  return [
    { label: "Registered Accounts", value: stats?.registeredUsers ?? 0 },
    { label: "Verified Email", value: stats?.emailVerifiedUsers ?? 0 },
    { label: "Unverified Email", value: stats?.emailUnverifiedUsers ?? 0 },
    { label: "Verified Phone", value: stats?.phoneVerifiedUsers ?? 0 },
    { label: "Unverified Phone", value: stats?.phoneUnverifiedUsers ?? 0 },
    { label: "Without Stored Credentials", value: stats?.usersWithoutCredentials ?? 0 },
  ];
});
const credentials = computed(() => {
  const stats = authControlPlane.stats.value;
  return [
    { label: "Users With Passwords", value: stats?.passwordUsers ?? 0 },
    { label: "Users With Passkeys", value: stats?.passkeyUsers ?? 0 },
    { label: "Users With External Identities", value: stats?.federatedUsers ?? 0 },
    { label: "Stored Passkeys", value: stats?.totalPasskeys ?? 0 },
    { label: "Linked Identities", value: stats?.totalIdentities ?? 0 },
  ];
});
const sessionStates = computed(() => {
  const stats = authControlPlane.stats.value;
  return [
    { label: "Active", value: stats?.activeSessions ?? 0 },
    { label: "Expired", value: stats?.expiredSessions ?? 0 },
    { label: "Locked Account", value: stats?.lockedSessions ?? 0 },
    { label: "Idle Timeout", value: stats?.idleSessions ?? 0 },
  ];
});
const registrations = computed(() => authControlPlane.stats.value?.registrations ?? []);
const registrationPeak = computed(() =>
  Math.max(1, ...registrations.value.map((day) => day.totalUsers)),
);
const providerColumns: TableColumn<Models.ProviderStats>[] = [
  { accessorKey: "provider", header: "Provider" },
  { accessorKey: "userCount", header: "Users" },
  { accessorKey: "identityCount", header: "Linked Identities" },
  { id: "lastSignedInAt", header: "Last Provider Sign-In" },
];
const roleColumns: TableColumn<Models.RoleStats>[] = [
  { accessorKey: "role", header: "JWT Role" },
  { accessorKey: "totalUsers", header: "Users" },
];
const methods = computed(() => {
  const config = authControlPlane.config.value;
  if (!config) return [];
  return [
    { label: "Passwords", enabled: config.password.enabled },
    { label: "Passkeys", enabled: config.passkeys.enabled },
    { label: "Email Codes", enabled: config.emailOtp.enabled },
    { label: "SMS Codes", enabled: config.smsOtp.enabled },
    { label: "Anonymous Sign-In", enabled: config.anonymous.enabled },
  ];
});
const providers = computed(() => {
  const config = authControlPlane.config.value;
  return config ? authRecordRows("provider", config) : [];
});
const columns: TableColumn<Models.User>[] = [
  { id: "identity", header: "User" },
  { accessorKey: "role", header: "JWT Role" },
  { id: "status", header: "Access" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions", header: "" },
];
async function load() {
  if (auth.hasScope("auth:stats.read") && !(await authControlPlane.loadStats())) return;
  if (auth.hasScope("auth:user.read") && !(await authControlPlane.listUsers())) return;
  if (auth.hasScope("auth:config.read")) await authControlPlane.loadConfig();
}
onMounted(load);
</script>
<template>
  <div class="space-y-6" :aria-busy="authControlPlane.busy.value">
    <UAlert
      v-if="authControlPlane.error.value"
      title="Request Failed"
      :description="authControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UCard
      v-if="auth.hasScope('auth:stats.read')"
      title="Authentication Statistics"
      description="Directory-wide application user totals. Active sessions exclude expired, idle, and locked-user sessions; they are not sign-in event counts."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <dl v-if="authControlPlane.stats.value" class="grid gap-6 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="item in summary" :key="item.label">
          <dt class="text-sm text-muted">{{ item.label }}</dt>
          <dd class="text-3xl font-semibold tabular-nums text-highlighted">
            {{ item.value.toLocaleString() }}
          </dd>
        </div>
      </dl>
      <UEmpty
        v-else
        :title="authControlPlane.busy.value ? 'Loading Statistics…' : 'Statistics Unavailable'"
        :loading="authControlPlane.busy.value"
        icon="i-lucide-users"
      />
      <template #footer>
        <span v-if="authControlPlane.stats.value" class="mr-auto text-sm text-muted">
          Snapshot {{ authControlPlane.stats.value.generatedAt.toLocaleString() }}
        </span>
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value"
          @click="load"
        />
        <UButton
          v-if="auth.hasScope('auth:user.read')"
          label="Browse Users"
          to="/auth/users"
          color="neutral"
          variant="outline"
        />
      </template>
    </UCard>
    <template v-if="auth.hasScope('auth:stats.read') && authControlPlane.stats.value">
      <UCard
        title="Account Registrations"
        description="Retained accounts created during the last 14 UTC calendar days, including today. Deleted accounts are not reconstructed."
        :ui="{ body: 'space-y-6' }"
      >
        <dl class="grid gap-6 sm:grid-cols-3">
          <div>
            <dt class="text-sm text-muted">Last 24 Hours</dt>
            <dd class="text-2xl font-semibold tabular-nums">
              {{ authControlPlane.stats.value.newUsersLast24Hours.toLocaleString() }}
            </dd>
          </div>
          <div>
            <dt class="text-sm text-muted">Previous 24 Hours</dt>
            <dd class="text-2xl font-semibold tabular-nums">
              {{ authControlPlane.stats.value.newUsersPrevious24Hours.toLocaleString() }}
            </dd>
          </div>
          <div>
            <dt class="text-sm text-muted">Last 7 Days</dt>
            <dd class="text-2xl font-semibold tabular-nums">
              {{ authControlPlane.stats.value.newUsersLast7Days.toLocaleString() }}
            </dd>
          </div>
        </dl>
        <ul class="space-y-3" aria-label="Daily account registrations">
          <li
            v-for="day in registrations"
            :key="day.day.toISOString()"
            class="grid grid-cols-[6rem_1fr_3rem] items-center gap-3"
          >
            <span class="text-sm text-muted">{{ day.day.toISOString().slice(0, 10) }}</span>
            <UProgress
              :model-value="day.totalUsers"
              :max="registrationPeak"
              :aria-label="`${day.totalUsers} accounts on ${day.day.toISOString().slice(0, 10)} UTC: ${day.registeredUsers} registered and ${day.anonymousUsers} guest accounts`"
            />
            <span class="text-right text-sm tabular-nums">{{
              day.totalUsers.toLocaleString()
            }}</span>
          </li>
        </ul>
      </UCard>
      <div class="grid gap-6 xl:grid-cols-2">
        <UCard
          title="Account Health"
          description="Verification counts apply to stored contacts. Accounts without stored passwords, passkeys, or external identities exclude guests and may still sign in using enabled one-time codes."
        >
          <dl class="space-y-4">
            <div
              v-for="item in accountHealth"
              :key="item.label"
              class="flex items-center justify-between gap-4"
            >
              <dt class="text-sm text-muted">{{ item.label }}</dt>
              <dd class="font-semibold tabular-nums">{{ item.value.toLocaleString() }}</dd>
            </div>
          </dl>
        </UCard>
        <UCard
          title="Authentication Credentials"
          description="Users can have more than one sign-in method. These counts are not mutually exclusive."
        >
          <dl class="space-y-4">
            <div
              v-for="item in credentials"
              :key="item.label"
              class="flex items-center justify-between gap-4"
            >
              <dt class="text-sm text-muted">{{ item.label }}</dt>
              <dd class="font-semibold tabular-nums">{{ item.value.toLocaleString() }}</dd>
            </div>
          </dl>
        </UCard>
      </div>
      <UCard
        title="Stored Sessions"
        description="Each retained session is counted once: expired first, then locked account, idle timeout, or active. Revoked sessions are deleted, not counted here."
        :ui="{ body: 'space-y-6' }"
      >
        <p class="text-sm text-muted">
          {{ authControlPlane.stats.value.totalSessions.toLocaleString() }} stored sessions ·
          {{ authControlPlane.stats.value.activeSessionUsers.toLocaleString() }} users with active
          sessions
        </p>
        <dl class="grid gap-6 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="item in sessionStates" :key="item.label">
            <dt class="text-sm text-muted">{{ item.label }}</dt>
            <dd class="text-2xl font-semibold tabular-nums">{{ item.value.toLocaleString() }}</dd>
          </div>
        </dl>
      </UCard>
      <div class="grid gap-6 xl:grid-cols-2">
        <UCard
          title="External Identity Providers"
          description="Currently linked identities, not historical sign-in volume. Users linked to multiple providers appear in each provider's count."
        >
          <UTable
            :data="authControlPlane.stats.value.providers ?? []"
            :columns="providerColumns"
            caption="Linked external identities by provider."
          >
            <template #lastSignedInAt-cell="{ row }">{{
              row.original.lastSignedInAt?.toLocaleString() || "Not recorded"
            }}</template>
            <template #empty
              ><UEmpty title="No External Identities" icon="i-lucide-log-in"
            /></template>
          </UTable>
        </UCard>
        <UCard
          title="User Roles"
          description="Current application user counts grouped by JWT role."
        >
          <UTable
            :data="authControlPlane.stats.value.roles ?? []"
            :columns="roleColumns"
            caption="Application users by JWT role."
          >
            <template #empty><UEmpty title="No User Roles" icon="i-lucide-users" /></template>
          </UTable>
        </UCard>
      </div>
    </template>
    <UCard
      v-if="auth.hasScope('auth:config.read')"
      title="Sign-In Configuration"
      description="Enabled authentication methods and the hosted OIDC experience."
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template v-if="authControlPlane.config.value">
        <dl class="grid gap-5 sm:grid-cols-2">
          <div
            v-for="method in methods"
            :key="method.label"
            class="flex flex-wrap items-center gap-2"
          >
            <dt class="text-sm">{{ method.label }}</dt>
            <dd>
              <UBadge
                :label="method.enabled ? 'Enabled' : 'Disabled'"
                :color="method.enabled ? 'success' : 'neutral'"
                variant="subtle"
              />
            </dd>
          </div>
        </dl>
        <USeparator label="External Providers" position="start" />
        <dl v-if="providers.length" class="grid gap-5 sm:grid-cols-2">
          <div
            v-for="provider in providers"
            :key="provider.key"
            class="flex flex-wrap items-center gap-2"
          >
            <dt class="text-sm" :title="String(provider.key)">{{ provider.name }}</dt>
            <dd>
              <UBadge
                :label="provider.status"
                :color="provider.status === 'Enabled' ? 'success' : 'neutral'"
                variant="subtle"
              />
            </dd>
          </div>
        </dl>
        <p v-else class="text-sm text-muted">No external identity providers configured.</p>
        <dl class="border-t border-default pt-5">
          <div class="flex flex-wrap items-center gap-2">
            <dt class="text-sm">Hosted OIDC</dt>
            <dd>
              <UBadge
                :label="authControlPlane.config.value.oidc.enabled ? 'Enabled' : 'Disabled'"
                :color="authControlPlane.config.value.oidc.enabled ? 'success' : 'neutral'"
                variant="subtle"
              />
            </dd>
          </div>
        </dl>
      </template>
      <UEmpty
        v-else
        title="Configuration Unavailable"
        description="Refresh to try loading your sign-in settings again."
        icon="i-lucide-settings"
      />
      <template #footer>
        <UButton
          label="Sign-In Methods"
          to="/auth/sign-in-methods"
          color="neutral"
          variant="outline"
        />
        <UButton label="Security" to="/auth/security" color="neutral" variant="outline" />
        <UButton
          label="Hosted Sign-In"
          to="/auth/hosted-sign-in"
          color="neutral"
          variant="outline"
        />
      </template>
    </UCard>
    <UCard
      v-if="auth.hasScope('auth:user.read')"
      title="Recent Users"
      description="The latest application accounts. Open the user directory for account details, access controls, and session management."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="authControlPlane.users.value?.data?.slice(0, 5) || []"
        :columns="columns"
        :loading="authControlPlane.busy.value"
        caption="Five most recently created application users."
      >
        <template #identity-cell="{ row }">{{ authUserLabel(row.original) }}</template>
        <template #status-cell="{ row }"
          ><UBadge
            :label="authUserLocked(row.original) ? 'Locked' : 'Unlocked'"
            :color="authUserLocked(row.original) ? 'warning' : 'neutral'"
            variant="subtle"
        /></template>
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #actions-cell="{ row }"
          ><UButton
            label="Sessions"
            :to="{ path: '/auth/sessions', query: { user_id: row.original.id } }"
            color="neutral"
            variant="ghost"
        /></template>
        <template #empty
          ><UEmpty
            title="No Users Yet"
            description="Application users will appear here after signing up. You can also create users in the directory."
            icon="i-lucide-users"
        /></template>
      </UTable>
      <template #footer
        ><UButton label="View All Users" to="/auth/users" color="neutral" variant="outline"
      /></template>
    </UCard>
  </div>
</template>
