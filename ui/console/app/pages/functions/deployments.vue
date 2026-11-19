<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { FunctionControlPlaneModels } from "layr-client";
import { readFunctionArchive, resolveFunctionDeployment } from "../../utils/function-forms";
const auth = useAuth();
const client = useClient();
const route = useRoute();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const endpointId = ref("");
const draftReady = ref(false);
const selectedDeployment = ref<FunctionControlPlaneModels.Deployment | null>(null);
const confirmingDeploy = ref(false);
const confirmingRollback = ref(false);
const showValues = ref(false);
const archiveFile = shallowRef<File | null>(null);
const readingArchive = ref(false);
const archiveError = ref("");
const selectedFileIndex = ref(0);
const selectedVariableIndex = ref(-1);
const editingVariableIndex = ref(-1);
const editingVariable = ref(false);
const showDraftValue = ref(false);
const removingDraftItem = ref<"file" | "variable" | null>(null);
const deploymentFormTemplateRef = useTemplateRef("deploymentFormElement");
const variableFormTemplateRef = useTemplateRef("variableFormElement");
const deploymentForm = reactive({
  bundleSource: "archive" as "archive" | "files",
  archive: null as Awaited<ReturnType<typeof readFunctionArchive>> | null,
  files: [{ path: "index.js", content: "" }],
  variables: [] as { name: string; value: string }[],
  overrideRuntime: false,
  overrideFlags: false,
  compatibilityDate: "",
  compatibilityFlags: [] as string[],
});
const variableForm = reactive({ name: "", value: "" });
const columns: TableColumn<FunctionControlPlaneModels.Deployment>[] = [
  { accessorKey: "version", header: "Version" },
  { id: "active", header: "Active" },
  { accessorKey: "bundleFormat", header: "Format" },
  { accessorKey: "createdAt", header: "Created" },
  { accessorKey: "bundleHash", header: "Bundle Hash" },
];
const activeDeployment = computed(() =>
  functionControlPlane.deployments.value?.data?.find(
    (deployment) =>
      deployment.id === functionControlPlane.selectedEndpoint.value?.activeDeploymentId,
  ),
);
const selectedFile = computed(() => deploymentForm.files[selectedFileIndex.value] || null);
const fileOptions = computed(() =>
  deploymentForm.files.map((file, index) => ({
    label: file.path || "Untitled file",
    value: index,
  })),
);
const draftManifest = computed(() =>
  deploymentForm.files.map((file) => ({
    path: file.path,
    bytes: new TextEncoder().encode(file.content).length,
  })),
);
const removedVariables = computed(() =>
  Object.keys(activeDeployment.value?.environmentVariables || {}).filter(
    (name) => !deploymentForm.variables.some((variable) => variable.name.trim() === name),
  ),
);
const manifest = computed(() =>
  Object.keys(selectedDeployment.value?.bundleFiles || {}).map((path) => ({ path })),
);
const environment = computed(() =>
  Object.entries(selectedDeployment.value?.environmentVariables || {}).map(([name, value]) => ({
    name,
    value: showValues.value ? value : "Hidden",
  })),
);
async function chooseEndpoint(id: string) {
  endpointId.value = id;
  draftReady.value = false;
  selectedDeployment.value = null;
  archiveFile.value = null;
  deploymentForm.archive = null;
  archiveError.value = "";
  readingArchive.value = false;
  selectedFileIndex.value = 0;
  selectedVariableIndex.value = -1;
  editingVariable.value = false;
  confirmingDeploy.value = false;
  showValues.value = false;
  if (!id) {
    functionControlPlane.selectedEndpoint.value = null;
    functionControlPlane.deployments.value = null;
    return;
  }
  if (await functionControlPlane.selectEndpoint(id, "deployments")) {
    const runtimeConfig = activeDeployment.value?.workerdRuntimeConfig;
    Object.assign(deploymentForm, {
      bundleSource: "archive",
      files: [{ path: functionControlPlane.selectedEndpoint.value!.entrypoint, content: "" }],
      variables: Object.entries(activeDeployment.value?.environmentVariables || {}).map(
        ([name, value]) => ({ name, value }),
      ),
      overrideRuntime: !!runtimeConfig,
      overrideFlags: runtimeConfig?.compatibilityFlags !== undefined,
      compatibilityDate:
        runtimeConfig?.compatibilityDate ||
        functionControlPlane.config.value?.workerdRuntime.compatibilityDate ||
        "",
      compatibilityFlags: [
        ...(runtimeConfig?.compatibilityFlags ||
          functionControlPlane.config.value?.workerdRuntime.compatibilityFlags ||
          []),
      ],
    });
    draftReady.value = true;
  }
}
async function selectArchive(file: File | null | undefined) {
  archiveFile.value = file || null;
  deploymentForm.archive = null;
  archiveError.value = "";
  readingArchive.value = false;
  if (!file) return;
  readingArchive.value = true;
  try {
    const archive = await readFunctionArchive(
      file,
      functionControlPlane.config.value?.maxBundleSizeBytes,
    );
    if (archiveFile.value === file) deploymentForm.archive = archive;
  } catch (err) {
    if (archiveFile.value === file)
      archiveError.value = err instanceof Error ? err.message : String(err);
  } finally {
    if (archiveFile.value === file) readingArchive.value = false;
  }
}
function rejectArchive() {
  archiveFile.value = null;
  deploymentForm.archive = null;
  readingArchive.value = false;
  archiveError.value = "Choose a ZIP (.zip) or uncompressed TAR (.tar) archive.";
}
function addFile() {
  deploymentForm.files.push({ path: "", content: "" });
  selectedFileIndex.value = deploymentForm.files.length - 1;
}
function removeDraftItem() {
  if (removingDraftItem.value === "file" && deploymentForm.files.length > 1) {
    deploymentForm.files.splice(selectedFileIndex.value, 1);
    selectedFileIndex.value = Math.min(selectedFileIndex.value, deploymentForm.files.length - 1);
  } else if (removingDraftItem.value === "variable" && selectedVariableIndex.value >= 0) {
    deploymentForm.variables.splice(selectedVariableIndex.value, 1);
    selectedVariableIndex.value = -1;
  }
  removingDraftItem.value = null;
}
function newVariable() {
  editingVariableIndex.value = -1;
  Object.assign(variableForm, { name: "", value: "" });
  showDraftValue.value = false;
  editingVariable.value = true;
}
function editVariable() {
  const variable = deploymentForm.variables[selectedVariableIndex.value];
  if (!variable) return;
  editingVariableIndex.value = selectedVariableIndex.value;
  Object.assign(variableForm, variable);
  showDraftValue.value = false;
  editingVariable.value = true;
}
function validateVariable(): FormError[] {
  const name = variableForm.name.trim();
  if (
    !/^[A-Za-z_][A-Za-z0-9_]*$/.test(name) ||
    deploymentForm.variables.some(
      (variable, index) => index !== editingVariableIndex.value && variable.name.trim() === name,
    )
  )
    return [
      { name: "name", message: "Enter a unique environment variable name, such as API_URL." },
    ];
  const allowed = functionControlPlane.config.value?.allowedEnvironmentKeys;
  if (allowed?.length && !allowed.includes(name))
    return [{ name: "name", message: "This key is not in the service environment-key allowlist." }];
  return [];
}
function saveVariable() {
  const errors = validateVariable();
  if (errors.length) {
    variableFormTemplateRef.value?.setErrors(errors);
    return;
  }
  const variable = { name: variableForm.name.trim(), value: variableForm.value };
  if (editingVariableIndex.value >= 0)
    deploymentForm.variables[editingVariableIndex.value] = variable;
  else deploymentForm.variables.push(variable);
  selectedVariableIndex.value =
    editingVariableIndex.value >= 0
      ? editingVariableIndex.value
      : deploymentForm.variables.length - 1;
  editingVariable.value = false;
  showDraftValue.value = false;
}
function validateDeployment() {
  return resolveFunctionDeployment(
    deploymentForm,
    functionControlPlane.selectedEndpoint.value?.entrypoint || "index.js",
  ).errors;
}
function reviewDeployment() {
  if (!functionControlPlane.selectedEndpoint.value || readingArchive.value) return;
  const errors = validateDeployment();
  if (errors.length) {
    deploymentFormTemplateRef.value?.setErrors(errors);
    return;
  }
  confirmingDeploy.value = true;
}
async function deploy() {
  if (!functionControlPlane.selectedEndpoint.value || readingArchive.value) return;
  const { input, errors } = resolveFunctionDeployment(
    deploymentForm,
    functionControlPlane.selectedEndpoint.value.entrypoint,
  );
  if (errors.length) {
    deploymentFormTemplateRef.value?.setErrors(errors);
    confirmingDeploy.value = false;
    return;
  }
  if (await functionControlPlane.deploy(endpointId.value, input)) confirmingDeploy.value = false;
}
async function rollback() {
  if (!selectedDeployment.value) return;
  if (await functionControlPlane.rollback(endpointId.value, selectedDeployment.value.version))
    confirmingRollback.value = false;
}
onMounted(async () => {
  if (!auth.hasScope("function:endpoint.read")) return;
  await functionControlPlane.load("endpoints");
  if (auth.hasScope("function:config.read")) await functionControlPlane.load("configuration");
  if (typeof route.query.endpoint === "string") await chooseEndpoint(route.query.endpoint);
});
</script>
<template>
  <div class="space-y-6" :aria-busy="functionControlPlane.busy.value || readingArchive">
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
      title="Endpoint Deployments"
      description="Select an endpoint to publish a version or inspect deployment history."
    >
      <UFormField label="Endpoint" description="Search registered endpoint names.">
        <USelectMenu
          :model-value="endpointId"
          :items="functionControlPlane.endpoints.value?.data || []"
          value-key="id"
          label-key="name"
          placeholder="Choose an endpoint"
          :disabled="functionControlPlane.busy.value || readingArchive"
          class="w-full"
          @update:model-value="chooseEndpoint"
        />
      </UFormField>
    </UCard>
    <UForm
      v-if="
        draftReady &&
        functionControlPlane.selectedEndpoint.value &&
        auth.hasScope('function:endpoint.deploy')
      "
      id="function-deployment"
      ref="deploymentFormElement"
      :state="deploymentForm"
      :validate="validateDeployment"
      :disabled="functionControlPlane.busy.value || readingArchive"
      class="space-y-6"
      @submit="reviewDeployment"
    >
      <UCard
        title="New Deployment"
        description="Upload a prepared bundle, or edit source files for this version. Nothing changes until you review and activate the deployment."
        :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
      >
        <div class="space-y-5">
          <UTabs
            v-model="deploymentForm.bundleSource"
            :items="[
              { label: 'Upload Archive', value: 'archive', icon: 'i-lucide-upload' },
              { label: 'Source Files', value: 'files', icon: 'i-lucide-code' },
            ]"
            :content="false"
          />
          <template v-if="deploymentForm.bundleSource === 'archive'">
            <UFormField
              label="Bundle Archive"
              name="archive"
              :error="archiveError || undefined"
              :description="`Include ${functionControlPlane.selectedEndpoint.value.entrypoint} at its configured path, without an enclosing project folder.`"
              required
            >
              <UFileUpload
                :model-value="archiveFile"
                accept=".zip,.tar"
                label="Drop your deployment bundle here"
                :description="
                  functionControlPlane.config.value
                    ? `ZIP or uncompressed TAR · up to ${(functionControlPlane.config.value.maxBundleSizeBytes / (1024 * 1024)).toLocaleString()} MiB`
                    : 'ZIP or uncompressed TAR · service size limits apply'
                "
                layout="list"
                :file-image="false"
                :file-delete="false"
                :disabled="functionControlPlane.busy.value || readingArchive"
                class="w-full min-h-40"
                @update:model-value="selectArchive"
                @reject="rejectArchive"
              />
            </UFormField>
            <UAlert
              v-if="readingArchive"
              title="Reading Bundle…"
              description="Preparing the selected archive. Keep this page open."
              color="info"
            />
          </template>
          <UFormField
            v-else
            label="Bundle Files"
            name="files"
            description="Edit one file at a time. Paths are relative to the bundle root and must include the endpoint entrypoint."
            required
          >
            <div class="space-y-4">
              <UFormField
                v-if="deploymentForm.files.length > 1"
                label="File"
                description="Choose the file to edit."
              >
                <USelect
                  v-model="selectedFileIndex"
                  :items="fileOptions"
                  value-key="value"
                  class="w-full"
                />
              </UFormField>
              <template v-if="selectedFile">
                <UFormField
                  label="File Path"
                  :name="`files.${selectedFileIndex}.path`"
                  description="For example, index.js or lib/helper.js."
                  required
                  ><UInput
                    v-model="selectedFile.path"
                    placeholder="lib/helper.js"
                    class="w-full font-mono"
                /></UFormField>
                <UFormField label="Source Code" :name="`files.${selectedFileIndex}.content`"
                  ><UTextarea
                    v-model="selectedFile.content"
                    :rows="14"
                    class="w-full font-mono"
                    placeholder="Enter the source for this file"
                /></UFormField>
              </template>
              <UEmpty
                v-else
                title="No Source Files"
                description="Add a file to start writing this bundle."
                icon="i-lucide-file-code"
              />
            </div>
          </UFormField>
        </div>
        <template v-if="deploymentForm.bundleSource === 'files' || archiveFile" #footer>
          <template v-if="deploymentForm.bundleSource === 'files'">
            <UButton
              label="Add File"
              color="neutral"
              variant="outline"
              :disabled="functionControlPlane.busy.value || readingArchive"
              @click="addFile"
            />
            <UButton
              label="Remove Selected File"
              color="error"
              variant="outline"
              :disabled="
                functionControlPlane.busy.value || deploymentForm.files.length <= 1 || !selectedFile
              "
              @click="removingDraftItem = 'file'"
            />
          </template>
          <UButton
            v-else-if="archiveFile"
            label="Clear Archive"
            color="neutral"
            variant="outline"
            :disabled="functionControlPlane.busy.value || readingArchive"
            @click="selectArchive(null)"
          />
        </template>
      </UCard>
      <UCard
        title="Environment Variables"
        :description="
          activeDeployment
            ? `Starting with variables from version ${activeDeployment.version}. Edits apply only to the new deployment; values stay hidden in this list.`
            : 'Add values your function needs at runtime. Variables belong to this deployment, not the service configuration.'
        "
        :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
      >
        <UTable
          :data="deploymentForm.variables"
          :columns="[
            { accessorKey: 'name', header: 'Variable' },
            { id: 'value', header: 'Value' },
          ]"
          caption="Select a variable to edit or remove it from the draft."
          @select="(_event, row) => (selectedVariableIndex = row.index)"
        >
          <template #value-cell
            ><UBadge label="Hidden" color="neutral" variant="subtle"
          /></template>
          <template #empty
            ><UEmpty
              title="No Environment Variables"
              description="Add a variable if your function needs configuration or secrets."
              icon="i-lucide-key-round"
          /></template>
        </UTable>
        <template #footer>
          <UButton
            label="Add Variable"
            color="neutral"
            variant="outline"
            :disabled="functionControlPlane.busy.value || readingArchive"
            @click="newVariable"
          />
          <UButton
            label="Edit Selected Variable"
            color="neutral"
            variant="outline"
            :disabled="
              functionControlPlane.busy.value || readingArchive || selectedVariableIndex < 0
            "
            @click="editVariable"
          />
          <UButton
            label="Remove Selected Variable"
            color="error"
            variant="outline"
            :disabled="
              functionControlPlane.busy.value || readingArchive || selectedVariableIndex < 0
            "
            @click="removingDraftItem = 'variable'"
          />
        </template>
      </UCard>
      <UCard
        title="Runtime Compatibility"
        description="Use service defaults, or pin compatibility behavior for this version. Overrides from the active deployment are retained in the draft."
        :ui="{ footer: 'flex justify-end' }"
      >
        <div class="space-y-4">
          <USwitch
            v-model="deploymentForm.overrideRuntime"
            label="Override Workerd Compatibility"
            description="Pin a compatibility date instead of inheriting the service default."
          />
          <template v-if="deploymentForm.overrideRuntime">
            <UFormField label="Deployment Compatibility Date" name="compatibilityDate" required
              ><UInput v-model="deploymentForm.compatibilityDate" type="date" class="w-full"
            /></UFormField>
            <USwitch
              v-model="deploymentForm.overrideFlags"
              label="Override Compatibility Flags"
              description="Leave off to inherit service flags. An explicit empty list disables all default flags for this version."
            />
            <UFormField
              v-if="deploymentForm.overrideFlags"
              label="Deployment Compatibility Flags"
              name="compatibilityFlags"
              description="Press Enter to add a flag supported by the installed workerd version."
              ><UInputTags
                v-model="deploymentForm.compatibilityFlags"
                placeholder="nodejs_compat"
                :delimiter="/[\n,]/"
                add-on-paste
                add-on-blur
                class="w-full"
            /></UFormField>
          </template>
        </div>
        <template #footer
          ><UButton
            type="submit"
            form="function-deployment"
            label="Review Deployment"
            :disabled="functionControlPlane.busy.value || readingArchive"
        /></template>
      </UCard>
    </UForm>
    <UCard
      v-if="functionControlPlane.selectedEndpoint.value"
      title="Deployment History"
      description="Select a version to inspect its manifest or restore its code, environment, and runtime overrides."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UTable
        :data="functionControlPlane.deployments.value?.data || []"
        :columns="columns"
        :loading="functionControlPlane.busy.value"
        :ui="{ td: 'whitespace-normal break-all' }"
        caption="Immutable deployments. Select a version to inspect it."
        @select="(_event, row) => (selectedDeployment = row.original)"
      >
        <template #active-cell="{ row }"
          ><UBadge
            :label="
              row.original.id === functionControlPlane.selectedEndpoint.value.activeDeploymentId
                ? 'Active'
                : 'Inactive'
            "
            :color="
              row.original.id === functionControlPlane.selectedEndpoint.value.activeDeploymentId
                ? 'success'
                : 'neutral'
            "
            variant="subtle"
        /></template>
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #empty
          ><UEmpty
            title="No Deployments Yet"
            description="An endpoint needs a code deployment before it can run."
            icon="i-lucide-rocket"
        /></template>
        <template #loading
          ><UEmpty title="Loading Deployments…" loading icon="i-lucide-rocket"
        /></template>
      </UTable>
      <template v-if="auth.hasScope('function:endpoint.deploy')" #footer
        ><UButton
          label="Roll Back to Selected Version"
          color="neutral"
          variant="outline"
          :disabled="
            functionControlPlane.busy.value ||
            !selectedDeployment ||
            selectedDeployment.id === functionControlPlane.selectedEndpoint.value.activeDeploymentId
          "
          @click="confirmingRollback = true"
      /></template>
    </UCard>
    <UCard
      v-if="selectedDeployment"
      :title="`Deployment Version ${selectedDeployment.version}`"
      description="File manifest and environment for the selected version. Source code is not returned by the history API."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UTable :data="manifest" :columns="[{ accessorKey: 'path', header: 'File' }]" />
      <UTable
        :data="environment"
        :columns="[
          { accessorKey: 'name', header: 'Environment Key' },
          { accessorKey: 'value', header: 'Value' },
        ]"
      />
      <USwitch
        v-model="showValues"
        label="Show Stored Variable Values"
        description="Environment values can contain secrets."
      />
      <template #footer
        ><UButton
          label="Close Version Details"
          color="neutral"
          variant="outline"
          @click="selectedDeployment = null"
      /></template>
    </UCard>
    <UModal
      v-model:open="editingVariable"
      :title="editingVariableIndex >= 0 ? 'Edit Environment Variable' : 'Add Environment Variable'"
      description="This changes only the deployment draft. Empty values are stored as empty strings."
      :ui="{ footer: 'justify-end' }"
    >
      <template #body>
        <UForm
          id="function-variable"
          ref="variableFormElement"
          :state="variableForm"
          :validate="validateVariable"
          class="space-y-4"
          @submit="saveVariable"
        >
          <UFormField label="Variable Name" name="name" required
            ><UInput v-model="variableForm.name" placeholder="API_URL" class="w-full font-mono"
          /></UFormField>
          <UFormField label="Variable Value" name="value"
            ><UInput
              v-model="variableForm.value"
              :type="showDraftValue ? 'text' : 'password'"
              autocomplete="off"
              class="w-full"
          /></UFormField>
          <USwitch
            v-model="showDraftValue"
            label="Show Variable Value"
            description="Reveal only while you need to inspect or edit this value."
          />
        </UForm>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          @click="editingVariable = false" /><UButton
          type="submit"
          form="function-variable"
          label="Save Variable"
      /></template>
    </UModal>
    <UModal
      :open="!!removingDraftItem"
      title="Remove from Deployment Draft?"
      description="This removes the selected item from this draft only. The active deployment is unchanged until you activate a new version."
      :ui="{ footer: 'justify-end' }"
      @update:open="
        (open) => {
          if (!open) removingDraftItem = null;
        }
      "
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          @click="removingDraftItem = null" /><UButton
          label="Remove from Draft"
          color="error"
          @click="removeDraftItem"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmingDeploy"
      title="Activate New Deployment?"
      description="Confirm the bundle and configuration below. Activation replaces the endpoint's current version immediately."
      :dismissible="!functionControlPlane.busy.value"
      :ui="{ footer: 'justify-end' }"
    >
      <template #body>
        <div class="space-y-5">
          <UAlert
            v-if="functionControlPlane.error.value"
            title="Deployment Failed"
            :description="functionControlPlane.error.value"
            color="error"
          />
          <dl class="grid gap-4 sm:grid-cols-2">
            <div>
              <dt class="text-sm text-muted">Endpoint</dt>
              <dd>{{ functionControlPlane.selectedEndpoint.value?.name }}</dd>
            </div>
            <div>
              <dt class="text-sm text-muted">Entrypoint</dt>
              <dd class="font-mono">
                {{ functionControlPlane.selectedEndpoint.value?.entrypoint }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted">Bundle</dt>
              <dd>
                {{
                  deploymentForm.bundleSource === "archive"
                    ? deploymentForm.archive?.name
                    : `${deploymentForm.files.length} source files`
                }}
              </dd>
            </div>
            <div v-if="deploymentForm.bundleSource === 'archive' && deploymentForm.archive">
              <dt class="text-sm text-muted">Archive</dt>
              <dd>
                {{ deploymentForm.archive.format.toUpperCase() }} ·
                {{ deploymentForm.archive.size.toLocaleString() }} bytes
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted">Compatibility</dt>
              <dd>
                {{
                  deploymentForm.overrideRuntime
                    ? deploymentForm.compatibilityDate
                    : "Service defaults"
                }}
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted">Compatibility Flags</dt>
              <dd>
                {{
                  deploymentForm.overrideRuntime && deploymentForm.overrideFlags
                    ? deploymentForm.compatibilityFlags.join(", ") || "No explicit flags"
                    : "Service defaults"
                }}
              </dd>
            </div>
          </dl>
          <UTable
            v-if="deploymentForm.bundleSource === 'files'"
            :data="draftManifest"
            :columns="[
              { accessorKey: 'path', header: 'File' },
              { accessorKey: 'bytes', header: 'Bytes' },
            ]"
          />
          <p v-else class="text-sm text-muted">
            The server validates archive contents and the configured entrypoint before activation.
          </p>
          <USeparator label="Environment" position="start" />
          <UTable
            :data="deploymentForm.variables.map((variable) => ({ name: variable.name }))"
            :columns="[{ accessorKey: 'name', header: 'Variable Name' }]"
            empty="No environment variables will be set."
          />
          <UAlert
            v-if="removedVariables.length"
            title="Variables Will Be Removed"
            :description="removedVariables.join(', ')"
            color="warning"
          />
        </div>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="confirmingDeploy = false" /><UButton
          label="Activate Deployment"
          :loading="functionControlPlane.busy.value"
          @click="deploy"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmingRollback"
      title="Roll Back Deployment?"
      description="The selected version's code and environment will become active immediately. External side effects are not undone."
      :dismissible="!functionControlPlane.busy.value"
      :ui="{ footer: 'justify-end' }"
    >
      <template #body
        ><UAlert
          v-if="functionControlPlane.error.value"
          title="Rollback Failed"
          :description="functionControlPlane.error.value"
          color="error"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="functionControlPlane.busy.value"
          @click="confirmingRollback = false" /><UButton
          label="Confirm Rollback"
          :loading="functionControlPlane.busy.value"
          @click="rollback"
      /></template>
    </UModal>
  </div>
</template>
