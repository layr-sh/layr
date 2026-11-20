<script setup lang="ts">
import type { FormError, TableColumn } from "@nuxt/ui";
import type { FileStorageControlPlaneModels } from "layr-client";
import { fileStorageBytes, validFileStorageKey } from "../../utils/file-storage-forms";
const auth = useAuth();
const client = useClient();
const route = useRoute();
const fileStorageControlPlane = useFileStorageControlPlane(client.value.file_storage);
const bucketName = ref("");
const filter = ref("");
const selectedObject = ref<FileStorageControlPlaneModels.ModelObject | null>(null);
const deleting = ref<FileStorageControlPlaneModels.ModelObject | null>(null);
const uploading = ref(false);
const reviewingUpload = ref(false);
const file = shallowRef<File | null>(null);
const uploadForm = reactive({ key: "", contentType: "application/octet-stream" });
const uploadFormTemplateRef = useTemplateRef("uploadFormElement");
const rows = computed(() =>
  (fileStorageControlPlane.objects.value?.data || []).filter((object) =>
    object.objectKey.toLowerCase().includes(filter.value.toLowerCase()),
  ),
);
const publicUrl = computed(
  () =>
    `${client.value.endpoint}/v1/file-storage/objects/${encodeURIComponent(bucketName.value)}/${(selectedObject.value?.objectKey || "").split("/").map(encodeURIComponent).join("/")}`,
);
const columns: TableColumn<FileStorageControlPlaneModels.ModelObject>[] = [
  { accessorKey: "objectKey", header: "Object Key" },
  { accessorKey: "contentType", header: "Content Type" },
  { accessorKey: "sizeBytes", header: "Size" },
  { accessorKey: "lastUpdatedAt", header: "Last Updated" },
  { id: "actions", header: "" },
];
async function chooseBucket(name: string) {
  bucketName.value = name;
  filter.value = "";
  selectedObject.value = null;
  deleting.value = null;
  uploading.value = false;
  reviewingUpload.value = false;
  file.value = null;
  await fileStorageControlPlane.selectBucket(name);
}
function newUpload() {
  fileStorageControlPlane.error.value = "";
  file.value = null;
  Object.assign(uploadForm, { key: "", contentType: "application/octet-stream" });
  uploading.value = true;
}
function selectFile(value: File | null | undefined) {
  file.value = value || null;
  uploadForm.key = value?.name || "";
  uploadForm.contentType = value?.type || "application/octet-stream";
}
function validateUpload(): FormError[] {
  const errors: FormError[] = [];
  if (!file.value) errors.push({ name: "file", message: "Choose a file to upload." });
  if (!validFileStorageKey(uploadForm.key.trim()))
    errors.push({
      name: "key",
      message:
        "Enter a relative object key without empty, . or .. path segments, backslashes, or control characters.",
    });
  if (!/^[^\s/]+\/[^\s/]+$/.test(uploadForm.contentType.trim()))
    errors.push({ name: "contentType", message: "Enter a MIME type such as application/pdf." });
  const bucket = fileStorageControlPlane.bucket.value;
  if (
    file.value &&
    bucket &&
    bucket.maxFileSizeBytes > 0 &&
    file.value.size > bucket.maxFileSizeBytes
  )
    errors.push({
      name: "file",
      message: `The file exceeds this bucket's ${fileStorageBytes(bucket.maxFileSizeBytes)} limit.`,
    });
  const allowed = bucket?.allowedMimeTypes || [];
  if (
    allowed.length &&
    !allowed.some(
      (type) =>
        type === "*/*" || type.toLowerCase() === uploadForm.contentType.trim().toLowerCase(),
    )
  )
    errors.push({ name: "contentType", message: "This MIME type is not allowed by the bucket." });
  return errors;
}
function reviewUpload() {
  if (!fileStorageControlPlane.bucket.value) return;
  const errors = validateUpload();
  if (errors.length) {
    uploadFormTemplateRef.value?.setErrors(errors);
    return;
  }
  reviewingUpload.value = true;
}
async function upload() {
  if (!file.value || !fileStorageControlPlane.bucket.value || validateUpload().length) return;
  if (
    await fileStorageControlPlane.upload(
      bucketName.value,
      uploadForm.key.trim(),
      file.value,
      uploadForm.contentType.trim(),
    )
  ) {
    reviewingUpload.value = false;
    uploading.value = false;
    file.value = null;
  }
}
function confirmDelete(object: FileStorageControlPlaneModels.ModelObject) {
  fileStorageControlPlane.error.value = "";
  deleting.value = object;
}
async function deleteObject() {
  if (!deleting.value || !fileStorageControlPlane.bucket.value) return;
  if (await fileStorageControlPlane.deleteObject(bucketName.value, deleting.value.objectKey))
    deleting.value = null;
}
onMounted(async () => {
  if (!auth.hasScope("storage:bucket.read")) return;
  await fileStorageControlPlane.load("buckets");
  if (typeof route.query.bucket === "string") await chooseBucket(route.query.bucket);
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
      title="Bucket Objects"
      description="Browse stored files by their full object keys. Uploading to an existing key replaces its contents."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UFormField label="Bucket" description="Choose the bucket whose files you want to manage."
        ><USelectMenu
          :model-value="bucketName"
          :items="fileStorageControlPlane.buckets.value?.data || []"
          value-key="name"
          label-key="name"
          placeholder="Choose a bucket"
          :disabled="fileStorageControlPlane.busy.value"
          class="w-full"
          @update:model-value="chooseBucket"
      /></UFormField>
      <template v-if="fileStorageControlPlane.bucket.value">
        <UFormField
          label="Filter Current Page"
          description="Matches object keys on this page only. Clear the filter or change pages to browse other objects."
          ><UInput
            v-model="filter"
            icon="i-lucide-search"
            placeholder="Filter object keys…"
            class="w-full"
        /></UFormField>
        <UTable
          :data="rows"
          :columns="columns"
          :loading="fileStorageControlPlane.busy.value"
          :inert="fileStorageControlPlane.busy.value"
          :ui="{ td: 'whitespace-normal align-top' }"
          caption="Objects in this bucket with file actions."
          @select="(_event, row) => (selectedObject = row.original)"
        >
          <template #objectKey-cell="{ row }"
            ><span class="break-all font-medium text-highlighted">{{
              row.original.objectKey
            }}</span></template
          >
          <template #sizeBytes-cell="{ row }">{{
            fileStorageBytes(row.original.sizeBytes)
          }}</template>
          <template #lastUpdatedAt-cell="{ row }">{{
            row.original.lastUpdatedAt.toLocaleString()
          }}</template>
          <template #actions-cell="{ row }"
            ><div class="flex justify-end">
              <UDropdownMenu
                :items="[
                  {
                    label: 'Details',
                    icon: 'i-lucide-info',
                    onSelect: () => (selectedObject = row.original),
                  },
                  ...(auth.hasScope('storage:object.read')
                    ? [
                        {
                          label: 'Download',
                          icon: 'i-lucide-download',
                          onSelect: () =>
                            fileStorageControlPlane.download(bucketName, row.original.objectKey),
                        },
                        {
                          label: 'Create Signed URL',
                          icon: 'i-lucide-link',
                          to: {
                            path: '/file-storage/signing',
                            query: { bucket: bucketName, key: row.original.objectKey },
                          },
                        },
                      ]
                    : []),
                  ...(auth.hasScope('storage:object.write')
                    ? [
                        { type: 'separator' as const },
                        {
                          label: 'Delete Object',
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
                  :aria-label="`Actions for ${row.original.objectKey}`"
              /></UDropdownMenu></div
          ></template>
          <template #empty
            ><UEmpty
              :title="filter ? 'No Matching Objects' : 'No Stored Objects'"
              description="Clear the current-page filter, change pages, or upload a file."
              icon="i-lucide-files"
          /></template>
          <template #loading
            ><UEmpty title="Loading Objects…" loading icon="i-lucide-files"
          /></template>
        </UTable>
      </template>
      <UEmpty
        v-else
        title="Choose a Bucket"
        description="Select a bucket to view its objects and upload restrictions."
        icon="i-lucide-folder"
        :loading="fileStorageControlPlane.busy.value"
      />
      <template v-if="fileStorageControlPlane.bucket.value" #footer>
        <p class="mr-auto text-sm text-muted">
          {{ fileStorageControlPlane.objects.value?.total || 0 }} objects · Page
          {{ Math.floor(fileStorageControlPlane.offset.value / fileStorageControlPlane.limit) + 1 }}
        </p>
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="
            fileStorageControlPlane.listObjects(bucketName, fileStorageControlPlane.offset.value)
          "
        />
        <UButton
          label="Previous"
          color="neutral"
          variant="outline"
          :disabled="
            fileStorageControlPlane.busy.value || fileStorageControlPlane.offset.value === 0
          "
          @click="
            fileStorageControlPlane.listObjects(
              bucketName,
              fileStorageControlPlane.offset.value - fileStorageControlPlane.limit,
            )
          "
        />
        <UButton
          label="Next"
          color="neutral"
          variant="outline"
          :disabled="
            fileStorageControlPlane.busy.value ||
            fileStorageControlPlane.offset.value + fileStorageControlPlane.limit >=
              (fileStorageControlPlane.objects.value?.total || 0)
          "
          @click="
            fileStorageControlPlane.listObjects(
              bucketName,
              fileStorageControlPlane.offset.value + fileStorageControlPlane.limit,
            )
          "
        />
        <UButton
          v-if="auth.hasScope('storage:object.write')"
          label="Upload File"
          icon="i-lucide-upload"
          :disabled="fileStorageControlPlane.busy.value"
          @click="newUpload"
        />
      </template>
    </UCard>
    <UModal
      v-if="selectedObject"
      :open="!!selectedObject"
      title="Object Details"
      description="Stored metadata for this object. Object keys cannot be renamed; upload a copy under a new key instead."
      :ui="{ content: 'max-w-4xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) selectedObject = null;
        }
      "
    >
      <template #body>
        <dl class="grid gap-4 text-sm sm:grid-cols-2">
          <div class="sm:col-span-2">
            <dt class="text-muted">Object Key</dt>
            <dd class="break-all font-mono">{{ selectedObject.objectKey }}</dd>
          </div>
          <div>
            <dt class="text-muted">Content Type</dt>
            <dd>{{ selectedObject.contentType }}</dd>
          </div>
          <div>
            <dt class="text-muted">Size</dt>
            <dd>{{ selectedObject.sizeBytes.toLocaleString() }} bytes</dd>
          </div>
          <div>
            <dt class="text-muted">Created</dt>
            <dd>{{ selectedObject.createdAt.toLocaleString() }}</dd>
          </div>
          <div>
            <dt class="text-muted">Last Updated</dt>
            <dd>{{ selectedObject.lastUpdatedAt.toLocaleString() }}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-muted">SHA-256 Checksum</dt>
            <dd class="break-all font-mono">{{ selectedObject.checksumSha256 }}</dd>
          </div>
        </dl>
        <section class="space-y-5">
          <USeparator label="Custom Metadata" position="start" />
          <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{
            JSON.stringify(selectedObject.metadata || {}, null, 2)
          }}</pre>
        </section>
        <section v-if="fileStorageControlPlane.bucket.value?.isPublic" class="space-y-5">
          <header class="space-y-2">
            <USeparator label="Public URL" position="start" />
            <p class="text-sm text-muted">
              Anyone with this URL can read the object while the bucket remains public.
            </p>
          </header>
          <UInput :model-value="publicUrl" readonly aria-label="Public Object URL" class="w-full" />
        </section>
      </template>
      <template #footer
        ><UButton label="Close" color="neutral" variant="outline" @click="selectedObject = null"
      /></template>
    </UModal>
    <UModal
      v-model:open="uploading"
      title="Upload File Storage Object"
      description="Choose a file and its object key. Review the destination before uploading; existing contents at that key will be replaced."
      :dismissible="!fileStorageControlPlane.busy.value"
      :close="{ disabled: fileStorageControlPlane.busy.value }"
      :ui="{ content: 'max-w-3xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body
        ><UForm
          id="storage-upload"
          ref="uploadFormElement"
          :state="uploadForm"
          :validate="validateUpload"
          :disabled="fileStorageControlPlane.busy.value"
          class="space-y-5"
          @submit="reviewUpload"
          ><UFormField label="File" name="file" required
            ><UFileUpload
              :model-value="file"
              label="Drop a file here or choose a file"
              :description="`Bucket: ${bucketName}`"
              layout="list"
              :file-image="false"
              :disabled="fileStorageControlPlane.busy.value"
              @update:model-value="selectFile" /></UFormField
          ><UFormField
            label="Object Key"
            name="key"
            description="A relative path within this bucket, such as documents/report.pdf."
            required
            ><UInput v-model="uploadForm.key" class="w-full" /></UFormField
          ><UFormField
            label="Content Type"
            name="contentType"
            description="Detected from the selected file when available. Otherwise, specify its MIME type."
            required
            ><UInput v-model="uploadForm.contentType" class="w-full" /></UFormField></UForm
      ></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="uploading = false" /><UButton
          type="submit"
          form="storage-upload"
          label="Review Upload"
          :disabled="fileStorageControlPlane.busy.value"
      /></template>
    </UModal>
    <UModal
      v-model:open="reviewingUpload"
      title="Confirm Object Upload?"
      description="If an object already exists at this key, uploading replaces it permanently. The API does not provide a create-only upload option."
      :dismissible="!fileStorageControlPlane.busy.value"
      :close="{ disabled: fileStorageControlPlane.busy.value }"
      :ui="{ content: 'max-w-3xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body
        ><UAlert
          v-if="fileStorageControlPlane.error.value"
          title="Upload Failed"
          :description="fileStorageControlPlane.error.value"
          color="error"
          role="alert"
        />
        <dl class="space-y-4 text-sm">
          <div>
            <dt class="text-muted">Destination</dt>
            <dd class="break-all font-mono">{{ bucketName }}/{{ uploadForm.key.trim() }}</dd>
          </div>
          <div>
            <dt class="text-muted">File</dt>
            <dd>{{ file?.name }} · {{ fileStorageBytes(file?.size || 0) }}</dd>
          </div>
          <div>
            <dt class="text-muted">Content Type</dt>
            <dd>{{ uploadForm.contentType }}</dd>
          </div>
        </dl></template
      >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="reviewingUpload = false" /><UButton
          label="Upload Object"
          :loading="fileStorageControlPlane.busy.value"
          @click="upload"
      /></template>
    </UModal>
    <UModal
      :open="!!deleting"
      title="Delete File Storage Object?"
      :description="`Permanently delete ${deleting?.objectKey} from ${bucketName}, including its stored contents. This cannot be undone.`"
      :dismissible="!fileStorageControlPlane.busy.value"
      :close="{ disabled: fileStorageControlPlane.busy.value }"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="
        (open) => {
          if (!open) deleting = null;
        }
      "
    >
      <template v-if="fileStorageControlPlane.error.value" #body
        ><UAlert
          title="Deletion Failed"
          :description="fileStorageControlPlane.error.value"
          color="error"
          role="alert"
      /></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="deleting = null" /><UButton
          label="Delete Object"
          color="error"
          :loading="fileStorageControlPlane.busy.value"
          @click="deleteObject"
      /></template>
    </UModal>
  </div>
</template>
