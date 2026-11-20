<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import { validFileStorageKey } from "../../utils/file-storage-forms";
const auth = useAuth();
const client = useClient();
const route = useRoute();
const fileStorageControlPlane = useFileStorageControlPlane(client.value.file_storage);
const form = reactive({ bucket: "", key: "", operation: "read", expiresInSeconds: 0 });
const formTemplateRef = useTemplateRef("signingFormElement");
const signedUrl = computed(() =>
  fileStorageControlPlane.signedUrl.value
    ? new URL(fileStorageControlPlane.signedUrl.value.url, client.value.endpoint).href
    : "",
);
watch(form, () => {
  fileStorageControlPlane.signedUrl.value = null;
});
function validate(): FormError[] {
  const errors: FormError[] = [];
  if (!/^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/.test(form.bucket.trim()))
    errors.push({ name: "bucket", message: "Enter the name of an existing bucket." });
  if (!validFileStorageKey(form.key.trim()))
    errors.push({
      name: "key",
      message: "Enter a relative object key without empty, . or .. path segments.",
    });
  if (
    !Number.isSafeInteger(form.expiresInSeconds) ||
    form.expiresInSeconds < 0 ||
    form.expiresInSeconds > 604800
  )
    errors.push({
      name: "expiresInSeconds",
      message: "Use 0 for the service default, or 1 to 604800 seconds (7 days).",
    });
  if (
    !["read", "write"].includes(form.operation) ||
    !auth.hasScope(`storage:object.${form.operation}`)
  )
    errors.push({
      name: "operation",
      message: "Choose an operation allowed by your Service Account.",
    });
  return errors;
}
async function sign() {
  const errors = validate();
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  await fileStorageControlPlane.sign({ ...form, bucket: form.bucket.trim(), key: form.key.trim() });
}
onMounted(() => {
  if (typeof route.query.bucket === "string") form.bucket = route.query.bucket;
  if (typeof route.query.key === "string") form.key = route.query.key;
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
    <UCard
      title="File Storage URL Signing"
      description="Create a temporary capability URL for an exact bucket, object key, and operation. A read URL does not require making the bucket public."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="storage-signing"
        ref="signingFormElement"
        :state="form"
        :validate="validate"
        :disabled="fileStorageControlPlane.busy.value"
        class="space-y-5"
        @submit="sign"
      >
        <UFormField
          label="Bucket Name"
          name="bucket"
          description="Use an existing bucket name. Listing buckets is not required to sign a known path."
          required
          ><UInput v-model="form.bucket" class="w-full"
        /></UFormField>
        <UFormField
          label="Object Key"
          name="key"
          description="For example, documents/report.pdf. Read signing does not check whether the object exists."
          required
          ><UInput v-model="form.key" class="w-full"
        /></UFormField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            label="Operation"
            name="operation"
            description="The HTTP operation this signed URL permits."
            required
            ><USelect
              v-model="form.operation"
              :items="[
                { label: 'Read (GET)', value: 'read' },
                ...(auth.hasScope('storage:object.write')
                  ? [{ label: 'Write (PUT)', value: 'write' }]
                  : []),
              ]"
              class="w-full" /></UFormField
          ><UFormField
            label="URL Lifetime (Seconds)"
            name="expiresInSeconds"
            description="Use 0 for the configured service default, or 1 second to 7 days."
            required
            ><UInputNumber v-model="form.expiresInSeconds" :min="0" :max="604800" class="w-full"
          /></UFormField>
        </div>
        <UAlert
          v-if="form.operation === 'write'"
          title="Write Access Can Replace Files"
          description="Anyone holding a write URL can upload to this key or delete its object until the URL expires. Share it only with trusted recipients."
          color="warning"
        />
      </UForm>
      <template #footer
        ><UButton
          v-if="auth.hasScope('storage:object.read')"
          type="submit"
          form="storage-signing"
          label="Sign URL"
          :loading="fileStorageControlPlane.busy.value"
      /></template>
    </UCard>
    <UCard
      v-if="fileStorageControlPlane.signedUrl.value"
      title="Signed File Storage URL"
      description="Treat this URL as a secret. Anyone with it can perform the granted operation until it expires."
      :ui="{ body: 'space-y-5' }"
    >
      <UFormField label="Signed URL"
        ><UTextarea :model-value="signedUrl" readonly :rows="3" class="w-full"
      /></UFormField>
      <dl class="grid gap-4 text-sm sm:grid-cols-2">
        <div>
          <dt class="text-muted">Operation</dt>
          <dd>{{ form.operation === "read" ? "Read (GET)" : "Write (PUT)" }}</dd>
        </div>
        <div>
          <dt class="text-muted">Expires At</dt>
          <dd>{{ fileStorageControlPlane.signedUrl.value.expiresAt.toLocaleString() }}</dd>
        </div>
      </dl>
    </UCard>
  </div>
</template>
