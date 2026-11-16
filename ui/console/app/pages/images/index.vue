<script setup lang="ts">
const { copy, copied } = useClipboard({ copiedDuring: 2000 });
const copiedPreset = ref("");

function copyPresetUrl(presetName: string) {
  copiedPreset.value = presetName;
  copy(`/v1/images/${presetName}`);
}

const presets = [
  { name: "avatar-small", format: "webp", width: 64, height: 64, fit: "cover" },
  { name: "thumbnail", format: "webp", width: 320, height: 240, fit: "contain" },
  { name: "hero-banner", format: "avif", width: 1280, height: 720, fit: "cover" },
];
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Images</h1>
        <p class="text-sm text-muted mt-1">Image transformation and optimization presets.</p>
      </div>

      <div class="flex items-center gap-3">
        <UButton
          label="New Preset"
          icon="i-lucide-plus"
          color="neutral"
          variant="solid"
          size="sm"
        />
      </div>
    </div>

    <!-- Presets -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-image" class="size-4 text-muted" />
          <span class="text-sm font-semibold text-highlighted">Transformation Presets</span>
        </div>
        <span class="text-xs text-muted font-mono">{{ presets.length }} presets</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="preset in presets"
          :key="preset.name"
          class="p-4 flex items-center justify-between hover:bg-accented/30 transition"
        >
          <div class="flex items-center gap-3">
            <div
              class="size-8 rounded bg-accented/50 border border-default flex items-center justify-center text-highlighted"
            >
              <UIcon name="i-lucide-sliders" class="size-4" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-mono text-sm font-semibold text-highlighted">{{
                  preset.name
                }}</span>
                <UBadge color="neutral" variant="outline" size="xs" class="text-[10px] uppercase">
                  {{ preset.format }}
                </UBadge>
              </div>
              <p class="text-xs text-dimmed mt-0.5">
                {{ preset.width }}&times;{{ preset.height }} &bull; fit: {{ preset.fit }}
              </p>
            </div>
          </div>

          <UButton
            :icon="copied && copiedPreset === preset.name ? 'i-lucide-check' : 'i-lucide-copy'"
            color="neutral"
            variant="ghost"
            size="xs"
            :label="copied && copiedPreset === preset.name ? 'Copied' : 'Copy URL'"
            @click="copyPresetUrl(preset.name)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
