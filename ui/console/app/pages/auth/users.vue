<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { AuthControlPlaneModels as Models } from "layr-client";
import { authUserLabel, authUserLocked, resolveAuthUser } from "../../utils/auth-forms";
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
const search = reactive({ search: "", role: "" });
const creating = ref(false);
const viewing = ref(false);
const form = reactive({
  email: "",
  phone: "",
  password: "",
  role: "authenticated",
  emailVerified: false,
  phoneVerified: false,
  properties: "{}",
});
const formTemplateRef = useTemplateRef("userFormElement");
const confirmation = ref<{ action: "lock" | "unlock" | "delete"; user: Models.User } | null>(null);
const lockUntil = ref("");
const lockError = ref("");
const columns: TableColumn<Models.User>[] = [
  { id: "identity", header: "User" },
  { accessorKey: "role", header: "JWT Role" },
  { id: "status", header: "Access" },
  { accessorKey: "mfaEnabled", header: "MFA" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions", header: "" },
];
function newUser() {
  Object.assign(form, {
    email: "",
    phone: "",
    password: "",
    role: "authenticated",
    emailVerified: false,
    phoneVerified: false,
    properties: "{}",
  });
  authControlPlane.error.value = "";
  creating.value = true;
}
function validate(): FormError[] {
  return resolveAuthUser(form).errors;
}
async function createUser() {
  const { input, errors } = resolveAuthUser(form);
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  if (await authControlPlane.createUser(input)) {
    creating.value = false;
    form.password = "";
  }
}
async function details(id: string) {
  if (await authControlPlane.getUser(id)) viewing.value = true;
}
function confirm(action: "lock" | "unlock" | "delete", user: Models.User) {
  lockUntil.value = "";
  lockError.value = "";
  authControlPlane.error.value = "";
  viewing.value = false;
  confirmation.value = { action, user };
}
async function apply() {
  if (!confirmation.value) return;
  const { action, user } = confirmation.value;
  let result: boolean;
  if (action === "lock") {
    const date = lockUntil.value ? new Date(lockUntil.value) : undefined;
    if (date && (!Number.isFinite(date.getTime()) || date.getTime() <= Date.now())) {
      lockError.value = "Choose a future date and time.";
      return;
    }
    result = await authControlPlane.lockUser(user.id, date);
  } else if (action === "unlock") result = await authControlPlane.unlockUser(user.id);
  else result = await authControlPlane.deleteUser(user.id);
  if (result) confirmation.value = null;
}
function filter() {
  Object.assign(authControlPlane.filters, search);
  return authControlPlane.listUsers();
}
onMounted(() => {
  if (auth.hasScope("auth:user.read")) return authControlPlane.listUsers();
});
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
    <UAlert
      v-if="authControlPlane.notice.value"
      :title="authControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="Users"
      description="Find application users, review their account and verification status, manage sign-in access, or inspect their sessions. Console administrators and Service Accounts are managed separately."
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        :state="search"
        :disabled="authControlPlane.busy.value"
        class="space-y-5"
        @submit="filter"
      >
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Search Email or Phone"
            ><UInput
              v-model="search.search"
              placeholder="Search by email or phone…"
              icon="i-lucide-search"
              class="w-full" /></UFormField
          ><UFormField label="JWT Role Filter"
            ><UInput
              v-model="search.role"
              placeholder="Exact role; leave blank for all roles"
              class="w-full"
          /></UFormField>
        </div>
        <div class="flex flex-wrap justify-end gap-3">
          <UButton label="Apply Filters" type="submit" color="neutral" variant="outline" />
        </div>
      </UForm>
      <UTable
        :data="authControlPlane.users.value?.data || []"
        :columns="columns"
        :loading="authControlPlane.busy.value"
        :inert="authControlPlane.busy.value"
        caption="Application users and management actions."
        @select="(_event, row) => details(row.original.id)"
      >
        <template #identity-cell="{ row }"
          ><div class="space-y-2">
            <p class="font-medium text-highlighted">{{ authUserLabel(row.original) }}</p>
            <p class="text-xs text-muted">{{ row.original.id }}</p>
            <UBadge
              v-if="row.original.email"
              :label="row.original.emailVerifiedAt ? 'Email Verified' : 'Email Unverified'"
              :color="row.original.emailVerifiedAt ? 'success' : 'neutral'"
              variant="subtle"
            />
            <UBadge
              v-if="row.original.phone"
              :label="row.original.phoneVerifiedAt ? 'Phone Verified' : 'Phone Unverified'"
              :color="row.original.phoneVerifiedAt ? 'success' : 'neutral'"
              variant="subtle"
            />
            <UBadge
              v-if="row.original.isAnonymous"
              label="Anonymous"
              color="neutral"
              variant="subtle"
            /></div
        ></template>
        <template #status-cell="{ row }"
          ><UBadge
            :label="authUserLocked(row.original) ? 'Locked' : 'Unlocked'"
            :color="authUserLocked(row.original) ? 'warning' : 'neutral'"
            variant="subtle"
        /></template>
        <template #mfaEnabled-cell="{ row }">{{
          row.original.mfaEnabled ? "Enrolled" : "Not Enrolled"
        }}</template>
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #actions-cell="{ row }"
          ><div @click.stop>
            <UDropdownMenu
              :items="[
                { label: 'Details', onSelect: () => details(row.original.id) },
                {
                  label: 'Sessions',
                  to: { path: '/auth/sessions', query: { user_id: row.original.id } },
                },
                ...(auth.hasScope('auth:user.write')
                  ? [
                      {
                        label: authUserLocked(row.original) ? 'Unlock User' : 'Lock User',
                        onSelect: () =>
                          confirm(authUserLocked(row.original) ? 'unlock' : 'lock', row.original),
                      },
                      {
                        label: 'Delete User',
                        color: 'error' as const,
                        onSelect: () => confirm('delete', row.original),
                      },
                    ]
                  : []),
              ]"
              ><UButton
                square
                icon="i-lucide-ellipsis"
                aria-label="User Actions"
                color="neutral"
                variant="ghost"
            /></UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            :title="
              authControlPlane.busy.value
                ? 'Loading Users…'
                : authControlPlane.filters.search || authControlPlane.filters.role
                  ? 'No Matching Users'
                  : 'No Users Yet'
            "
            :description="
              authControlPlane.filters.search || authControlPlane.filters.role
                ? 'Try a different email, phone number, or role.'
                : 'Users appear here after signing up in your application. You can also create an account administratively.'
            "
            icon="i-lucide-users"
        /></template>
      </UTable>
      <template #footer>
        <span class="mr-auto text-sm text-muted"
          >Page {{ Math.floor(authControlPlane.offset.value / authControlPlane.limit) + 1 }} ·
          {{ authControlPlane.users.value?.data?.length || 0 }} users shown</span
        >
        <UButton
          label="Previous"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value || !authControlPlane.offset.value"
          @click="
            authControlPlane.listUsers(authControlPlane.offset.value - authControlPlane.limit)
          "
        />
        <UButton
          label="Next"
          color="neutral"
          variant="outline"
          :disabled="
            authControlPlane.busy.value ||
            (authControlPlane.users.value?.data?.length || 0) < authControlPlane.limit
          "
          @click="
            authControlPlane.listUsers(authControlPlane.offset.value + authControlPlane.limit)
          "
        />
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value"
          @click="authControlPlane.listUsers(authControlPlane.offset.value)"
        />
        <UButton
          v-if="auth.hasScope('auth:user.write')"
          label="New User"
          icon="i-lucide-user-plus"
          :disabled="authControlPlane.busy.value"
          @click="newUser"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="creating"
      title="Create User"
      description="Administrative provisioning does not run the public sign-up workflow or send verification messages. Only assert verification when you have independently verified the identity."
      :dismissible="!authControlPlane.busy.value"
      :close="{ disabled: authControlPlane.busy.value }"
      :ui="{ content: 'max-w-2xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          v-if="authControlPlane.error.value"
          title="Creation Failed"
          :description="authControlPlane.error.value"
          color="error"
        />
        <UForm
          id="auth-user-create"
          ref="userFormElement"
          :state="form"
          :validate="validate"
          :disabled="authControlPlane.busy.value"
          class="space-y-5"
          @submit="createUser"
        >
          <USeparator label="Identity" position="start" />
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField label="Email" name="email" description="Provide email, phone, or both."
              ><UInput v-model="form.email" type="email" class="w-full" /></UFormField
            ><UFormField
              label="Phone"
              name="phone"
              description="International E.164 format, for example +14155552671."
              ><UInput v-model="form.phone" type="tel" class="w-full"
            /></UFormField>
          </div>
          <UFormField
            label="Initial Password"
            name="password"
            description="Optional. Leave blank for an account without a password credential."
            ><UInput
              v-model="form.password"
              type="password"
              autocomplete="new-password"
              class="w-full"
          /></UFormField>
          <UFormField
            label="JWT Role"
            name="role"
            description="Application JWT role, not a console role or Service Account scope."
            required
            ><UInput v-model="form.role" class="w-full"
          /></UFormField>
          <USeparator label="Verification and Properties" position="start" />
          <UFormField name="emailVerified"
            ><USwitch
              v-model="form.emailVerified"
              label="Email Already Verified"
              description="Mark email ownership as independently verified."
          /></UFormField>
          <UFormField name="phoneVerified"
            ><USwitch
              v-model="form.phoneVerified"
              label="Phone Already Verified"
              description="Mark phone ownership as independently verified."
          /></UFormField>
          <UFormField
            label="Properties (JSON)"
            name="properties"
            description="Initial application metadata as a JSON object; never store credentials here."
            ><UTextarea v-model="form.properties" :rows="4" class="w-full"
          /></UFormField>
        </UForm>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value"
          @click="creating = false" /><UButton
          label="Create User"
          type="submit"
          form="auth-user-create"
          :loading="authControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      v-model:open="viewing"
      :title="
        authControlPlane.user.value ? authUserLabel(authControlPlane.user.value) : 'User Details'
      "
      description="Account profile, verification, and security status. Profile editing is not available in the administrative API. Credentials and MFA secrets are never displayed."
      :ui="{ content: 'max-w-2xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body
        ><template v-if="authControlPlane.user.value">
          <section class="space-y-5">
            <USeparator label="Identity" position="start" />
            <dl class="grid gap-4 text-sm sm:grid-cols-2">
              <div
                v-for="item in [
                  ['User ID', authControlPlane.user.value.id],
                  ['Email', authControlPlane.user.value.email || 'Not Set'],
                  ['Phone', authControlPlane.user.value.phone || 'Not Set'],
                  ['JWT Role', authControlPlane.user.value.role],
                  [
                    'Account Type',
                    authControlPlane.user.value.isAnonymous ? 'Anonymous' : 'Registered',
                  ],
                  ['MFA', authControlPlane.user.value.mfaEnabled ? 'Enrolled' : 'Not Enrolled'],
                  [
                    'Email Verified At',
                    authControlPlane.user.value.emailVerifiedAt?.toLocaleString() || 'Not Verified',
                  ],
                  [
                    'Phone Verified At',
                    authControlPlane.user.value.phoneVerifiedAt?.toLocaleString() || 'Not Verified',
                  ],
                  [
                    'Locked Until',
                    authControlPlane.user.value.lockedUntil?.toLocaleString() || 'Not Locked',
                  ],
                  ['Created At', authControlPlane.user.value.createdAt.toLocaleString()],
                  ['Updated At', authControlPlane.user.value.lastUpdatedAt.toLocaleString()],
                ]"
                :key="item[0]"
              >
                <dt class="text-muted">{{ item[0] }}</dt>
                <dd class="break-all">{{ item[1] }}</dd>
              </div>
            </dl>
          </section>
          <section class="space-y-5">
            <USeparator label="Application Properties" position="start" />
            <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{
              JSON.stringify(authControlPlane.user.value.properties || {}, null, 2)
            }}</pre>
          </section>
        </template></template
      >
      <template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="viewing = false" />
        <UButton
          v-if="authControlPlane.user.value"
          label="View Sessions"
          :to="{ path: '/auth/sessions', query: { user_id: authControlPlane.user.value.id } }"
          color="neutral"
          variant="outline"
          @click="viewing = false"
        />
        <UButton
          v-if="authControlPlane.user.value && auth.hasScope('auth:user.write')"
          :label="authUserLocked(authControlPlane.user.value) ? 'Unlock User' : 'Lock User'"
          color="neutral"
          variant="outline"
          @click="
            confirm(
              authUserLocked(authControlPlane.user.value) ? 'unlock' : 'lock',
              authControlPlane.user.value,
            )
          "
        />
      </template>
    </UModal>
    <UModal
      :open="!!confirmation"
      :title="
        confirmation
          ? `${confirmation.action === 'lock' ? 'Lock' : confirmation.action === 'unlock' ? 'Unlock' : 'Delete'} User?`
          : 'Confirm User Action'
      "
      :description="
        confirmation
          ? `${authUserLabel(confirmation.user)}: ${confirmation.action === 'lock' ? 'Block new authentication until the lock expires or is lifted. Refresh sessions are revoked; already issued access JWTs may remain valid until expiry.' : confirmation.action === 'unlock' ? 'Restore authentication access. Previously revoked sessions are not restored.' : 'Permanently delete this application user, sessions, identities, and passkeys. This cannot be undone.'}`
          : ''
      "
      :dismissible="!authControlPlane.busy.value"
      :close="{ disabled: authControlPlane.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) confirmation = null;
        }
      "
    >
      <template v-if="confirmation?.action === 'lock' || authControlPlane.error.value" #body>
        <UAlert
          v-if="authControlPlane.error.value"
          title="Action Failed"
          :description="authControlPlane.error.value"
          color="error"
        />
        <UFormField
          v-if="confirmation?.action === 'lock'"
          label="Lock Until (Local Time)"
          :error="lockError"
          description="Optional. Leave blank for the backend's long-term administrative lock (100 years)."
          ><UInput v-model="lockUntil" type="datetime-local" class="w-full"
        /></UFormField>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value"
          @click="confirmation = null" /><UButton
          :label="
            confirmation?.action === 'lock'
              ? 'Lock User'
              : confirmation?.action === 'unlock'
                ? 'Unlock User'
                : 'Delete User'
          "
          :color="confirmation?.action === 'delete' ? 'error' : 'primary'"
          :loading="authControlPlane.busy.value"
          @click="apply"
      /></template>
    </UModal>
  </div>
</template>
