<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { CoreControlPlaneModels } from "layr-client";

const auth = useAuth();
const route = useRoute();
const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);
const accountForm = reactive({
  name: "",
  description: "",
  scopes: [] as string[],
  allowedIps: "",
  expiresAt: "",
  isEnabled: true,
});
const confirmingDelete = ref(false);
const scopeOptions = computed(() =>
  [
    ...new Set([
      "*",
      "core:service-account.read",
      "core:service-account.write",
      "core:event.read",
      "core:config.read",
      "core:config.write",
      "core:event-hook.read",
      "core:event-hook.write",
      "data:schema.read",
      "data:schema.write",
      "data:query.read",
      "data:query.write",
      "data:cache.read",
      "data:cache.write",
      "data:config.read",
      "data:config.write",
      "auth:user.read",
      "auth:user.write",
      "auth:config.read",
      "auth:config.write",
      "storage:bucket.read",
      "storage:bucket.write",
      "storage:object.read",
      "storage:object.write",
      "storage:config.read",
      "storage:config.write",
      "tasks:job.read",
      "tasks:job.write",
      "tasks:execution.read",
      "tasks:execution.write",
      "tasks:config.read",
      "tasks:config.write",
      "tasks:stats.read",
      "image:config.read",
      "image:config.write",
      "image:preset.read",
      "image:preset.write",
      "image:sign.write",
      "image:stats.read",
      "function:endpoint.read",
      "function:endpoint.write",
      "function:endpoint.deploy",
      "function:endpoint.invoke",
      "function:config.read",
      "function:config.write",
      "function:stats.read",
      "console:user.read",
      "console:user.write",
      "console:config.read",
      "console:config.write",
      ...(auth.state.value?.scopes || []),
      ...controlPlane.accounts.value.flatMap((account) => account.scopes || []),
      ...accountForm.scopes,
    ]),
  ].sort(),
);
const columns: TableColumn<CoreControlPlaneModels.ServiceAccount>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "keyPrefix", header: "Key Prefix" },
  { id: "owner", header: "Ownership" },
  { id: "access", header: "Access" },
  { accessorKey: "isEnabled", header: "Status" },
  { accessorKey: "lastUsedAt", header: "Last Used" },
  { id: "actions" },
];
function lines(value: string) {
  return value
    .split(/[\n,]/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}
function clearAccount() {
  controlPlane.selectedAccount.value = null;
  Object.assign(accountForm, {
    name: "",
    description: "",
    scopes: [],
    allowedIps: "",
    expiresAt: "",
    isEnabled: true,
  });
}
watch(controlPlane.selectedAccount, () => {
  if (!controlPlane.selectedAccount.value) return;
  Object.assign(accountForm, {
    name: controlPlane.selectedAccount.value.name,
    description: controlPlane.selectedAccount.value.description || "",
    scopes: [...(controlPlane.selectedAccount.value.scopes || [])],
    allowedIps: (controlPlane.selectedAccount.value.allowedIps || []).join("\n"),
    expiresAt: controlPlane.selectedAccount.value.expiresAt?.toISOString().slice(0, -1) || "",
    isEnabled: controlPlane.selectedAccount.value.isEnabled,
  });
});
async function editAccount(accountId: string) {
  await controlPlane.selectAccount(accountId);
}
async function saveAccount() {
  if (!accountForm.scopes.length) {
    controlPlane.error.value = "At least one explicit scope is required.";
    return;
  }
  const input = {
    name: accountForm.name.trim(),
    description: accountForm.description,
    scopes: accountForm.scopes,
    allowedIps: lines(accountForm.allowedIps),
    ...(accountForm.expiresAt ? { expiresAt: new Date(`${accountForm.expiresAt}Z`) } : {}),
  };
  if (
    await (controlPlane.selectedAccount.value
      ? controlPlane.updateAccount(controlPlane.selectedAccount.value.id, {
          ...input,
          isEnabled: accountForm.isEnabled,
        })
      : controlPlane.createAccount(input))
  )
    clearAccount();
}
async function deleteAccount() {
  if (
    controlPlane.selectedAccount.value &&
    (await controlPlane.deleteAccount(controlPlane.selectedAccount.value.id))
  ) {
    confirmingDelete.value = false;
    clearAccount();
  }
}
onMounted(async () => {
  await controlPlane.load("service-accounts");
  if (typeof route.query.account === "string") await editAccount(route.query.account);
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
      v-if="controlPlane.secretKey.value"
      title="Save Your Secret Key"
      description="This key is shown only once. Store it securely; it will not be available again."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UInput
        :model-value="controlPlane.secretKey.value"
        readonly
        aria-label="New Service Account Secret Key"
        class="w-full"
      />
      <template #footer
        ><UButton
          label="I Have Saved the Key"
          color="neutral"
          variant="outline"
          @click="controlPlane.secretKey.value = ''"
      /></template>
    </UCard>
    <UCard
      title="Service Accounts"
      description="Service Accounts authorize control-plane access. They may belong to a Console User or be standalone credentials."
    >
      <UTable
        :data="controlPlane.accounts.value"
        :columns="columns"
        :loading="controlPlane.busy.value"
      >
        <template #isEnabled-cell="cell"
          ><UBadge :color="cell.row.original.isEnabled ? 'success' : 'neutral'" variant="subtle">{{
            cell.row.original.isEnabled ? "Enabled" : "Disabled"
          }}</UBadge></template
        >
        <template #owner-cell="cell"
          ><UBadge color="neutral" variant="subtle">{{
            cell.row.original.consoleUserId ? "Console User" : "Standalone"
          }}</UBadge></template
        >
        <template #access-cell="cell"
          ><UBadge
            :color="cell.row.original.scopes?.includes('*') ? 'primary' : 'neutral'"
            variant="subtle"
            >{{ cell.row.original.scopes?.includes("*") ? "Wildcard (*)" : "Scoped" }}</UBadge
          ></template
        >
        <template #lastUsedAt-cell="cell">{{
          cell.row.original.lastUsedAt?.toLocaleString() || "Never"
        }}</template>
        <template #actions-cell="cell"
          ><UButton
            label="Manage"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="editAccount(cell.row.original.id)"
        /></template>
      </UTable>
    </UCard>
    <UCard
      v-if="controlPlane.selectedAccount.value || auth.hasScope('core:service-account.write')"
      :title="
        controlPlane.selectedAccount.value ? 'Manage Service Account' : 'Create Service Account'
      "
      description="Configure credentials, scopes, and network access."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UAlert
        v-if="controlPlane.selectedAccount.value?.consoleUserId"
        color="info"
        title="Console User Owned Service Account"
        description="This account belongs to a Console User. Delete the Console User to remove its account."
      />
      <UForm
        id="service-account-form"
        :state="accountForm"
        :disabled="controlPlane.busy.value || !auth.hasScope('core:service-account.write')"
        class="space-y-5"
        @submit="saveAccount"
      >
        <UFormField label="Name" required
          ><UInput v-model="accountForm.name" required class="w-full"
        /></UFormField>
        <UFormField label="Description"
          ><UTextarea v-model="accountForm.description" class="w-full"
        /></UFormField>
        <UFormField
          label="Scopes"
          description="Select or enter scopes. Use least-privilege scopes; * grants full platform access."
          required
        >
          <USelectMenu
            v-model="accountForm.scopes"
            :items="scopeOptions"
            multiple
            create-item
            required
            class="w-full"
            placeholder="Select Scopes"
            @create="(scope) => accountForm.scopes.push(scope)"
          />
        </UFormField>
        <UFormField
          label="Allowed IP Addresses"
          description="One address or CIDR per line. Empty allows any address."
          ><UTextarea v-model="accountForm.allowedIps" class="w-full"
        /></UFormField>
        <UFormField
          label="Expiration (UTC)"
          :description="
            controlPlane.selectedAccount.value
              ? 'An existing expiration cannot be removed here.'
              : 'Optional. Leave empty for no expiration.'
          "
          ><UInput
            v-model="accountForm.expiresAt"
            type="datetime-local"
            step="0.001"
            :required="!!controlPlane.selectedAccount.value?.expiresAt"
            class="w-full"
        /></UFormField>
        <USwitch
          v-if="controlPlane.selectedAccount.value"
          v-model="accountForm.isEnabled"
          label="Account Enabled"
        />
      </UForm>
      <template #footer>
        <UButton
          v-if="
            controlPlane.selectedAccount.value?.consoleUserId && auth.hasScope('console:user.read')
          "
          label="View Console User"
          color="neutral"
          variant="outline"
          :to="{
            path: '/control-plane/users',
            query: { user: controlPlane.selectedAccount.value.consoleUserId },
          }"
        />
        <UButton
          v-if="
            controlPlane.selectedAccount.value &&
            !controlPlane.selectedAccount.value.consoleUserId &&
            auth.hasScope('core:service-account.write')
          "
          label="Delete Account"
          color="error"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDelete = true"
        />
        <UButton
          v-if="controlPlane.selectedAccount.value"
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="clearAccount"
        />
        <UButton
          v-if="auth.hasScope('core:service-account.write')"
          type="submit"
          form="service-account-form"
          :label="controlPlane.selectedAccount.value ? 'Save Account' : 'Create Account'"
          :disabled="controlPlane.busy.value"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="confirmingDelete"
      title="Delete Service Account?"
      description="Its secret key will immediately stop working. This cannot be undone."
      :dismissible="!controlPlane.busy.value"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Account"
          color="error"
          :loading="controlPlane.busy.value"
          @click="deleteAccount"
      /></template>
    </UModal>
  </div>
</template>
