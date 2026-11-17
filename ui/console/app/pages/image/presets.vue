<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { ImageControlPlaneModels } from "layr-client";

const columns: TableColumn<ImageControlPlaneModels.Preset>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "description", header: "Description" },
  { accessorKey: "processingOptions", header: "Processing Options" },
];
const auth = useAuth();
const client = useClient();
const imageControlPlane = useImageControlPlane(client.value.image.cp);
const selectedPresetId = ref("");
const editing = ref(false);
const confirmingDelete = ref(false);
const presetForm = reactive({ name: "", description: "", processingOptions: "" });
function newPreset() {
  selectedPresetId.value = "";
  imageControlPlane.selectedPreset.value = null;
  Object.assign(presetForm, { name: "", description: "", processingOptions: "" });
  editing.value = true;
}
async function editPreset() {
  if (!selectedPresetId.value) return;
  if (await imageControlPlane.selectPreset(selectedPresetId.value)) {
    Object.assign(presetForm, {
      name: imageControlPlane.selectedPreset.value!.name,
      description: imageControlPlane.selectedPreset.value!.description || "",
      processingOptions: imageControlPlane.selectedPreset.value!.processingOptions,
    });
    editing.value = true;
  }
}
async function savePreset() {
  if (
    await (imageControlPlane.selectedPreset.value
      ? imageControlPlane.updatePreset(imageControlPlane.selectedPreset.value.id, {
          name: presetForm.name.trim(),
          description: presetForm.description,
          processingOptions: presetForm.processingOptions.trim(),
        })
      : imageControlPlane.createPreset({
          name: presetForm.name.trim(),
          description: presetForm.description,
          processingOptions: presetForm.processingOptions.trim(),
        }))
  )
    editing.value = false;
}
async function deletePreset() {
  if (!selectedPresetId.value) return;
  if (await imageControlPlane.deletePreset(selectedPresetId.value)) {
    selectedPresetId.value = "";
    confirmingDelete.value = false;
    editing.value = false;
  }
}
onMounted(() => {
  if (auth.hasScope("image:preset.read")) return imageControlPlane.load("presets");
});
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
      title="Transformation Presets"
      description="Manage reusable named image processing options."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UTable
        :data="imageControlPlane.presets.value?.data || []"
        :columns="columns"
        :loading="imageControlPlane.busy.value"
        :get-row-id="(preset) => preset.id"
        :row-selection="{ [selectedPresetId]: true }"
        :ui="{ td: 'whitespace-normal align-top' }"
        caption="Transformation presets. Select a row to view, edit, or delete its preset."
        @select="(_event, row) => (selectedPresetId = row.original.id)"
      >
        <template #name-cell="{ row }">
          <span class="font-medium text-highlighted">{{ row.original.name }}</span>
        </template>
        <template #description-cell="{ row }">
          {{ row.original.description || "No description" }}
        </template>
        <template #processingOptions-cell="{ row }">
          <code class="break-all text-xs">{{ row.original.processingOptions }}</code>
        </template>
        <template #empty>
          <UEmpty
            title="No Presets Yet"
            description="Create a preset to reuse image transformation options."
            icon="i-lucide-sliders-horizontal"
          />
        </template>
        <template #loading>
          <UEmpty title="Loading Presets…" loading icon="i-lucide-sliders-horizontal" />
        </template>
      </UTable>
      <template #footer>
        <UButton
          v-if="auth.hasScope('image:preset.write')"
          label="Delete Preset"
          color="error"
          variant="outline"
          :disabled="imageControlPlane.busy.value || !selectedPresetId"
          @click="confirmingDelete = true"
        />
        <UButton
          label="Edit Preset"
          color="neutral"
          variant="outline"
          :disabled="imageControlPlane.busy.value || !selectedPresetId"
          @click="editPreset"
        />
        <UButton
          v-if="auth.hasScope('image:preset.write')"
          label="New Preset"
          icon="i-lucide-plus"
          :disabled="imageControlPlane.busy.value"
          @click="newPreset"
        />
      </template>
    </UCard>
    <UCard
      v-if="editing"
      :title="
        imageControlPlane.selectedPreset.value ? 'Manage Image Preset' : 'Create Image Preset'
      "
      description="Define a name, description, and imgproxy-compatible processing options."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="image-preset"
        :state="presetForm"
        :disabled="imageControlPlane.busy.value || !auth.hasScope('image:preset.write')"
        class="space-y-5"
        @submit="savePreset"
      >
        <UFormField label="Name" required
          ><UInput v-model="presetForm.name" required class="w-full"
        /></UFormField>
        <UFormField label="Description"
          ><UTextarea v-model="presetForm.description" class="w-full"
        /></UFormField>
        <UFormField
          label="Processing Options"
          description="Use the image service's processing option syntax, such as rs:fill:320:240/q:80."
          required
          ><UInput v-model="presetForm.processingOptions" required class="w-full"
        /></UFormField>
      </UForm>
      <template #footer>
        <UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="imageControlPlane.busy.value"
          @click="editing = false"
        />
        <UButton
          v-if="auth.hasScope('image:preset.write')"
          type="submit"
          form="image-preset"
          :label="imageControlPlane.selectedPreset.value ? 'Save Preset' : 'Create Preset'"
          :disabled="imageControlPlane.busy.value"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="confirmingDelete"
      title="Delete Image Preset?"
      description="Existing image URLs that reference this preset will stop resolving. This cannot be undone."
      :dismissible="!imageControlPlane.busy.value"
      :ui="{ footer: 'justify-end' }"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="imageControlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Preset"
          color="error"
          :loading="imageControlPlane.busy.value"
          @click="deletePreset"
      /></template>
    </UModal>
  </div>
</template>
