<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const imageControlPlane = useImageControlPlane(client.value.image.cp);
const signingForm = reactive({ path: "" });
async function sign() {
  await imageControlPlane.signUrl(signingForm.path);
}
</script>

<template>
  <div class="space-y-6" :aria-busy="imageControlPlane.busy.value">
    <UAlert
      v-if="imageControlPlane.error.value"
      title="Request Failed"
      :description="imageControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="imageControlPlane.notice.value"
      :title="imageControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="URL Signing"
      description="Generate a signed image URL using the service's signing key without exposing it to the browser."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UForm
        id="image-signing"
        :state="signingForm"
        :disabled="imageControlPlane.busy.value || !auth.hasScope('image:sign.write')"
        @submit="sign"
      >
        <UFormField
          label="Image Path"
          description="Enter the processing options and source path to sign, without the /v1/image prefix or a signature."
          required
          ><UInput
            v-model="signingForm.path"
            placeholder="/rs:fill:320:240/plain/https://example.com/image.jpg"
            required
            class="w-full"
        /></UFormField>
      </UForm>
      <template v-if="auth.hasScope('image:sign.write')" #footer
        ><UButton
          type="submit"
          form="image-signing"
          label="Sign URL"
          :loading="imageControlPlane.busy.value"
      /></template>
    </UCard>
    <UCard
      v-if="imageControlPlane.signedUrl.value"
      title="Signed Image URL"
      description="The URL is relative to the Image service endpoint. Anyone with this URL may request the image, subject to source access policies."
      :ui="{ body: 'space-y-5' }"
    >
      <UFormField label="Signed URL"
        ><UInput :model-value="imageControlPlane.signedUrl.value.url" readonly class="w-full"
      /></UFormField>
      <UFormField label="Signature"
        ><UInput :model-value="imageControlPlane.signedUrl.value.signature" readonly class="w-full"
      /></UFormField>
    </UCard>
  </div>
</template>
