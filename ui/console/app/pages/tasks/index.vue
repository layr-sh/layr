<script setup lang="ts">
const queues = [
  { name: "default", pending: 0, processing: 0, failed: 0, status: "Active" },
  { name: "notifications", pending: 0, processing: 0, failed: 0, status: "Active" },
];
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Tasks</h1>
        <p class="text-sm text-muted mt-1">Queues and scheduled background jobs.</p>
      </div>

      <div class="flex items-center gap-3">
        <UButton
          label="Dispatch Task"
          icon="i-lucide-play"
          color="neutral"
          variant="solid"
          size="sm"
        />
      </div>
    </div>

    <!-- Queues -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-clock" class="size-4 text-muted" />
          <span class="text-sm font-semibold text-highlighted">Active Job Queues</span>
        </div>
        <span class="text-xs text-muted font-mono">{{ queues.length }} queues</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="queue in queues"
          :key="queue.name"
          class="p-4 flex items-center justify-between hover:bg-accented/30 transition"
        >
          <div class="flex items-center gap-3">
            <div
              class="size-8 rounded bg-accented/50 border border-default flex items-center justify-center text-highlighted"
            >
              <UIcon name="i-lucide-layers" class="size-4" />
            </div>
            <div>
              <span class="font-mono text-sm font-semibold text-highlighted">{{ queue.name }}</span>
              <p class="text-xs text-dimmed mt-0.5">
                {{ queue.pending }} pending &bull; {{ queue.processing }} running &bull;
                {{ queue.failed }} failed
              </p>
            </div>
          </div>

          <UBadge color="success" variant="subtle" size="xs" class="text-[10px]">
            {{ queue.status }}
          </UBadge>
        </div>
      </div>
    </div>
  </div>
</template>
