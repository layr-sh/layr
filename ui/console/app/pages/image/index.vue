<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const imageControlPlane = useImageControlPlane(client.value.image.cp);
onMounted(() => {
  if (auth.hasScope("image:stats.read")) return imageControlPlane.load("statistics");
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
    <UCard
      title="Image Statistics"
      description="A current snapshot of image traffic and process memory. Counters reset when the service restarts."
    >
      <dl v-if="imageControlPlane.stats.value" class="grid gap-6 sm:grid-cols-3">
        <div class="space-y-2">
          <dt class="text-sm text-muted">Total Requests</dt>
          <dd
            class="text-3xl font-semibold tracking-tight tabular-nums text-highlighted"
            data-testid="total-requests"
          >
            {{ imageControlPlane.stats.value.totalRequests.toLocaleString() }}
          </dd>
          <dd class="text-sm text-muted">Image requests received</dd>
        </div>
        <div class="space-y-2">
          <dt class="text-sm text-muted">Bytes Served</dt>
          <dd
            class="text-3xl font-semibold tracking-tight tabular-nums text-highlighted"
            :title="`${imageControlPlane.stats.value.bytesServed.toLocaleString()} bytes`"
            data-testid="bytes-served"
          >
            {{
              (imageControlPlane.stats.value.bytesServed / 1048576).toLocaleString(undefined, {
                maximumFractionDigits: 2,
              })
            }}
            <span class="text-sm font-normal text-muted">MiB</span>
          </dd>
          <dd class="text-sm text-muted">Transformed image data delivered</dd>
        </div>
        <div class="space-y-2">
          <dt class="text-sm text-muted">Process Memory</dt>
          <dd
            class="text-3xl font-semibold tracking-tight tabular-nums text-highlighted"
            :title="`${imageControlPlane.stats.value.memoryUsageBytes.toLocaleString()} bytes`"
            data-testid="process-memory"
          >
            {{
              (imageControlPlane.stats.value.memoryUsageBytes / 1048576).toLocaleString(undefined, {
                maximumFractionDigits: 2,
              })
            }}
            <span class="text-sm font-normal text-muted">MiB</span>
          </dd>
          <dd class="text-sm text-muted">Memory used by the service process</dd>
        </div>
      </dl>
      <UEmpty
        v-else
        :title="
          imageControlPlane.busy.value ? 'Loading Image Statistics…' : 'Statistics Unavailable'
        "
        :loading="imageControlPlane.busy.value"
        icon="i-lucide-chart-no-axes-combined"
      />
    </UCard>
    <UCard
      v-if="imageControlPlane.stats.value"
      title="Cache Performance"
      description="Cached responses avoid repeating image transformations. This snapshot does not include historical trends."
      :ui="{ body: 'space-y-6' }"
    >
      <div class="space-y-3">
        <dl class="flex items-baseline justify-between gap-4">
          <dt class="text-sm text-muted">Cache Hit Ratio</dt>
          <dd
            class="text-3xl font-semibold tabular-nums text-highlighted"
            data-testid="cache-hit-ratio"
          >
            {{ (imageControlPlane.stats.value.cacheHitRatio * 100).toFixed(2) }}%
          </dd>
        </dl>
        <UProgress
          :model-value="imageControlPlane.stats.value.cacheHitRatio * 100"
          :max="100"
          :get-value-label="(value) => `Cache Hit Ratio: ${value}%`"
        />
      </div>
      <dl class="grid gap-6 sm:grid-cols-2">
        <div class="space-y-2">
          <dt class="text-sm text-muted">Cache Hits</dt>
          <dd class="text-2xl font-semibold tabular-nums text-highlighted" data-testid="cache-hits">
            {{ imageControlPlane.stats.value.cacheHits.toLocaleString() }}
          </dd>
          <dd class="text-sm text-muted">Responses served from cache</dd>
        </div>
        <div class="space-y-2">
          <dt class="text-sm text-muted">Cache Misses</dt>
          <dd
            class="text-2xl font-semibold tabular-nums text-highlighted"
            data-testid="cache-misses"
          >
            {{ imageControlPlane.stats.value.cacheMisses.toLocaleString() }}
          </dd>
          <dd class="text-sm text-muted">Requests requiring image processing</dd>
        </div>
      </dl>
    </UCard>
  </div>
</template>
