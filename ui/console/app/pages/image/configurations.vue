<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const imageControlPlane = useImageControlPlane(client.value.image.cp);
const allowedDomains = ref("");
async function load() {
  if (!auth.hasScope("image:config.read")) return;
  await imageControlPlane.load("configuration");
  allowedDomains.value = (imageControlPlane.config.value?.allowedDomains || []).join("\n");
}
async function save() {
  if (!imageControlPlane.config.value) return;
  await imageControlPlane.saveConfig({
    ...imageControlPlane.config.value,
    allowedDomains: allowedDomains.value
      .split(/[\n,]/)
      .map((domain) => domain.trim())
      .filter(Boolean),
  });
}
onMounted(load);
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
      title="Image Configuration"
      description="Configure source access, image processing defaults, caching, and watermarks."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UForm
        v-if="imageControlPlane.config.value"
        id="image-configuration"
        :state="imageControlPlane.config.value"
        :disabled="imageControlPlane.busy.value || !auth.hasScope('image:config.write')"
        class="space-y-5"
        @submit="save"
      >
        <USwitch
          v-model="imageControlPlane.config.value.allowInsecure"
          label="Allow Unsigned Requests"
          description="Allow requests using the insecure signature. Keep disabled to require signed image URLs."
        />
        <UFormField
          label="Allowed Source Domains"
          description="One domain per line. Empty allows all domains; source URL security checks still apply."
          ><UTextarea v-model="allowedDomains" placeholder="images.example.com" class="w-full"
        /></UFormField>
        <div class="grid items-end gap-4 sm:grid-cols-2">
          <UFormField
            label="Default Quality"
            description="Output quality from 1 to 100, unless overridden in the image URL."
            required
            ><UInputNumber
              v-model="imageControlPlane.config.value.defaultQuality"
              :min="1"
              :max="100"
              required
              class="w-full"
          /></UFormField>
          <UFormField
            label="Maximum Source Resolution (Megapixels)"
            description="Reject source images larger than this number of megapixels."
            required
            ><UInputNumber
              v-model="imageControlPlane.config.value.maxSrcResolution"
              :min="1"
              required
              class="w-full"
          /></UFormField>
          <UFormField
            label="Maximum Animation Frames"
            description="Reject animated source images exceeding this frame limit."
            required
            ><UInputNumber
              v-model="imageControlPlane.config.value.maxAnimationFrames"
              :min="1"
              required
              class="w-full"
          /></UFormField>
          <UFormField
            label="Cache Lifetime (Seconds)"
            description="Sets the Cache-Control max-age for transformed images. Zero requires immediate revalidation."
            required
            ><UInputNumber
              v-model="imageControlPlane.config.value.cacheTtlSeconds"
              :min="0"
              required
              class="w-full"
          /></UFormField>
        </div>
        <USwitch
          v-model="imageControlPlane.config.value.jpegProgressive"
          label="Progressive JPEG"
          description="Encode JPEG images in progressive scans for gradual loading."
        />
        <USwitch
          v-model="imageControlPlane.config.value.pngInterlaced"
          label="Interlaced PNG"
          description="Encode PNG images with interlacing for gradual loading."
        />
        <USwitch
          v-model="imageControlPlane.config.value.stripMetadata"
          label="Strip Metadata"
          description="Remove embedded metadata from transformed images by default."
        />
        <USwitch
          v-model="imageControlPlane.config.value.keepCopyright"
          label="Keep Copyright"
          description="Retain supported copyright information when stripping metadata."
        />
        <USwitch
          v-model="imageControlPlane.config.value.stripColorProfile"
          label="Strip Color Profile"
          description="Remove embedded color profiles from transformed images by default."
        />
        <UFormField
          label="Watermark URL"
          description="Optional source URL for the default watermark."
          ><UInput v-model="imageControlPlane.config.value.watermarkUrl" type="url" class="w-full"
        /></UFormField>
        <UFormField
          label="Watermark Opacity"
          description="From 0 (transparent) to 1 (opaque)."
          required
          ><UInputNumber
            v-model="imageControlPlane.config.value.watermarkOpacity"
            :min="0"
            :max="1"
            :step="0.01"
            required
            class="w-full"
        /></UFormField>
      </UForm>
      <UEmpty
        v-else
        :title="
          imageControlPlane.busy.value
            ? 'Loading Image Configuration…'
            : 'Configuration Unavailable'
        "
        :loading="imageControlPlane.busy.value"
        icon="i-lucide-settings"
      />
      <template v-if="auth.hasScope('image:config.write')" #footer
        ><UButton
          type="submit"
          form="image-configuration"
          label="Save Image Config"
          :disabled="imageControlPlane.busy.value || !imageControlPlane.config.value"
      /></template>
    </UCard>
  </div>
</template>
