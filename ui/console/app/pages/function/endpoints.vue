<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";
import { validFunctionFilePath } from "../../utils/function-forms";
const auth = useAuth();
const client = useClient();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const selectedEndpointId = ref("");
const editing = ref(false);
const confirmingDelete = ref(false);
const endpointFormTemplateRef = useTemplateRef("endpointFormElement");
const endpointForm = reactive({
  name: "",
  description: "",
  runtime: "default",
  entrypoint: "index.js",
  overrideLimits: false,
  memoryLimitMb: 128,
  timeoutSeconds: 30,
  isPublic: false,
});
const runtimeOptions = computed(() =>
  [...new Set(["workerd", endpointForm.runtime])].filter((name) => name !== "default"),
);
const columns: TableColumn<FunctionControlPlaneModels.Endpoint>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "runtime", header: "Runtime" },
  { accessorKey: "isPublic", header: "Access" },
  { accessorKey: "activeDeploymentId", header: "Deployment" },
  { id: "actions", header: "" },
];
const invocationUrl = computed(() =>
  functionControlPlane.selectedEndpoint.value
    ? `${client.value.endpoint}/v1/function/${encodeURIComponent(functionControlPlane.selectedEndpoint.value.name)}`
    : "",
);
function validateEndpoint(): FormError[] {
  const errors: FormError[] = [];
  if (!/^[a-zA-Z0-9_-]{1,128}$/.test(endpointForm.name.trim()))
    errors.push({ name: "name", message: "Use 1–128 letters, numbers, underscores, or hyphens." });
  if (!validFunctionFilePath(endpointForm.entrypoint.trim()))
    errors.push({
      name: "entrypoint",
      message: "Use a relative entrypoint file path, such as index.js.",
    });
  if (endpointForm.overrideLimits)
    for (const key of ["memoryLimitMb", "timeoutSeconds"] as const)
      if (!Number.isInteger(endpointForm[key]) || endpointForm[key] < 1)
        errors.push({ name: key, message: "Enter a positive whole number." });
  return errors;
}
function newEndpoint() {
  selectedEndpointId.value = "";
  functionControlPlane.selectedEndpoint.value = null;
  Object.assign(endpointForm, {
    name: "",
    description: "",
    runtime: "default",
    entrypoint: "index.js",
    overrideLimits: false,
    memoryLimitMb: 128,
    timeoutSeconds: 30,
    isPublic: false,
  });
  editing.value = true;
}
async function editEndpoint() {
  if (!selectedEndpointId.value) return;
  if (await functionControlPlane.selectEndpoint(selectedEndpointId.value)) {
    const endpoint = functionControlPlane.selectedEndpoint.value!;
    Object.assign(endpointForm, {
      name: endpoint.name,
      description: endpoint.description || "",
      runtime: endpoint.runtime,
      entrypoint: endpoint.entrypoint,
      overrideLimits: true,
      memoryLimitMb: endpoint.memoryLimitMb,
      timeoutSeconds: endpoint.timeoutSeconds,
      isPublic: endpoint.isPublic,
    });
    editing.value = true;
  }
}
async function saveEndpoint() {
  const errors = validateEndpoint();
  if (errors.length) {
    endpointFormTemplateRef.value?.setErrors(errors);
    return;
  }
  if (
    await functionControlPlane.saveEndpoint(
      {
        name: endpointForm.name.trim(),
        description: endpointForm.description,
        runtime: endpointForm.runtime === "default" ? undefined : endpointForm.runtime,
        entrypoint: endpointForm.entrypoint.trim(),
        memoryLimitMb: endpointForm.overrideLimits ? endpointForm.memoryLimitMb : undefined,
        timeoutSeconds: endpointForm.overrideLimits ? endpointForm.timeoutSeconds : undefined,
        isPublic: endpointForm.isPublic,
      },
      selectedEndpointId.value,
    )
  ) {
    selectedEndpointId.value = functionControlPlane.selectedEndpoint.value!.id;
    editing.value = false;
  }
}
async function deleteEndpoint() {
  if (!selectedEndpointId.value) return;
  if (await functionControlPlane.deleteEndpoint(selectedEndpointId.value)) {
    selectedEndpointId.value = "";
    confirmingDelete.value = false;
    editing.value = false;
  }
}
onMounted(() => {
  if (auth.hasScope("function:endpoint.read")) return functionControlPlane.load("endpoints");
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
      title="Function Endpoints"
      description="Configure endpoint access and runtime settings. New endpoints are private and need a deployment before they can run."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="functionControlPlane.endpoints.value?.data || []"
        :columns="columns"
        :loading="functionControlPlane.busy.value"
        :inert="editing || functionControlPlane.busy.value"
        :get-row-id="(endpoint) => endpoint.id"
        caption="Serverless endpoints and management actions."
        @select="
          (_event, row) => {
            selectedEndpointId = row.original.id;
            editEndpoint();
          }
        "
      >
        <template #name-cell="{ row }"
          ><span class="font-medium text-highlighted">{{ row.original.name }}</span></template
        >
        <template #isPublic-cell="{ row }"
          ><UBadge
            :label="row.original.isPublic ? 'Public' : 'Private'"
            color="neutral"
            variant="subtle"
        /></template>
        <template #activeDeploymentId-cell="{ row }"
          ><UBadge
            :label="row.original.activeDeploymentId ? 'Deployed' : 'Not Deployed'"
            :color="row.original.activeDeploymentId ? 'success' : 'neutral'"
            variant="subtle"
        /></template>
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
                    selectedEndpointId = row.original.id;
                    editEndpoint();
                  },
                },
                ...(auth.hasScope('function:endpoint.write')
                  ? [
                      { type: 'separator' as const },
                      {
                        label: 'Delete Endpoint',
                        icon: 'i-lucide-trash-2',
                        color: 'error' as const,
                        disabled: functionControlPlane.busy.value,
                        onSelect: () => {
                          selectedEndpointId = row.original.id;
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
                :aria-label="`Actions for ${row.original.name}`"
              />
            </UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            title="No Function Endpoints"
            description="Create an endpoint, then deploy its code."
            icon="i-lucide-square-terminal"
        /></template>
        <template #loading
          ><UEmpty title="Loading Endpoints…" loading icon="i-lucide-square-terminal"
        /></template>
      </UTable>
      <template #footer>
        <UButton
          v-if="auth.hasScope('function:endpoint.write')"
          label="New Endpoint"
          icon="i-lucide-plus"
          :disabled="functionControlPlane.busy.value"
          @click="newEndpoint"
        />
      </template>
    </UCard>
    <UModal
      :open="!!editing"
      :dismissible="!functionControlPlane.busy.value"
      :title="selectedEndpointId ? 'Endpoint Details' : 'New Endpoint'"
      description="Configure the endpoint identity and runtime settings. Changing settings on a deployed endpoint may require redeployment."
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
          id="function-endpoint"
          ref="endpointFormElement"
          :state="endpointForm"
          :validate="validateEndpoint"
          :disabled="functionControlPlane.busy.value || !auth.hasScope('function:endpoint.write')"
          class="space-y-5"
          @submit="saveEndpoint"
        >
          <UFormField
            label="Name"
            name="name"
            description="Used in the invocation URL. Names cannot be changed after creation."
            required
            ><UInput v-model="endpointForm.name" :disabled="!!selectedEndpointId" class="w-full"
          /></UFormField>
          <UFormField
            label="Description"
            name="description"
            description="Optional explanation of what this endpoint does."
            ><UTextarea v-model="endpointForm.description" class="w-full"
          /></UFormField>
          <UFormField
            label="Runtime"
            name="runtime"
            description="workerd is the runtime registered by this service. New endpoints can inherit the configured default."
            required
            ><USelect
              v-model="endpointForm.runtime"
              :items="[
                ...(!selectedEndpointId ? [{ label: 'Service Default', value: 'default' }] : []),
                ...runtimeOptions.map((value) => ({ label: value, value })),
              ]"
              class="w-full"
          /></UFormField>
          <UFormField
            label="Entrypoint"
            name="entrypoint"
            description="The relative file path to include in every deployment, such as index.js."
            required
            ><UInput v-model="endpointForm.entrypoint" class="w-full"
          /></UFormField>
          <USwitch
            v-if="!selectedEndpointId"
            v-model="endpointForm.overrideLimits"
            label="Customize Resource Settings"
            description="Leave off to inherit service defaults when the endpoint is created."
          />
          <div v-if="endpointForm.overrideLimits" class="grid items-end gap-4 sm:grid-cols-2">
            <UFormField
              label="Memory Setting (MB)"
              name="memoryLimitMb"
              description="The memory setting stored with this endpoint."
              required
              ><UInputNumber v-model="endpointForm.memoryLimitMb" :min="1" class="w-full"
            /></UFormField>
            <UFormField
              label="Timeout (Seconds)"
              name="timeoutSeconds"
              description="The timeout setting stored with this endpoint."
              required
              ><UInputNumber v-model="endpointForm.timeoutSeconds" :min="1" class="w-full"
            /></UFormField>
          </div>
          <USwitch
            v-model="endpointForm.isPublic"
            label="Public Endpoint"
            description="Allow unauthenticated invocation. Private endpoints require a Service Account with function:endpoint.invoke scope."
          />
        </UForm>
        <section v-if="functionControlPlane.selectedEndpoint.value" class="space-y-5">
          <header class="space-y-2">
            <USeparator label="Endpoint Access" position="start" />
            <p class="text-sm text-muted">
              The gateway invocation URL. Deploy code before invoking this endpoint.
            </p>
          </header>
          <code class="break-all text-sm">{{ invocationUrl }}</code>
          <p class="text-sm text-muted">
            Private endpoints require Service Account authentication; Console session cookies alone
            do not grant invocation access.
          </p>
          <div class="flex flex-wrap justify-end gap-3">
            <UButton
              :to="{
                path: '/function/deployments',
                query: { endpoint: functionControlPlane.selectedEndpoint.value.id },
              }"
              label="Deployments"
              color="neutral"
              variant="outline"
            />
            <UButton
              :to="{
                path: '/function/executions',
                query: { endpoint: functionControlPlane.selectedEndpoint.value.id },
              }"
              label="Execution Logs"
              color="neutral"
              variant="outline"
            />
            <UButton
              v-if="
                functionControlPlane.selectedEndpoint.value.isPublic &&
                functionControlPlane.selectedEndpoint.value.activeDeploymentId
              "
              :to="invocationUrl"
              target="_blank"
              label="Open Public Endpoint"
              color="neutral"
              variant="outline"
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
          form="function-endpoint"
          :label="selectedEndpointId ? 'Save Endpoint' : 'Create Endpoint'"
          :disabled="functionControlPlane.busy.value"
      /></template>
    </UModal>

    <UModal
      v-model:open="confirmingDelete"
      title="Delete Function Endpoint?"
      description="This removes the endpoint, deployment history, execution records, and custom-domain route mappings. This cannot be undone."
      :dismissible="!functionControlPlane.busy.value"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      :close="{ disabled: functionControlPlane.busy.value }"
      ><template v-if="functionControlPlane.error.value" #body
        ><UAlert
          title="Request Failed"
          :description="functionControlPlane.error.value"
          color="error"
          role="alert" /></template
      ><template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Endpoint"
          color="error"
          :loading="functionControlPlane.busy.value"
          @click="deleteEndpoint" /></template
    ></UModal>
  </div>
</template>
