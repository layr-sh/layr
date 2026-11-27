<script setup lang="ts">
import type { AuthControlPlaneModels as Models } from "layr-client";
import type { FormError, TableColumn } from "@nuxt/ui";
import { refDebounced } from "@vueuse/core";
import { authUserLabel } from "../../utils/auth-forms";
const auth = useAuth();
const client = useClient();
const route = useRoute();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
type UserOption = { id: string; label: string };
const userSearch = ref("");
const debouncedSearch = refDebounced(userSearch, 250);
const {
  data: directory,
  status: directoryStatus,
  error: directoryError,
  refresh: refreshUsers,
} = await useAsyncData(
  "auth:session-user-picker",
  () =>
    auth.hasScope("auth:user.read")
      ? client.value.auth.cp.users.list({ search: debouncedSearch.value.trim(), limit: 20 })
      : Promise.resolve({ data: [], count: 0, limit: 20, offset: 0 }),
  { watch: [debouncedSearch] },
);
const userOptions = computed<UserOption[]>(() =>
  (directory.value?.data ?? []).map((user) => ({
    id: user.id,
    label: user.isAnonymous ? "Guest account" : authUserLabel(user),
  })),
);
const chosenUser = ref<UserOption>();
const form = reactive({ userId: "" });
const selected = ref<Models.Session | null>(null);
const revoking = ref(false);
const formTemplateRef = useTemplateRef("sessionFormElement");
const columns: TableColumn<Models.Session>[] = [
  { accessorKey: "id", header: "Session" },
  { accessorKey: "ipAddress", header: "IP Address" },
  { accessorKey: "clientId", header: "OIDC Client" },
  { accessorKey: "expiresAt", header: "Expires" },
  { id: "status", header: "Status" },
  { id: "actions", header: "" },
];
function validate(): FormError[] {
  return /^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$/i.test(form.userId.trim())
    ? []
    : [{ name: "userId", message: "Choose an application user." }];
}
async function load() {
  const errors = validate();
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  formTemplateRef.value?.setErrors([]);
  selected.value = null;
  if (await authControlPlane.listSessions(form.userId.trim())) {
    const user = authControlPlane.user.value!;
    chosenUser.value = {
      id: user.id,
      label: user.isAnonymous ? "Guest account" : authUserLabel(user),
    };
  }
}
async function chooseUser(value: UserOption | undefined) {
  if (!value) return;
  form.userId = value.id;
  await load();
}
async function revoke() {
  if (!authControlPlane.user.value) return;
  if (await authControlPlane.revokeSessions(authControlPlane.user.value.id)) revoking.value = false;
}
watch(
  () => route.query.user_id,
  (userId) => {
    if (auth.hasScope("auth:user.read") && typeof userId === "string") {
      form.userId = userId;
      return load();
    }
  },
  { immediate: true },
);
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
      title="Sessions"
      description="Review a user's devices and refresh sessions, or revoke their sessions to require sign-in again. Expired records may still appear here. This is a per-user view, not a global list of active sessions."
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        ref="sessionFormElement"
        :state="form"
        :validate="validate"
        :disabled="authControlPlane.busy.value || !auth.hasScope('auth:user.read')"
        class="space-y-5"
        @submit="load"
      >
        <UFormField label="Application User" name="userId" required>
          <USelectMenu
            v-model="chosenUser"
            v-model:search-term="userSearch"
            :items="userOptions"
            :loading="directoryStatus === 'pending'"
            :ignore-filter="true"
            :search-input="{ placeholder: 'Search by email or phone…' }"
            placeholder="Choose an application user…"
            class="w-full"
            @update:model-value="chooseUser"
          >
            <template #item-label="{ item }">
              <span class="block truncate">{{ item.label }}</span>
              <span class="block truncate text-xs text-muted">{{ item.id }}</span>
            </template>
            <template #empty>No matching users. Try another email or phone number.</template>
          </USelectMenu>
        </UFormField>
        <UAlert
          v-if="directoryError"
          title="User Search Failed"
          :description="directoryError.message"
          color="error"
        >
          <template #actions
            ><UButton
              label="Retry User Search"
              color="neutral"
              variant="outline"
              @click="refreshUsers()"
          /></template>
        </UAlert>
        <div class="flex flex-wrap justify-end gap-3">
          <UButton label="Browse Users" to="/auth/users" color="neutral" variant="outline" />
          <UButton
            label="Refresh Sessions"
            type="submit"
            color="neutral"
            variant="outline"
            :disabled="!form.userId"
          />
        </div>
      </UForm>
      <template v-if="authControlPlane.user.value && authControlPlane.sessions.value">
        <USeparator
          :label="`Sessions for ${authUserLabel(authControlPlane.user.value)}`"
          position="start"
        />
        <UTable
          :data="authControlPlane.sessions.value.data || []"
          :columns="columns"
          :inert="authControlPlane.busy.value"
          caption="Application refresh-session records."
          @select="(_event, row) => (selected = row.original)"
        >
          <template #ipAddress-cell="{ row }">{{
            row.original.ipAddress || "Not Recorded"
          }}</template>
          <template #clientId-cell="{ row }">{{ row.original.clientId || "Direct Auth" }}</template>
          <template #expiresAt-cell="{ row }">{{
            row.original.expiresAt.toLocaleString()
          }}</template>
          <template #status-cell="{ row }"
            ><UBadge
              :label="row.original.expiresAt.getTime() > Date.now() ? 'Unexpired' : 'Expired'"
              color="neutral"
              variant="subtle"
          /></template>
          <template #actions-cell="{ row }"
            ><div @click.stop>
              <UDropdownMenu
                :items="[{ label: 'Details', onSelect: () => (selected = row.original) }]"
                ><UButton
                  square
                  icon="i-lucide-ellipsis"
                  aria-label="Session Actions"
                  color="neutral"
                  variant="ghost"
              /></UDropdownMenu></div
          ></template>
          <template #empty
            ><UEmpty
              title="No Session Records"
              description="This user has no stored refresh sessions."
              icon="i-lucide-monitor"
          /></template>
        </UTable>
      </template>
      <UEmpty
        v-else
        title="Choose an Application User"
        description="Search by email or phone above. Selecting a user loads their sessions automatically."
        icon="i-lucide-user-search"
      />
      <template #footer
        ><UButton
          v-if="auth.hasScope('auth:user.write')"
          label="Revoke All Sessions"
          color="error"
          variant="outline"
          :disabled="
            authControlPlane.busy.value ||
            !authControlPlane.user.value ||
            !authControlPlane.sessions.value?.data?.length
          "
          @click="
            authControlPlane.error.value = '';
            revoking = true;
          "
      /></template>
    </UCard>
    <UModal
      :open="!!selected"
      title="Session Details"
      description="Refresh-session metadata only. Access tokens, refresh tokens, and token hashes are never shown."
      :ui="{ content: 'max-w-2xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) selected = null;
        }
      "
    >
      <template #body
        ><section v-if="selected" class="space-y-5">
          <dl class="grid gap-4 text-sm sm:grid-cols-2">
            <div
              v-for="item in [
                ['Session ID', selected.id],
                ['User ID', selected.userId],
                ['OIDC Client', selected.clientId || 'Direct Auth'],
                ['IP Address', selected.ipAddress || 'Not Recorded'],
                ['User Agent', selected.userAgent || 'Not Recorded'],
                ['Created At', selected.createdAt.toLocaleString()],
                ['Expires At', selected.expiresAt.toLocaleString()],
              ]"
              :key="item[0]"
            >
              <dt class="text-muted">{{ item[0] }}</dt>
              <dd class="break-all">{{ item[1] }}</dd>
            </div>
          </dl>
        </section></template
      >
      <template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="selected = null"
      /></template>
    </UModal>
    <UModal
      v-model:open="revoking"
      title="Revoke All Sessions?"
      :description="`Invalidate every stored refresh session for ${authControlPlane.user.value ? authUserLabel(authControlPlane.user.value) : 'this user'} and dispatch configured OIDC back-channel sign-out notifications. Existing access JWTs may remain usable until they expire.`"
      :dismissible="!authControlPlane.busy.value"
      :close="{ disabled: authControlPlane.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template v-if="authControlPlane.error.value" #body
        ><UAlert
          title="Revocation Failed"
          :description="authControlPlane.error.value"
          color="error"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="authControlPlane.busy.value"
          @click="revoking = false" /><UButton
          label="Revoke Sessions"
          color="error"
          :loading="authControlPlane.busy.value"
          @click="revoke"
      /></template>
    </UModal>
  </div>
</template>
