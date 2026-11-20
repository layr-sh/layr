<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { FileStorageControlPlaneModels } from "layr-client";
import {
  resolveFileStorageBucket,
  fileStorageBytes,
  type FileStorageBucketForm,
} from "../../utils/file-storage-forms";
const auth = useAuth();
const client = useClient();
const fileStorageControlPlane = useFileStorageControlPlane(client.value.file_storage);
const editing = ref(false);
const editingName = ref("");
const deleting = ref<FileStorageControlPlaneModels.Bucket | null>(null);
const confirmationName = ref("");
const formTemplateRef = useTemplateRef("bucketFormElement");
const form = reactive<FileStorageBucketForm>({
  name: "",
  backend: "database",
  isPublic: false,
  maxFileSizeBytes: 52428800,
  allowedMimeTypes: [],
  endpoint: "",
  region: "us-east-1",
  upstreamBucket: "",
  accessKeyId: "",
  secretAccessKey: "",
});
const columns: TableColumn<FileStorageControlPlaneModels.Bucket>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "backend", header: "Backend" },
  { accessorKey: "isPublic", header: "Access" },
  { accessorKey: "maxFileSizeBytes", header: "File Limit" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions", header: "" },
];
async function newBucket() {
  if (
    auth.hasScope("storage:config.read") &&
    !(await fileStorageControlPlane.load("configuration"))
  )
    return;
  editingName.value = "";
  fileStorageControlPlane.bucket.value = null;
  fileStorageControlPlane.error.value = "";
  Object.assign(form, {
    name: "",
    backend: "database",
    isPublic: false,
    maxFileSizeBytes: fileStorageControlPlane.config.value?.defaultMaxFileSizeBytes || 52428800,
    allowedMimeTypes: [...(fileStorageControlPlane.config.value?.defaultAllowedMimeTypes || [])],
    endpoint: "",
    region: "us-east-1",
    upstreamBucket: "",
    accessKeyId: "",
    secretAccessKey: "",
  });
  editing.value = true;
}
async function details(name: string) {
  if (await fileStorageControlPlane.getBucket(name)) {
    const bucket = fileStorageControlPlane.bucket.value!;
    const config = bucket.backendConfig || {};
    editingName.value = bucket.name;
    Object.assign(form, {
      name: bucket.name,
      backend: bucket.backend,
      isPublic: bucket.isPublic,
      maxFileSizeBytes: bucket.maxFileSizeBytes,
      allowedMimeTypes: [...(bucket.allowedMimeTypes || [])],
      endpoint: config.endpoint || "",
      region: config.region || "us-east-1",
      upstreamBucket: config.bucket || "",
      accessKeyId: config.access_key_id || "",
      secretAccessKey: "",
    });
    editing.value = true;
  }
}
function validate(): FormError[] {
  return resolveFileStorageBucket(form, !editingName.value).errors;
}
async function save() {
  const { input, errors } = resolveFileStorageBucket(form, !editingName.value);
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  if (await fileStorageControlPlane.saveBucket(input, editingName.value)) editing.value = false;
}
function confirmDelete(bucket: FileStorageControlPlaneModels.Bucket) {
  fileStorageControlPlane.error.value = "";
  confirmationName.value = "";
  deleting.value = bucket;
}
async function deleteBucket() {
  if (!deleting.value || confirmationName.value !== deleting.value.name) return;
  if (await fileStorageControlPlane.deleteBucket(deleting.value.name)) deleting.value = null;
}
onMounted(() => {
  if (auth.hasScope("storage:bucket.read")) return fileStorageControlPlane.load("buckets");
});
</script>
<template>
  <div class="space-y-6" :aria-busy="fileStorageControlPlane.busy.value">
    <UAlert
      v-if="fileStorageControlPlane.error.value"
      title="Request Failed"
      :description="fileStorageControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="fileStorageControlPlane.notice.value"
      :title="fileStorageControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="File Storage Buckets"
      description="Organize files with independent access policies, upload limits, and database or S3 storage."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="fileStorageControlPlane.buckets.value?.data || []"
        :columns="columns"
        :loading="fileStorageControlPlane.busy.value"
        :inert="fileStorageControlPlane.busy.value"
        :ui="{ td: 'whitespace-normal align-top' }"
        caption="File Storage buckets and management actions."
        @select="(_event, row) => details(row.original.name)"
      >
        <template #name-cell="{ row }"
          ><span class="font-medium text-highlighted">{{ row.original.name }}</span></template
        >
        <template #backend-cell="{ row }">{{
          row.original.backend === "s3" ? "S3" : "Database"
        }}</template>
        <template #isPublic-cell="{ row }"
          ><UBadge
            :label="row.original.isPublic ? 'Public' : 'Private'"
            color="neutral"
            variant="subtle"
        /></template>
        <template #maxFileSizeBytes-cell="{ row }">{{
          fileStorageBytes(row.original.maxFileSizeBytes)
        }}</template>
        <template #createdAt-cell="{ row }">{{ row.original.createdAt.toLocaleString() }}</template>
        <template #actions-cell="{ row }"
          ><div class="flex justify-end">
            <UDropdownMenu
              :items="[
                {
                  label: 'Details',
                  icon: 'i-lucide-info',
                  onSelect: () => details(row.original.name),
                },
                {
                  label: 'Browse Objects',
                  icon: 'i-lucide-files',
                  to: { path: '/file-storage/objects', query: { bucket: row.original.name } },
                },
                ...(auth.hasScope('storage:bucket.write')
                  ? [
                      { type: 'separator' as const },
                      {
                        label: 'Delete Bucket',
                        icon: 'i-lucide-trash-2',
                        color: 'error' as const,
                        onSelect: () => confirmDelete(row.original),
                      },
                    ]
                  : []),
              ]"
              :content="{ align: 'end' }"
              ><UButton
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                :disabled="fileStorageControlPlane.busy.value"
                :aria-label="`Actions for ${row.original.name}`"
            /></UDropdownMenu></div
        ></template>
        <template #empty
          ><UEmpty
            title="No File Storage Buckets"
            description="Create a bucket to start storing files."
            icon="i-lucide-folder"
        /></template>
        <template #loading
          ><UEmpty title="Loading Buckets…" loading icon="i-lucide-folder"
        /></template>
      </UTable>
      <template v-if="auth.hasScope('storage:bucket.write')" #footer
        ><UButton
          label="New Bucket"
          icon="i-lucide-plus"
          :disabled="fileStorageControlPlane.busy.value"
          @click="newBucket"
      /></template>
    </UCard>
    <UModal
      v-model:open="editing"
      :title="editingName ? 'Bucket Details' : 'New Bucket'"
      description="Configure storage, public access, and upload restrictions. Bucket names cannot be changed."
      :dismissible="!fileStorageControlPlane.busy.value"
      :close="{ disabled: fileStorageControlPlane.busy.value }"
      :ui="{ content: 'max-w-4xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          v-if="fileStorageControlPlane.error.value"
          title="Request Failed"
          :description="fileStorageControlPlane.error.value"
          color="error"
          role="alert"
        />
        <UForm
          id="storage-bucket"
          ref="bucketFormElement"
          :state="form"
          :validate="validate"
          :disabled="fileStorageControlPlane.busy.value || !auth.hasScope('storage:bucket.write')"
          class="space-y-6"
          @submit="save"
        >
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField
              label="Bucket Name"
              name="name"
              description="3–63 lowercase letters, numbers, or hyphens."
              required
              ><UInput v-model="form.name" :disabled="!!editingName" class="w-full"
            /></UFormField>
            <UFormField
              label="File Storage Backend"
              name="backend"
              description="Backend changes do not migrate files. Create a new bucket to use a different backend."
              required
              ><USelect
                v-model="form.backend"
                :items="[
                  { label: 'Database', value: 'database' },
                  { label: 'S3', value: 's3' },
                ]"
                :disabled="!!editingName"
                class="w-full"
            /></UFormField>
          </div>
          <section class="space-y-5">
            <header class="space-y-2">
              <USeparator label="Access and Upload Limits" position="start" />
              <p class="text-sm text-muted">
                These settings apply to this bucket. Making it public permits unauthenticated
                downloads, not uploads.
              </p>
            </header>
            <USwitch
              v-model="form.isPublic"
              label="Public Bucket"
              description="Allow anyone with an object's URL to read it."
            />
            <UFormField
              label="Maximum File Size (Bytes)"
              name="maxFileSizeBytes"
              description="Set an explicit per-file limit for this bucket."
              required
              ><UInputNumber v-model="form.maxFileSizeBytes" :min="1" class="w-full"
            /></UFormField>
            <UFormField
              label="Allowed MIME Types"
              name="allowedMimeTypes"
              description="Press Enter to add a MIME type or wildcard, such as image/png or image/*. Empty or */* permits all types."
              ><UInputTags
                v-model="form.allowedMimeTypes"
                :delimiter="/[\n,]/"
                add-on-paste
                add-on-blur
                placeholder="image/png"
                class="w-full"
            /></UFormField>
          </section>
          <section v-if="form.backend === 's3'" class="space-y-5">
            <header class="space-y-2">
              <USeparator label="S3 Connection" position="start" />
              <p class="text-sm text-muted">
                Use an existing upstream bucket. Saving these settings does not provision or
                validate the upstream connection.
              </p>
            </header>
            <UFormField
              label="S3 Endpoint"
              name="endpoint"
              description="Optional for AWS S3. For compatible providers, enter the full HTTP or HTTPS endpoint."
              ><UInput v-model="form.endpoint" placeholder="https://s3.example.com" class="w-full"
            /></UFormField>
            <div class="grid gap-4 sm:grid-cols-2">
              <UFormField
                label="S3 Bucket"
                description="Leave blank to use the File Storage bucket name."
                ><UInput v-model="form.upstreamBucket" class="w-full" /></UFormField
              ><UFormField
                label="S3 Region"
                name="region"
                description="The upstream bucket's region, for example us-east-1."
                required
                ><UInput v-model="form.region" class="w-full"
              /></UFormField>
            </div>
            <UFormField label="S3 Access Key ID" name="accessKeyId" required
              ><UInput v-model="form.accessKeyId" autocomplete="off" class="w-full"
            /></UFormField>
            <UFormField
              label="S3 Secret Access Key"
              name="secretAccessKey"
              :description="
                editingName
                  ? 'Leave blank to preserve the configured secret. Stored secrets are never returned.'
                  : 'The server encrypts this secret at rest.'
              "
              :required="!editingName"
              ><UInput
                v-model="form.secretAccessKey"
                type="password"
                autocomplete="new-password"
                class="w-full"
            /></UFormField>
          </section>
        </UForm>
        <section v-if="fileStorageControlPlane.bucket.value" class="space-y-5">
          <USeparator label="Bucket Metadata" position="start" />
          <dl class="grid gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-muted">Bucket ID</dt>
              <dd class="break-all font-mono">{{ fileStorageControlPlane.bucket.value.id }}</dd>
            </div>
            <div>
              <dt class="text-muted">Last Updated</dt>
              <dd>{{ fileStorageControlPlane.bucket.value.lastUpdatedAt.toLocaleString() }}</dd>
            </div>
          </dl>
        </section>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="editing = false" /><UButton
          v-if="auth.hasScope('storage:bucket.write')"
          type="submit"
          form="storage-bucket"
          :label="editingName ? 'Save Bucket' : 'Create Bucket'"
          :loading="fileStorageControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      :open="!!deleting"
      title="Delete File Storage Bucket?"
      :description="`Permanently remove ${deleting?.name} and its object metadata. Database file contents and registered upstream S3 objects are deleted. Untracked upstream files are not affected. This cannot be undone.`"
      :dismissible="!fileStorageControlPlane.busy.value"
      :close="{ disabled: fileStorageControlPlane.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) deleting = null;
        }
      "
    >
      <template #body
        ><UAlert
          v-if="fileStorageControlPlane.error.value"
          title="Deletion Failed"
          :description="fileStorageControlPlane.error.value"
          color="error"
          role="alert" /><UFormField
          label="Confirm Bucket Name"
          description="Type the bucket name to confirm deletion."
          ><UInput
            v-model="confirmationName"
            :disabled="fileStorageControlPlane.busy.value"
            autocomplete="off"
            class="w-full" /></UFormField
      ></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="deleting = null" /><UButton
          label="Delete Bucket"
          color="error"
          :loading="fileStorageControlPlane.busy.value"
          :disabled="confirmationName !== deleting?.name"
          @click="deleteBucket"
      /></template>
    </UModal>
  </div>
</template>
