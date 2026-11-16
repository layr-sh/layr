<script setup lang="ts">
import type { FileStorageControlPlaneModels } from "layr-client";

const client = useClient();

const defaultBuckets = [
  { name: "default", isPublic: false, size: "0 B", objects: 0, region: "local" },
  { name: "public-assets", isPublic: true, size: "0 B", objects: 0, region: "local" },
];

const { data: bucketsData } = await useAsyncData("storage:buckets", () =>
  client.value.file_storage.cp.buckets.list().catch(() => null),
);

const buckets = computed(() => {
  const fetched = bucketsData.value?.data;
  if (fetched && fetched.length > 0) {
    return fetched.map((item: FileStorageControlPlaneModels.Bucket) => ({
      name: item.name,
      isPublic: item.isPublic ?? false,
      size: "0 B",
      objects: 0,
      region: "local",
    }));
  }
  return defaultBuckets;
});
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Storage</h1>
        <p class="text-sm text-muted mt-1">Buckets, objects, and file storage.</p>
      </div>

      <div class="flex items-center gap-3">
        <UButton
          label="Create Bucket"
          icon="i-lucide-folder-plus"
          color="neutral"
          variant="solid"
          size="sm"
        />
      </div>
    </div>

    <!-- Buckets List -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-hard-drive" class="size-4 text-muted" />
          <span class="text-sm font-semibold text-highlighted">Storage Buckets</span>
        </div>
        <span class="text-xs text-muted font-mono">{{ buckets.length }} buckets</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="bucket in buckets"
          :key="bucket.name"
          class="p-4 flex items-center justify-between hover:bg-accented/30 transition"
        >
          <div class="flex items-center gap-3">
            <div
              class="size-8 rounded bg-accented/50 border border-default flex items-center justify-center text-highlighted"
            >
              <UIcon name="i-lucide-folder" class="size-4" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-mono text-sm font-semibold text-highlighted">{{
                  bucket.name
                }}</span>
                <UBadge
                  :color="bucket.isPublic ? 'warning' : 'neutral'"
                  variant="subtle"
                  size="xs"
                  class="text-[10px]"
                >
                  {{ bucket.isPublic ? "Public" : "Private" }}
                </UBadge>
              </div>
              <p class="text-xs text-dimmed mt-0.5">
                {{ bucket.objects }} objects &bull; {{ bucket.size }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <UButton
              label="Upload"
              icon="i-lucide-upload"
              color="neutral"
              variant="outline"
              size="xs"
            />
            <UButton icon="i-lucide-ellipsis" color="neutral" variant="ghost" size="xs" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
