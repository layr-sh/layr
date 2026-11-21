<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type useAuthControlPlane from "../../composables/use-auth-control-plane";
import {
  authRecordFields,
  authProviderTypes,
  authRecordIdentity,
  authRecordRows,
  blankAuthRecord,
  resolveAuthRecord,
  updateAuthRecord,
  type AuthRecordKind,
} from "../../utils/auth-records";
const props = defineProps<{
  control: ReturnType<typeof useAuthControlPlane>;
  kind: AuthRecordKind;
  title: string;
  description: string;
}>();
const { error } = props.control;
const auth = useAuth();
const client = useClient();
const search = ref("");
const { copy, copied } = useClipboard({ copiedDuring: 2000 });
const editing = ref(false);
const editingKey = ref("");

const providerAdvanced = ref(false);
const form = ref<Record<string, unknown>>({});
const formTemplateRef = useTemplateRef("recordFormElement");
type RecordRow = ReturnType<typeof authRecordRows>[number];
const confirming = ref<RecordRow | null>(null);
const rows = computed(() =>
  (props.control.config.value ? authRecordRows(props.kind, props.control.config.value) : []).filter(
    (row) => `${row.name} ${row.detail}`.toLowerCase().includes(search.value.trim().toLowerCase()),
  ),
);
const providerType = computed(() => {
  const type = String(form.value.preset || form.value.key || "custom");
  return authProviderTypes.some((provider) => provider.value === type) ? type : "custom";
});
const providerFields = computed(() =>
  authRecordFields.provider
    .filter((field) => ["key", "clientId", "clientSecret", "enabled"].includes(field.name))
    .map((field) => ({
      ...field,
      disabled: field.name === "key" && (providerType.value !== "custom" || !!editingKey.value),
      description:
        field.name === "clientSecret" && providerType.value === "apple"
          ? "Use the client-secret JWT generated for Sign in with Apple, not your Apple private key. Replace it before it expires; Layr does not generate or rotate it. Leave blank to preserve the saved value."
          : field.description,
    })),
);
const providerCustomFields = computed(() =>
  authRecordFields.provider.filter((field) =>
    [
      "authUrl",
      "tokenUrl",
      "userinfoUrl",
      "idAttribute",
      "emailAttribute",
      "nameAttribute",
      "avatarAttribute",
    ].includes(field.name),
  ),
);
const providerAdvancedFields = computed(() =>
  authRecordFields.provider.filter((field) =>
    ["scope", "responseType", "responseMode"].includes(field.name),
  ),
);
const fieldSections = computed(() => {
  const fields = authRecordFields[props.kind];

  if (props.kind === "client")
    return [
      { label: "Application Details", fields: fields.slice(0, 4) },
      { label: "Callback URLs and Scopes", fields: fields.slice(4, 7) },
      { label: "Federated Sign-Out", fields: fields.slice(7) },
    ];
  return [{ label: "API Details", fields }];
});
const recordLabel = computed(
  () => ({ provider: "Identity Provider", client: "Application", resource: "API" })[props.kind],
);
const columns: TableColumn<RecordRow>[] = [
  { accessorKey: "name", header: "Name" },
  {
    accessorKey: "detail",
    header:
      props.kind === "resource"
        ? "Audience"
        : props.kind === "provider"
          ? "Provider ID"
          : "Client ID",
  },
  { accessorKey: "status", header: props.kind === "client" ? "Application Type" : "Status" },
  { id: "actions", header: "" },
];

function open(row?: RecordRow) {
  providerAdvanced.value = false;
  form.value = blankAuthRecord(props.kind);
  if (row)
    Object.assign(
      form.value,
      JSON.parse(JSON.stringify(row.value)),
      props.kind === "provider" ? { key: row.key } : {},
    );
  editingKey.value = row?.key || "";
  if (props.kind === "provider" && !row) form.value.preset = "custom";
  if (props.kind === "provider" && providerType.value === "custom" && row)
    providerAdvanced.value = providerAdvancedFields.value.some((field) => !!form.value[field.name]);
  error.value = "";
  editing.value = true;
}
function validate() {
  return props.control.config.value
    ? resolveAuthRecord(props.kind, form.value, props.control.config.value, editingKey.value).errors
    : [];
}
async function save() {
  if (!props.control.config.value) return;
  const { input, errors } = resolveAuthRecord(
    props.kind,
    form.value,
    props.control.config.value,
    editingKey.value,
  );
  if (errors.length) {
    if (props.kind === "provider") providerAdvanced.value = true;
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  if (
    await props.control.saveConfig(
      updateAuthRecord(props.kind, props.control.config.value, input, editingKey.value),
    )
  ) {
    editing.value = false;
    form.value.clientSecret = "";
  }
}
function confirm(row: RecordRow) {
  error.value = "";
  confirming.value = row;
}
async function remove() {
  if (!props.control.config.value || !confirming.value) return;
  const row = confirming.value;
  const input = { ...row.value, key: row.key, enabled: false };
  if (
    await props.control.saveConfig(
      updateAuthRecord(props.kind, props.control.config.value, input, row.key, true),
    )
  )
    confirming.value = null;
}
</script>
<template>
  <div class="space-y-6" :aria-busy="control.busy.value">
    <UAlert
      v-if="control.error.value"
      title="Request Failed"
      :description="control.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="control.notice.value"
      :title="control.notice.value"
      color="success"
      role="status"
    />
    <UCard
      :title="title"
      :description="description"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UFormField label="Search">
        <UInput
          v-model="search"
          placeholder="Search by name or identifier…"
          icon="i-lucide-search"
          class="w-full"
        />
      </UFormField>
      <UTable
        :data="rows"
        :columns="columns"
        :loading="control.busy.value"
        :inert="control.busy.value"
        :caption="`${title} and management actions.`"
        @select="(_event, row) => open(row.original)"
      >
        <template #status-cell="{ row }"
          ><UBadge :label="row.original.status" color="neutral" variant="subtle"
        /></template>
        <template #actions-cell="{ row }"
          ><div @click.stop>
            <UDropdownMenu
              :items="[
                {
                  label:
                    kind === 'provider' && auth.hasScope('auth:config.write')
                      ? 'Configure'
                      : 'Details',
                  onSelect: () => open(row.original),
                },
                ...(auth.hasScope('auth:config.write') &&
                (kind !== 'provider' || row.original.status === 'Enabled')
                  ? [
                      {
                        label: kind === 'provider' ? 'Disable Provider' : `Delete ${recordLabel}`,
                        color: 'error' as const,
                        onSelect: () => confirm(row.original),
                      },
                    ]
                  : []),
              ]"
              ><UButton
                aria-label="Auth Record Actions"
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
            /></UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            :title="control.busy.value ? 'Loading Auth Configuration…' : `No ${title}`"
            :description="
              control.config.value
                ? search
                  ? 'Try a different name or identifier.'
                  : `Create your first ${recordLabel.toLowerCase()} to get started.`
                : 'Configuration is unavailable.'
            "
            icon="i-lucide-key-round"
        /></template>
      </UTable>
      <template #footer
        ><UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="control.busy.value"
          @click="control.loadConfig()" /><UButton
          v-if="auth.hasScope('auth:config.write')"
          :label="kind === 'provider' ? 'Add Custom Provider' : `New ${recordLabel}`"
          :disabled="control.busy.value || !control.config.value"
          @click="open()"
      /></template>
    </UCard>
    <UModal
      v-model:open="editing"
      :title="
        kind === 'provider'
          ? editingKey
            ? `${authProviderTypes.find((provider) => provider.value === providerType)!.label} Settings`
            : 'Add Custom Provider'
          : editingKey
            ? `${recordLabel} Details`
            : `New ${recordLabel}`
      "
      :description="
        kind === 'provider'
          ? 'Connect an external identity provider. Leave an existing secret blank to keep it unchanged.'
          : kind === 'client'
            ? 'Configure an OpenID Connect client and its allowed callbacks. Client IDs cannot be changed after creation.'
            : 'Define the audience and permissions for a machine-to-machine API. Credentials are managed through Service Accounts.'
      "
      :dismissible="!control.busy.value"
      :close="{ disabled: control.busy.value }"
      :ui="{ content: 'max-w-3xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          v-if="control.error.value"
          title="Save Failed"
          :description="control.error.value"
          color="error"
        />
        <UForm
          id="auth-record"
          ref="recordFormElement"
          :state="form"
          :validate="validate"
          :disabled="control.busy.value || !auth.hasScope('auth:config.write')"
          class="space-y-5"
          @submit="save"
          @error="kind === 'provider' && (providerAdvanced = true)"
        >
          <template v-if="kind === 'provider'">
            <AuthFields :state="form" :fields="providerFields" />
            <UFormField
              label="Callback URL"
              description="Add this exact URL to the allowed redirect URLs in your external provider's settings."
            >
              <UInput
                :model-value="form.key ? `${client.endpoint}/v1/oauth/${form.key}/callback` : ''"
                placeholder="Enter a Provider ID to generate the callback URL"
                readonly
                class="w-full"
              />
            </UFormField>
            <div class="flex flex-wrap gap-3">
              <UButton
                :label="copied ? 'Copied' : 'Copy Callback URL'"
                :icon="copied ? 'i-lucide-check' : 'i-lucide-copy'"
                color="neutral"
                variant="outline"
                :disabled="!form.key"
                @click="copy(`${client.endpoint}/v1/oauth/${form.key}/callback`)"
              />
            </div>
            <section v-if="providerType === 'custom'" class="space-y-5">
              <header class="space-y-2">
                <USeparator label="Endpoints and Profile Mapping" position="start" />
                <p class="text-sm text-muted">
                  Provide authorization and token endpoints. Profile fields are read from the
                  user-info response or token claims; the default user ID field is sub, with id as a
                  fallback.
                </p>
              </header>
              <AuthFields :state="form" :fields="providerCustomFields" />
            </section>
            <UCollapsible
              v-if="providerType === 'custom'"
              v-model:open="providerAdvanced"
              class="space-y-5"
            >
              <UButton
                label="Advanced Settings"
                :trailing-icon="providerAdvanced ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"
                color="neutral"
                variant="outline"
              />
              <template #content>
                <div class="space-y-5">
                  <p class="text-sm text-muted">
                    Configure additional scopes and protocol settings required by your custom
                    provider.
                  </p>
                  <AuthFields :state="form" :fields="providerAdvancedFields" />
                </div>
              </template>
            </UCollapsible>
          </template>
          <template v-else>
            <section v-for="section in fieldSections" :key="section.label" class="space-y-5">
              <USeparator v-if="fieldSections.length > 1" :label="section.label" position="start" />
              <AuthFields
                :state="form"
                :fields="section.fields"
                :disabled-key="editingKey ? authRecordIdentity[kind] : ''"
              />
            </section>
          </template>
        </UForm>
      </template>
      <template #footer
        ><UButton
          label="Close"
          color="neutral"
          variant="outline"
          :disabled="control.busy.value"
          @click="editing = false" /><UButton
          v-if="auth.hasScope('auth:config.write')"
          :label="`Save ${recordLabel}`"
          type="submit"
          form="auth-record"
          :loading="control.busy.value"
      /></template>
    </UModal>
    <UModal
      :open="!!confirming"
      :title="kind === 'provider' ? 'Disable Identity Provider?' : `Delete ${recordLabel}?`"
      :description="`${confirming?.name}: ${kind === 'provider' ? 'Prevent new sign-ins through this provider. The configuration and saved secret are retained.' : 'Remove this registration from Auth configuration. Previously issued access tokens may remain valid until expiry.'}`"
      :dismissible="!control.busy.value"
      :close="{ disabled: control.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) confirming = null;
        }
      "
    >
      <template v-if="control.error.value" #body
        ><UAlert title="Action Failed" :description="control.error.value" color="error"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="control.busy.value"
          @click="confirming = null" /><UButton
          :label="kind === 'provider' ? 'Disable Provider' : `Delete ${recordLabel}`"
          color="error"
          :loading="control.busy.value"
          @click="remove"
      /></template>
    </UModal>
  </div>
</template>
