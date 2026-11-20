<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import { fileStorageMimeTypes, validFileStorageMimeTypes } from "../../utils/file-storage-forms";
const auth = useAuth();
const client = useClient();
const fileStorageControlPlane = useFileStorageControlPlane(client.value.file_storage);
const allowedTypes = ref<string[]>([]);
const formTemplateRef = useTemplateRef("configurationFormElement");
async function load() {
  if (!auth.hasScope("storage:config.read")) return;
  if (await fileStorageControlPlane.load("configuration"))
    allowedTypes.value = [...(fileStorageControlPlane.config.value!.defaultAllowedMimeTypes || [])];
}
function validate(): FormError[] {
  const config = fileStorageControlPlane.config.value;
  if (!config) return [];
  const errors: FormError[] = [];
  for (const key of [
    "defaultMaxFileSizeBytes",
    "chunkSizeBytes",
    "presignTokenExpirySeconds",
  ] as const)
    if (!Number.isSafeInteger(config[key]) || config[key] < 1)
      errors.push({ name: key, message: "Enter a positive whole number." });
  if (!validFileStorageMimeTypes(fileStorageMimeTypes(allowedTypes.value)))
    errors.push({
      name: "defaultAllowedMimeTypes",
      message: "Use MIME types such as image/png, image/*, or */*.",
    });
  return errors;
}
async function save() {
  if (!fileStorageControlPlane.config.value) return;
  const errors = validate();
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  await fileStorageControlPlane.saveConfig({
    ...fileStorageControlPlane.config.value,
    defaultAllowedMimeTypes: fileStorageMimeTypes(allowedTypes.value),
  });
}
onMounted(load);
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
      title="File Storage Configuration"
      description="Configure new-bucket defaults, upload chunks, and signed URL lifetimes."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        v-if="fileStorageControlPlane.config.value"
        id="storage-configuration"
        ref="configurationFormElement"
        :state="fileStorageControlPlane.config.value"
        :validate="validate"
        :disabled="fileStorageControlPlane.busy.value || !auth.hasScope('storage:config.write')"
        class="space-y-6"
        @submit="save"
      >
        <section class="space-y-5">
          <header class="space-y-2">
            <USeparator label="Runtime Defaults" position="start" />
            <p class="text-sm text-muted">
              Bucket defaults apply when settings are omitted on creation; existing buckets keep
              their policies. Chunk sizes apply to new uploads, and URL lifetimes apply when signing
              without an explicit lifetime.
            </p>
          </header>

          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField
              label="Default Maximum File Size (Bytes)"
              name="defaultMaxFileSizeBytes"
              required
              ><UInputNumber
                v-model="fileStorageControlPlane.config.value.defaultMaxFileSizeBytes"
                :min="1"
                class="w-full" /></UFormField
            ><UFormField label="Database Chunk Size (Bytes)" name="chunkSizeBytes" required
              ><UInputNumber
                v-model="fileStorageControlPlane.config.value.chunkSizeBytes"
                :min="1"
                class="w-full" /></UFormField
            ><UFormField
              label="Default URL Lifetime (Seconds)"
              name="presignTokenExpirySeconds"
              required
              ><UInputNumber
                v-model="fileStorageControlPlane.config.value.presignTokenExpirySeconds"
                :min="1"
                class="w-full"
            /></UFormField>
          </div>
          <UFormField
            label="Default Allowed MIME Types"
            name="defaultAllowedMimeTypes"
            description="MIME types, type wildcards such as image/*, or */*. Empty allows all content types."
            ><UInputTags
              v-model="allowedTypes"
              :delimiter="/[\n,]/"
              add-on-paste
              add-on-blur
              class="w-full"
          /></UFormField>
        </section>
      </UForm>
      <UEmpty
        v-else
        :title="
          fileStorageControlPlane.busy.value
            ? 'Loading File Storage Configuration…'
            : 'Configuration Unavailable'
        "
        :loading="fileStorageControlPlane.busy.value"
        icon="i-lucide-settings"
      />
      <template v-if="auth.hasScope('storage:config.write')" #footer
        ><UButton
          type="submit"
          form="storage-configuration"
          label="Save File Storage Config"
          :disabled="fileStorageControlPlane.busy.value || !fileStorageControlPlane.config.value"
      /></template>
    </UCard>
  </div>
</template>
