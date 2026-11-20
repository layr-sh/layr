<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { FileStorageControlPlaneModels } from "layr-client";
import { fileStorageBytes } from "../../utils/file-storage-forms";
const auth = useAuth();
const client = useClient();
const fileStorageControlPlane = useFileStorageControlPlane(client.value.file_storage);
const summary = computed(() => {
  const stats = fileStorageControlPlane.stats.value;
  if (!stats) return [];
  return [
    { label: "Total Buckets", value: stats.totalBuckets.toLocaleString() },
    { label: "Public Buckets", value: stats.publicBuckets.toLocaleString() },
    { label: "Private Buckets", value: stats.privateBuckets.toLocaleString() },
    { label: "Database Buckets", value: stats.databaseBuckets.toLocaleString() },
    { label: "S3 Buckets", value: stats.s3Buckets.toLocaleString() },
    { label: "Tracked Objects", value: stats.totalObjects.toLocaleString() },
    { label: "Logical File Size", value: fileStorageBytes(stats.totalSizeBytes) },
    { label: "Database Objects", value: stats.databaseObjects.toLocaleString() },
    { label: "Database File Size", value: fileStorageBytes(stats.databaseSizeBytes) },
    { label: "Tracked S3 Objects", value: stats.s3Objects.toLocaleString() },
    { label: "Tracked S3 File Size", value: fileStorageBytes(stats.s3SizeBytes) },
    { label: "Database Chunks", value: stats.databaseChunks.toLocaleString() },
    { label: "Chunk Payload Size", value: fileStorageBytes(stats.databaseChunkBytes) },
    { label: "Pending Multipart Uploads", value: stats.multipartUploads.toLocaleString() },
    { label: "Pending Parts", value: stats.multipartParts.toLocaleString() },
    { label: "Pending Part Size", value: fileStorageBytes(stats.multipartSizeBytes) },
  ];
});
const bucketColumns: TableColumn<FileStorageControlPlaneModels.BucketUsage>[] = [
  { accessorKey: "name", header: "Bucket" },
  { accessorKey: "backend", header: "Backend" },
  { accessorKey: "isPublic", header: "Access" },
  { accessorKey: "objectCount", header: "Objects" },
  { accessorKey: "sizeBytes", header: "Logical Size" },
  { accessorKey: "lastUpdatedAt", header: "Last Object Update" },
];
const typeColumns: TableColumn<FileStorageControlPlaneModels.ContentTypeUsage>[] = [
  { accessorKey: "contentType", header: "Content Type" },
  { accessorKey: "objectCount", header: "Objects" },
  { accessorKey: "sizeBytes", header: "Logical Size" },
];
onMounted(() => {
  if (auth.hasScope("storage:bucket.read")) return fileStorageControlPlane.load("statistics");
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
      title="File Storage Statistics"
      description="Live tracked object usage, bucket policies, and pending uploads. S3 totals include registered objects only, not untracked upstream files. Byte counts are logical payload sizes, not physical disk usage."
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template v-if="fileStorageControlPlane.stats.value">
        <dl class="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="item in summary" :key="item.label">
            <dt class="text-sm text-muted">{{ item.label }}</dt>
            <dd class="text-3xl font-semibold tabular-nums text-highlighted">{{ item.value }}</dd>
          </div>
        </dl>
        <USeparator label="Latest Activity" position="start" />
        <p class="text-sm text-muted">
          Last object update:
          {{
            fileStorageControlPlane.stats.value.lastUpdatedAt?.toLocaleString() ||
            "No objects uploaded"
          }}
        </p>
      </template>
      <UEmpty
        v-else
        :title="
          fileStorageControlPlane.busy.value
            ? 'Loading File Storage Statistics…'
            : 'Statistics Unavailable'
        "
        :loading="fileStorageControlPlane.busy.value"
        icon="i-lucide-hard-drive"
      />
      <template #footer
        ><UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="fileStorageControlPlane.busy.value"
          @click="fileStorageControlPlane.load('statistics')"
      /></template>
    </UCard>
    <template v-if="fileStorageControlPlane.stats.value">
      <UCard
        title="Bucket Usage"
        description="Tracked object counts and logical file sizes for each File Storage bucket."
      >
        <UTable
          :data="fileStorageControlPlane.stats.value.buckets || []"
          :columns="bucketColumns"
          empty="No File Storage buckets"
          caption="File Storage usage by bucket."
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
          <template #sizeBytes-cell="{ row }">{{
            fileStorageBytes(row.original.sizeBytes)
          }}</template>
          <template #lastUpdatedAt-cell="{ row }">{{
            row.original.lastUpdatedAt?.toLocaleString() || "No objects uploaded"
          }}</template>
        </UTable>
      </UCard>
      <UCard
        title="Content Type Usage"
        description="Tracked file counts and logical sizes grouped by stored content type."
      >
        <UTable
          :data="fileStorageControlPlane.stats.value.contentTypes || []"
          :columns="typeColumns"
          empty="No stored objects"
          caption="File Storage usage by content type."
        >
          <template #sizeBytes-cell="{ row }">{{
            fileStorageBytes(row.original.sizeBytes)
          }}</template>
        </UTable>
      </UCard>
    </template>
  </div>
</template>
