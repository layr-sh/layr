<script setup lang="ts">
const functions = [
  { name: "health-check", trigger: "HTTP GET", runtime: "Wasm / Go", status: "Active" },
  { name: "event-dispatcher", trigger: "core.event", runtime: "Wasm / Go", status: "Active" },
];
</script>

<template>
  <div class="space-y-8">
    <!-- Header -->
    <div
      class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-6 border-b border-default"
    >
      <div>
        <h1 class="text-2xl font-bold tracking-tight text-highlighted">Functions</h1>
        <p class="text-sm text-muted mt-1">Serverless functions and HTTP endpoints.</p>
      </div>

      <div class="flex items-center gap-3">
        <UButton
          label="New Function"
          icon="i-lucide-plus"
          color="neutral"
          variant="solid"
          size="sm"
        />
      </div>
    </div>

    <!-- Functions List -->
    <div class="rounded-lg border border-default bg-muted/20 overflow-hidden">
      <div class="p-4 border-b border-default flex items-center justify-between">
        <div class="flex items-center gap-2">
          <UIcon name="i-lucide-square-terminal" class="size-4 text-muted" />
          <span class="text-sm font-semibold text-highlighted">Deployed Functions</span>
        </div>
        <span class="text-xs text-muted font-mono">{{ functions.length }} deployed</span>
      </div>

      <div class="divide-y divide-default">
        <div
          v-for="functionItem in functions"
          :key="functionItem.name"
          class="p-4 flex items-center justify-between hover:bg-accented/30 transition"
        >
          <div class="flex items-center gap-3">
            <div
              class="size-8 rounded bg-accented/50 border border-default flex items-center justify-center text-highlighted"
            >
              <UIcon name="i-lucide-code-xml" class="size-4" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <span class="font-mono text-sm font-semibold text-highlighted">{{
                  functionItem.name
                }}</span>
                <UBadge color="neutral" variant="outline" size="xs" class="text-[10px]">
                  {{ functionItem.runtime }}
                </UBadge>
              </div>
              <p class="text-xs text-dimmed mt-0.5">Trigger: {{ functionItem.trigger }}</p>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <UBadge color="success" variant="subtle" size="xs" class="text-[10px]">
              {{ functionItem.status }}
            </UBadge>
            <UButton
              label="Logs"
              icon="i-lucide-file-text"
              color="neutral"
              variant="outline"
              size="xs"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
