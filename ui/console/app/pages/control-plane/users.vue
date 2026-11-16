<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { ConsoleControlPlaneModels } from "layr-client";

const auth = useAuth();
const route = useRoute();
const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);
const isSelf = computed(
  () =>
    !!controlPlane.selectedUser.value &&
    controlPlane.selectedUser.value.id === auth.state.value?.user.id,
);
const linkedAccount = computed(
  () =>
    controlPlane.selectedUser.value &&
    controlPlane.accounts.value.find(
      (account) => account.consoleUserId === controlPlane.selectedUser.value!.id,
    ),
);
const rootUserIds = computed(
  () =>
    new Set(
      controlPlane.accounts.value
        .filter(
          (account) =>
            account.isEnabled &&
            account.scopes?.includes("*") &&
            (!account.expiresAt || account.expiresAt > new Date()),
        )
        .map((account) => account.consoleUserId),
    ),
);
const isLastRoot = computed(
  () =>
    !!controlPlane.selectedUser.value?.isEnabled &&
    rootUserIds.value.has(controlPlane.selectedUser.value.id) &&
    !controlPlane.users.value.some(
      (user) =>
        user.id !== controlPlane.selectedUser.value!.id &&
        user.isEnabled &&
        rootUserIds.value.has(user.id),
    ),
);
const userForm = reactive({ email: "", password: "", isEnabled: true });
const confirmation = shallowRef<{
  title: string;
  description: string;
  action: () => Promise<boolean>;
} | null>(null);
const userColumns: TableColumn<ConsoleControlPlaneModels.User>[] = [
  { accessorKey: "email", header: "Email" },
  { accessorKey: "isEnabled", header: "Status" },
  { accessorKey: "mfaEnabled", header: "MFA" },
  { id: "access", header: "Service Account Access" },
  { accessorKey: "lastSignedInAt", header: "Last Sign-In" },
  { id: "actions" },
];
const sessionColumns: TableColumn<ConsoleControlPlaneModels.Session>[] = [
  { accessorKey: "ipAddress", header: "IP Address" },
  {
    accessorKey: "userAgent",
    header: "Device",
    meta: { class: { td: "max-w-xs whitespace-normal break-words" } },
  },
  { accessorKey: "expiresAt", header: "Expires" },
  { id: "actions" },
];
function closeUser() {
  controlPlane.selectedUser.value = null;
  userForm.email = "";
  userForm.password = "";
}
async function createUser() {
  if (await controlPlane.createUser({ email: userForm.email.trim(), password: userForm.password }))
    closeUser();
}
async function editUser(userId: string) {
  userForm.password = "";
  if (await controlPlane.selectUser(userId)) {
    userForm.email = controlPlane.selectedUser.value!.email;
    userForm.isEnabled = controlPlane.selectedUser.value!.isEnabled;
  }
}
async function saveUser() {
  if (!controlPlane.selectedUser.value) return;
  await controlPlane.updateUser(controlPlane.selectedUser.value.id, {
    email: userForm.email.trim(),
    isEnabled: userForm.isEnabled,
  });
}
async function resetPassword() {
  if (!controlPlane.selectedUser.value) return;
  if (await controlPlane.resetPassword(controlPlane.selectedUser.value.id, userForm.password))
    userForm.password = "";
}
async function confirmAction() {
  if (confirmation.value && (await confirmation.value.action())) {
    confirmation.value = null;
    if (!controlPlane.selectedUser.value) closeUser();
  }
}
onMounted(async () => {
  await controlPlane.load("users");
  if (typeof route.query.user === "string") await editUser(route.query.user);
});
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
      title="Console Users"
      description="Console Users authorize control-plane operations through their own Service Accounts, separately from End Users."
    >
      <UTable
        :data="controlPlane.users.value"
        :columns="userColumns"
        :loading="controlPlane.busy.value"
      >
        <template #isEnabled-cell="cell"
          ><UBadge :color="cell.row.original.isEnabled ? 'success' : 'neutral'" variant="subtle">{{
            cell.row.original.isEnabled ? "Enabled" : "Disabled"
          }}</UBadge></template
        >
        <template #mfaEnabled-cell="cell"
          ><UBadge :color="cell.row.original.mfaEnabled ? 'success' : 'warning'" variant="subtle">{{
            cell.row.original.mfaEnabled ? "Enrolled" : "Not Enrolled"
          }}</UBadge></template
        >
        <template #access-cell="cell"
          ><UBadge v-if="rootUserIds.has(cell.row.original.id)" color="primary" variant="subtle"
            >Root Console User</UBadge
          ><UBadge v-else color="neutral" variant="subtle">{{
            auth.hasScope("core:service-account.read") ? "Scoped Service Account" : "Not Visible"
          }}</UBadge></template
        >
        <template #lastSignedInAt-cell="cell">{{
          cell.row.original.lastSignedInAt?.toLocaleString() || "Never"
        }}</template>
        <template #actions-cell="cell"
          ><UButton
            label="Manage"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="editUser(cell.row.original.id)"
        /></template>
      </UTable>
    </UCard>
    <UCard
      v-if="controlPlane.selectedUser.value || auth.hasScope('console:user.write')"
      :title="controlPlane.selectedUser.value ? 'Manage Console User' : 'Add Console User'"
      description="Manage this Console User's identity and access."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UForm
        id="console-user-form"
        :state="userForm"
        :disabled="controlPlane.busy.value || !auth.hasScope('console:user.write')"
        class="space-y-5"
        @submit="controlPlane.selectedUser.value ? saveUser() : createUser()"
      >
        <UFormField label="Email" required
          ><UInput v-model="userForm.email" type="email" autocomplete="off" required class="w-full"
        /></UFormField>
        <UFormField v-if="!controlPlane.selectedUser.value" label="Initial Password" required
          ><UInput
            v-model="userForm.password"
            type="password"
            autocomplete="new-password"
            required
            class="w-full"
        /></UFormField>
        <USwitch
          v-if="controlPlane.selectedUser.value"
          v-model="userForm.isEnabled"
          label="Account Enabled"
          :disabled="isSelf || isLastRoot"
        />
        <UAlert
          v-if="isSelf || isLastRoot"
          color="info"
          title="Account Protection"
          :description="
            isSelf
              ? 'You cannot disable or delete your own Console User.'
              : 'The last enabled root Console User must retain access.'
          "
        />
        <UAlert
          v-if="linkedAccount"
          color="neutral"
          title="Linked Service Account"
          :description="linkedAccount.name"
        />
      </UForm>
      <template #footer>
        <UButton
          v-if="linkedAccount && auth.hasScope('core:service-account.read')"
          label="Manage Service Account"
          color="neutral"
          variant="outline"
          :to="{ path: '/control-plane/service-accounts', query: { account: linkedAccount.id } }"
        />
        <UButton
          v-if="
            auth.hasScope('console:user.write') &&
            controlPlane.selectedUser.value &&
            !isSelf &&
            !isLastRoot
          "
          label="Delete User"
          color="error"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="
            confirmation = {
              title: 'Delete Console User?',
              description:
                'This permanently removes the Console User, its linked Service Account, and its sessions. This cannot be undone.',
              action: () => controlPlane.deleteUser(controlPlane.selectedUser.value!.id),
            }
          "
        />
        <UButton
          v-if="controlPlane.selectedUser.value"
          label="Close"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="closeUser"
        />
        <UButton
          v-if="auth.hasScope('console:user.write')"
          type="submit"
          form="console-user-form"
          :label="controlPlane.selectedUser.value ? 'Save User' : 'Create User'"
          :disabled="controlPlane.busy.value"
        />
      </template>
    </UCard>
    <template v-if="controlPlane.selectedUser.value">
      <UCard
        v-if="auth.hasScope('console:user.write')"
        title="Reset Password"
        description="Set a new password and revoke this Console User's existing sessions."
        :ui="{ footer: 'flex justify-end' }"
      >
        <UForm
          id="console-password-form"
          :state="userForm"
          :disabled="controlPlane.busy.value"
          class="space-y-5"
          @submit="resetPassword"
        >
          <UFormField label="New Password" required
            ><UInput
              v-model="userForm.password"
              type="password"
              autocomplete="new-password"
              required
              class="w-full"
          /></UFormField>
        </UForm>
        <template #footer
          ><UButton
            type="submit"
            form="console-password-form"
            label="Reset Password"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
        /></template>
      </UCard>
      <UCard
        title="Active Sessions"
        description="Review devices and revoke access for this Console User."
        :ui="{ footer: 'flex justify-end' }"
      >
        <UTable
          :data="controlPlane.sessions.value"
          :columns="sessionColumns"
          :loading="controlPlane.busy.value"
          :ui="{ th: 'whitespace-nowrap' }"
        >
          <template #expiresAt-cell="cell">{{
            cell.row.original.expiresAt.toLocaleString()
          }}</template>
          <template #actions-cell="cell"
            ><UButton
              v-if="auth.hasScope('console:user.write')"
              label="Revoke"
              color="error"
              variant="ghost"
              :disabled="controlPlane.busy.value"
              @click="
                confirmation = {
                  title: 'Revoke Session?',
                  description: 'The device will need to sign in again.',
                  action: () =>
                    controlPlane.revokeSession(cell.row.original.userId, cell.row.original.id),
                }
              "
          /></template>
        </UTable>
        <template #footer>
          <UButton
            v-if="auth.hasScope('console:user.write')"
            label="Revoke All Sessions"
            color="error"
            variant="outline"
            :disabled="controlPlane.busy.value || !controlPlane.sessions.value.length"
            @click="
              confirmation = {
                title: 'Revoke All Sessions?',
                description:
                  'This signs the user out of all devices, including this browser if this is your account.',
                action: () => controlPlane.revokeSession(controlPlane.selectedUser.value!.id),
              }
            "
          />
        </template>
      </UCard>
    </template>
    <UModal
      :open="!!confirmation"
      :title="confirmation?.title"
      :description="confirmation?.description"
      :dismissible="!controlPlane.busy.value"
      @update:open="
        (open) => {
          if (!open) confirmation = null;
        }
      "
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmation = null" /><UButton
          label="Confirm"
          color="error"
          :loading="controlPlane.busy.value"
          @click="confirmAction"
      /></template>
    </UModal>
  </div>
</template>
