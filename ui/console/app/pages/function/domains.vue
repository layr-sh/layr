<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";
import { validFunctionDomain } from "../../utils/function-forms";
const auth = useAuth();
const attachingRoute = ref(false);
const client = useClient();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const domainId = ref("");
const endpointId = ref("");
const selectedRouteId = ref("");
const editing = ref(false);
const confirmingDelete = ref(false);
const confirmingRemove = ref(false);
const domainFormTemplateRef = useTemplateRef("domainFormElement");
const routeFormTemplateRef = useTemplateRef("routeFormElement");
const domainForm = reactive({ domain: "", tlsEnabled: true, tlsAcmeEmail: "" });
const routeForm = reactive({ customDomainId: "", pathPrefix: "/" });
const verifiedDomains = computed(() =>
  (functionControlPlane.domains.value?.data || []).filter((domain) => domain.status === "active"),
);
const domainColumns: TableColumn<FunctionControlPlaneModels.CustomDomain>[] = [
  { accessorKey: "domain", header: "Domain" },
  { accessorKey: "status", header: "DNS Status" },
  { accessorKey: "tlsEnabled", header: "TLS" },
  { accessorKey: "tlsStatus", header: "TLS Status" },
  { id: "actions", header: "" },
];
const routeColumns: TableColumn<FunctionControlPlaneModels.CustomDomainRoute>[] = [
  { accessorKey: "domain", header: "Domain" },
  { accessorKey: "pathPrefix", header: "Path Prefix" },
  { id: "actions", header: "" },
];
const dnsColumns = [
  { accessorKey: "type", header: "Type" },
  { accessorKey: "name", header: "Name" },
  { accessorKey: "target", header: "Target" },
  { accessorKey: "status", header: "Status" },
  { accessorKey: "comment", header: "Instructions" },
];
function validateDomain(): FormError[] {
  const errors: FormError[] = [];
  if (!validFunctionDomain(domainForm.domain.trim()))
    errors.push({
      name: "domain",
      message: "Enter a hostname such as api.example.com, without a scheme or path.",
    });
  if (domainForm.tlsAcmeEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(domainForm.tlsAcmeEmail))
    errors.push({
      name: "tlsAcmeEmail",
      message: "Enter a valid ACME email or leave blank to use the service default.",
    });
  return errors;
}
function newDomain() {
  domainId.value = "";
  functionControlPlane.selectedDomain.value = null;
  Object.assign(domainForm, { domain: "", tlsEnabled: true, tlsAcmeEmail: "" });
  editing.value = true;
}
async function editDomain() {
  if (!domainId.value) return;
  if (await functionControlPlane.selectDomain(domainId.value)) {
    const domain = functionControlPlane.selectedDomain.value!;
    Object.assign(domainForm, {
      domain: domain.domain,
      tlsEnabled: domain.tlsEnabled,
      tlsAcmeEmail: domain.tlsAcmeEmail || "",
    });
    editing.value = true;
  }
}
async function saveDomain() {
  const errors = validateDomain();
  if (errors.length) {
    domainFormTemplateRef.value?.setErrors(errors);
    return;
  }
  if (
    await functionControlPlane.saveDomain(
      { ...domainForm, domain: domainForm.domain.trim().toLowerCase() },
      domainId.value,
    )
  ) {
    domainId.value = functionControlPlane.selectedDomain.value!.id;
    editing.value = false;
  }
}
async function verifyDomain() {
  if (!domainId.value) return;
  await functionControlPlane.verifyDomain(domainId.value);
}
async function deleteDomain() {
  if (!domainId.value) return;
  if (await functionControlPlane.deleteDomain(domainId.value)) {
    domainId.value = "";
    editing.value = false;
    confirmingDelete.value = false;
  }
}
async function chooseEndpoint(id: string) {
  endpointId.value = id;
  selectedRouteId.value = "";
  if (!id) {
    functionControlPlane.selectedEndpoint.value = null;
    functionControlPlane.routes.value = null;
    return;
  }
  await functionControlPlane.selectEndpoint(id, "routes");
}
function validateRoute(): FormError[] {
  const errors: FormError[] = [];
  if (!verifiedDomains.value.some((domain) => domain.id === routeForm.customDomainId))
    errors.push({ name: "customDomainId", message: "Choose a DNS-verified domain." });
  if (
    !routeForm.pathPrefix.startsWith("/") ||
    routeForm.pathPrefix.length > 255 ||
    /[\s?#]/.test(routeForm.pathPrefix)
  )
    errors.push({
      name: "pathPrefix",
      message:
        "Use a path starting with /, at most 255 characters, without spaces, a query, or a fragment.",
    });
  return errors;
}
async function attachRoute() {
  if (!functionControlPlane.selectedEndpoint.value) return;
  const errors = validateRoute();
  if (errors.length) {
    routeFormTemplateRef.value?.setErrors(errors);
    return;
  }
  await functionControlPlane.attachRoute(
    endpointId.value,
    routeForm.customDomainId,
    routeForm.pathPrefix,
  );
}
async function deleteRoute() {
  if (!selectedRouteId.value) return;
  if (await functionControlPlane.deleteRoute(endpointId.value, selectedRouteId.value)) {
    selectedRouteId.value = "";
    confirmingRemove.value = false;
  }
}
onMounted(async () => {
  if (auth.hasScope("function:endpoint.read")) {
    await functionControlPlane.load("domains");
    await functionControlPlane.load("endpoints");
  }
});
</script>
<template>
  <div class="space-y-6" :aria-busy="functionControlPlane.busy.value">
    <UAlert
      v-if="functionControlPlane.error.value"
      title="Request Failed"
      :description="functionControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="functionControlPlane.notice.value"
      :title="functionControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="Custom Domains"
      description="Register a hostname, follow its DNS instructions, then verify it before attaching routes."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="functionControlPlane.domains.value?.data || []"
        :columns="domainColumns"
        :loading="functionControlPlane.busy.value"
        :get-row-id="(domain) => domain.id"
        caption="Custom domains. DNS, TLS, and domain management actions."
        @select="
          (_event, row) => {
            domainId = row.original.id;
            editDomain();
          }
        "
      >
        <template #status-cell="{ row }"
          ><UBadge
            :label="row.original.status"
            :color="row.original.status === 'active' ? 'success' : 'warning'"
            variant="subtle"
        /></template>
        <template #tlsEnabled-cell="{ row }">{{
          row.original.tlsEnabled ? "Enabled" : "Disabled"
        }}</template>
        <template #actions-cell="{ row }"
          ><div class="flex justify-end">
            <UDropdownMenu
              :items="[
                {
                  label: 'Details',
                  icon: 'i-lucide-info',
                  color: 'neutral' as const,
                  disabled: functionControlPlane.busy.value,
                  onSelect: () => {
                    domainId = row.original.id;
                    editDomain();
                  },
                },
                ...(auth.hasScope('function:endpoint.write')
                  ? [
                      { type: 'separator' as const },
                      {
                        label: 'Delete Domain',
                        icon: 'i-lucide-trash-2',
                        color: 'error' as const,
                        disabled: functionControlPlane.busy.value,
                        onSelect: () => {
                          domainId = row.original.id;
                          confirmingDelete = true;
                        },
                      },
                    ]
                  : []),
              ]"
              :content="{ align: 'end' }"
            >
              <UButton
                square
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                :aria-label="`Actions for ${row.original.domain}`"
              />
            </UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            title="No Custom Domains"
            description="Add a domain to route your hostname to an endpoint."
            icon="i-lucide-globe"
        /></template>
        <template #loading
          ><UEmpty title="Loading Domains…" loading icon="i-lucide-globe"
        /></template>
      </UTable>
      <template #footer
        ><UButton
          v-if="auth.hasScope('function:endpoint.write')"
          label="New Domain"
          icon="i-lucide-plus"
          :disabled="functionControlPlane.busy.value"
          @click="newDomain"
      /></template>
    </UCard>
    <div
      v-if="functionControlPlane.selectedEndpoint.value && auth.hasScope('function:endpoint.write')"
      class="flex justify-end"
    >
      <UButton label="Attach Domain Route" icon="i-lucide-plus" @click="attachingRoute = true" />
    </div>
    <UModal
      :open="!!editing"
      :dismissible="!functionControlPlane.busy.value"
      :title="domainId ? 'Domain Details' : 'New Domain'"
      description="Hostnames cannot be renamed. TLS provisioning requires verified DNS and an ACME contact email or service default."
      :ui="{ body: 'space-y-6', content: 'max-w-4xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
            editing = false;
          }
        }
      "
      ><template #body
        ><UAlert
          v-if="functionControlPlane.error.value"
          title="Request Failed"
          :description="functionControlPlane.error.value"
          color="error"
          role="alert" />
        <UForm
          id="function-domain"
          ref="domainFormElement"
          :state="domainForm"
          :validate="validateDomain"
          :disabled="functionControlPlane.busy.value || !auth.hasScope('function:endpoint.write')"
          class="space-y-5"
          @submit="saveDomain"
        >
          <UFormField
            label="Domain"
            name="domain"
            description="Hostname only, such as api.example.com."
            required
            ><UInput v-model="domainForm.domain" :disabled="!!domainId" class="w-full"
          /></UFormField>
          <USwitch
            v-model="domainForm.tlsEnabled"
            label="Enable TLS"
            description="Provision and renew a certificate after DNS verification succeeds."
          />
          <UFormField
            label="ACME Contact Email"
            name="tlsAcmeEmail"
            description="Leave blank to use the service's default ACME email."
            ><UInput v-model="domainForm.tlsAcmeEmail" class="w-full"
          /></UFormField>
        </UForm>
        <section v-if="functionControlPlane.selectedDomain.value" class="space-y-5">
          <header class="space-y-2">
            <USeparator label="DNS and TLS Instructions" position="start" />
            <p class="text-sm text-muted">
              {{
                `DNS setup and certificate status for ${functionControlPlane.selectedDomain.value.domain}.`
              }}
            </p>
          </header>
          <UAlert
            v-if="functionControlPlane.selectedDomain.value.status !== 'active'"
            title="DNS Not Verified"
            description="Apply the DNS records below, allow propagation, then verify again. Routes can only be attached to active domains."
            color="warning"
          />
          <UBadge
            :label="`DNS: ${functionControlPlane.selectedDomain.value.status} · TLS: ${functionControlPlane.selectedDomain.value.tlsStatus}`"
            color="neutral"
            variant="subtle"
          />
          <UAlert
            v-if="functionControlPlane.selectedDomain.value.tlsErrorMessage"
            title="TLS Needs Attention"
            :description="functionControlPlane.selectedDomain.value.tlsErrorMessage"
            color="warning"
          />
          <UTable
            :data="functionControlPlane.selectedDomain.value.requiredDnsRecords || []"
            :columns="dnsColumns"
            :ui="{ td: 'whitespace-normal break-all' }"
          />
          <div
            v-if="auth.hasScope('function:endpoint.write')"
            class="flex flex-wrap justify-end gap-3"
          >
            <UButton
              label="Verify DNS"
              :disabled="functionControlPlane.busy.value"
              @click="verifyDomain"
            />
          </div></section></template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="editing = false" /><UButton
          v-if="auth.hasScope('function:endpoint.write')"
          type="submit"
          form="function-domain"
          label="Save Domain"
          :disabled="functionControlPlane.busy.value"
      /></template>
    </UModal>

    <UCard
      title="Endpoint Domain Routes"
      description="Choose an endpoint to inspect its domain mappings. Only DNS-verified domains can receive new mappings."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UFormField label="Route Endpoint" description="Search for the endpoint to map to a domain."
        ><USelectMenu
          :model-value="endpointId"
          :items="functionControlPlane.endpoints.value?.data || []"
          value-key="id"
          label-key="name"
          placeholder="Choose an endpoint"
          :disabled="functionControlPlane.busy.value"
          class="w-full"
          @update:model-value="chooseEndpoint"
      /></UFormField>
      <UTable
        v-if="functionControlPlane.selectedEndpoint.value"
        :data="functionControlPlane.routes.value?.data || []"
        :columns="routeColumns"
        :loading="functionControlPlane.busy.value"
        caption="Domain routes attached to the chosen endpoint. Remove mappings using their row actions."
      >
        <template #actions-cell="{ row }"
          ><UButton
            label="Remove Route"
            color="error"
            variant="ghost"
            :disabled="functionControlPlane.busy.value"
            @click="
              selectedRouteId = row.original.id;
              confirmingRemove = true;
            "
        /></template>
        <template #empty
          ><UEmpty
            title="No Domain Routes"
            description="Attach a verified domain and path prefix below."
            icon="i-lucide-globe"
        /></template>
        <template #loading
          ><UEmpty title="Loading Routes…" loading icon="i-lucide-globe"
        /></template>
      </UTable>
    </UCard>
    <UModal
      :open="!!attachingRoute"
      :dismissible="!functionControlPlane.busy.value"
      title="Attach Domain Route"
      description="Map a verified hostname and path prefix to the selected endpoint. The backend checks for conflicting routes."
      :ui="{ body: 'space-y-6', content: 'max-w-4xl', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      @update:open="
        (open) => {
          if (!open) {
            attachingRoute = false;
          }
        }
      "
      ><template #body
        ><UAlert
          v-if="functionControlPlane.error.value"
          title="Request Failed"
          :description="functionControlPlane.error.value"
          color="error"
          role="alert"
        />
        <UForm
          id="function-domain-route"
          ref="routeFormElement"
          :state="routeForm"
          :validate="validateRoute"
          :disabled="functionControlPlane.busy.value"
          class="space-y-5"
          @submit="attachRoute"
        >
          <UFormField
            label="Verified Domain"
            name="customDomainId"
            description="Only active, DNS-verified domains appear here."
            required
            ><USelectMenu
              v-model="routeForm.customDomainId"
              :items="verifiedDomains"
              value-key="id"
              label-key="domain"
              placeholder="Choose a verified domain"
              class="w-full"
          /></UFormField>
          <UFormField
            label="Path Prefix"
            name="pathPrefix"
            description="Use / for the root, or a prefix such as /api."
            required
            ><UInput v-model="routeForm.pathPrefix" class="w-full"
          /></UFormField>
        </UForm> </template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="attachingRoute = false" /><UButton
          type="submit"
          form="function-domain-route"
          label="Attach Route"
          :disabled="functionControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmingDelete"
      title="Delete Custom Domain?"
      description="This permanently removes the domain and all its endpoint routes."
      :dismissible="!functionControlPlane.busy.value"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      ><template v-if="functionControlPlane.error.value" #body
        ><UAlert
          title="Deletion Failed"
          :description="functionControlPlane.error.value"
          color="error" /></template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Domain"
          color="error"
          :loading="functionControlPlane.busy.value"
          @click="deleteDomain" /></template
    ></UModal>
    <UModal
      v-model:open="confirmingRemove"
      title="Remove Domain Route?"
      description="This removes the selected mapping. The endpoint and custom domain remain registered."
      :dismissible="!functionControlPlane.busy.value"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      ><template v-if="functionControlPlane.error.value" #body
        ><UAlert
          title="Removal Failed"
          :description="functionControlPlane.error.value"
          color="error" /></template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="confirmingRemove = false" /><UButton
          label="Remove Route"
          color="error"
          :loading="functionControlPlane.busy.value"
          @click="deleteRoute" /></template
    ></UModal>
  </div>
</template>
