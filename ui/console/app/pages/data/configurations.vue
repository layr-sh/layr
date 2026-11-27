<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import type { DataControlPlaneModels } from "layr-client";
import { validDataIdentifier } from "../../utils/data-forms";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const schemas = ref<string[]>([]);
const excludedTables = ref<string[]>([]);
const origins = ref<string[]>([]);
const headers = ref<string[]>([]);
const config = dataControlPlane.config;
const baseline = ref<DataControlPlaneModels.Config | null>(null);
const reloadOpen = ref(false);
const sections = {
  schemas: "Exposed schemas",
  rest: "REST API",
  graphql: "GraphQL",
  realtime: "Realtime",
  cors: "CORS",
  cache: "Query cache",
  rateLimiting: "Rate limiting",
} as const;
function draft() {
  if (!config.value) return null;
  return {
    ...config.value,
    schemas: [...new Set(schemas.value)],
    rest: { ...config.value.rest, excludedTables: [...new Set(excludedTables.value)] },
    cors: {
      ...config.value.cors,
      allowedOrigins: [...new Set(origins.value)],
      allowedHeaders: [...new Set(headers.value)],
    },
  };
}
const changes = computed(() => {
  const current = draft();
  if (!baseline.value || !current) return [];
  const saved = baseline.value;
  return (Object.keys(sections) as (keyof typeof sections)[]).filter(
    (key) => JSON.stringify(saved[key]) !== JSON.stringify(current[key]),
  );
});
function snapshot(saved: DataControlPlaneModels.Config) {
  schemas.value = [...(saved.schemas ?? [])];
  excludedTables.value = [...(saved.rest.excludedTables ?? [])];
  origins.value = [...(saved.cors.allowedOrigins ?? [])];
  headers.value = [...(saved.cors.allowedHeaders ?? [])];
  baseline.value = JSON.parse(JSON.stringify(draft()));
}
async function load() {
  if (!auth.hasScope("data:config.read")) return;
  reloadOpen.value = false;
  if (await dataControlPlane.loadConfig()) snapshot(config.value!);
}
function validate(): FormError[] {
  const errors: FormError[] = [];
  if (!schemas.value.length || schemas.value.some((value) => !validDataIdentifier(value)))
    errors.push({
      name: "schemas",
      message: "Enter at least one valid PostgreSQL schema name.",
    });
  if (
    excludedTables.value.some(
      (value) => !value.split(".").every(validDataIdentifier) || value.split(".").length > 2,
    )
  )
    errors.push({ name: "excludedTables", message: "Use table names or schema.table names." });
  if (config.value) {
    for (const section of ["rest", "graphql", "realtime", "cache"] as const) {
      for (const [key, value] of Object.entries(config.value[section])) {
        if (
          key !== "excludedTables" &&
          typeof value !== "boolean" &&
          (typeof value !== "number" ||
            !Number.isSafeInteger(value) ||
            value < (section === "cache" ? 0 : 1))
        )
          errors.push({
            name: `${section}.${key}`,
            message:
              section === "cache"
                ? "Enter a non-negative whole number."
                : "Enter a positive whole number.",
          });
      }
    }
    if (
      config.value.rateLimiting.enabled &&
      [config.value.rateLimiting.requestsPerMinute, config.value.rateLimiting.burst].some(
        (value) => !Number.isSafeInteger(value) || value < 1,
      )
    )
      errors.push({
        name: "rateLimiting",
        message: "Enabled rate limits must be positive whole numbers.",
      });
    if (config.value.rest.defaultLimit > config.value.rest.maxLimit)
      errors.push({
        name: "rest.defaultLimit",
        message: "Default limit cannot exceed the maximum limit.",
      });
  }
  return errors;
}
async function save() {
  const value = draft();
  if (value && (await dataControlPlane.saveConfig(value))) snapshot(config.value!);
}
onMounted(load);
</script>
<template>
  <div class="space-y-6" :aria-busy="dataControlPlane.busy.value">
    <UAlert
      v-if="dataControlPlane.error.value"
      title="Request Failed"
      :description="dataControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="dataControlPlane.notice.value"
      :title="dataControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UAlert
      v-if="changes.length"
      title="Unsaved Changes"
      :description="changes.map((key) => sections[key]).join(', ')"
      icon="i-lucide-pencil"
      color="warning"
    />
    <UCard
      title="Data Configuration"
      description="Control exposed schemas, API limits, realtime connections, and query caching."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        v-if="config"
        id="data-config"
        :state="config"
        :validate="validate"
        :disabled="dataControlPlane.busy.value || !auth.hasScope('data:config.write')"
        class="space-y-6"
        @submit="save"
      >
        <UFormField
          label="Exposed Schemas"
          name="schemas"
          description="API access is limited to the listed schemas. Protected schemas are not allowed; a listed schema has no tables until it exists."
          required
          ><UInputTags v-model="schemas" placeholder="Add schema name…" class="w-full"
        /></UFormField>
        <section class="space-y-5">
          <USeparator label="REST API" position="start" />
          <USwitch v-model="config.rest.enabled" label="Enable REST API" />
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField label="Default Row Limit" name="rest.defaultLimit"
              ><UInputNumber v-model="config.rest.defaultLimit" :min="1" class="w-full"
            /></UFormField>
            <UFormField label="Maximum Row Limit" name="rest.maxLimit"
              ><UInputNumber v-model="config.rest.maxLimit" :min="1" class="w-full"
            /></UFormField>
          </div>
          <UFormField
            label="Excluded Tables"
            name="excludedTables"
            description="These tables are unavailable through REST, GraphQL, and Realtime. Use a table name or schema.table."
            ><UInputTags
              v-model="excludedTables"
              placeholder="Add table or schema.table…"
              class="w-full"
          /></UFormField>
        </section>
        <section class="space-y-5">
          <USeparator label="GraphQL" position="start" />
          <div class="grid gap-4 sm:grid-cols-2">
            <USwitch v-model="config.graphql.enabled" label="Enable GraphQL" /><USwitch
              v-model="config.graphql.introspectionEnabled"
              label="Allow Introspection"
            />
          </div>
          <div class="grid gap-4 sm:grid-cols-3">
            <UFormField
              v-for="item in [
                { key: 'maxDepth', label: 'Maximum Query Depth' },
                { key: 'maxComplexity', label: 'Maximum Complexity' },
                { key: 'defaultLimit', label: 'GraphQL Default Limit' },
              ] as const"
              :key="item.key"
              :label="item.label"
              :name="`graphql.${item.key}`"
              ><UInputNumber v-model="config.graphql[item.key]" :min="1" class="w-full"
            /></UFormField>
          </div>
        </section>
        <section class="space-y-5">
          <USeparator label="Realtime" position="start" />
          <USwitch
            v-model="config.realtime.enabled"
            label="Enable Realtime"
            description="Disabling Realtime rejects new connections and stops event delivery without closing existing connections. Heartbeat and channel limits apply when a connection opens."
          />
          <div class="grid gap-4 sm:grid-cols-3">
            <UFormField
              v-for="item in [
                { key: 'heartbeatIntervalMs', label: 'Heartbeat Interval (ms)' },
                { key: 'maxChannelsPerConnection', label: 'Channels per Connection' },
                { key: 'maxConnections', label: 'Maximum Connections per Node' },
              ] as const"
              :key="item.key"
              :label="item.label"
              :name="`realtime.${item.key}`"
              ><UInputNumber v-model="config.realtime[item.key]" :min="1" class="w-full"
            /></UFormField>
          </div>
        </section>
        <section class="space-y-5">
          <USeparator label="Query Cache" position="start" />
          <USwitch v-model="config!.cache.enabled" label="Enable Query Cache" />
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField
              label="Query TTL (seconds)"
              name="cache.queryTtlSeconds"
              description="Default lifetime for cached query results."
              ><UInputNumber v-model="config!.cache.queryTtlSeconds" :min="0" class="w-full"
            /></UFormField>
            <UFormField
              label="Maximum Cached Queries"
              name="cache.maxCachedQueries"
              description="Limits new cache writes using a shared counter. Zero disables the limit."
              ><UInputNumber v-model="config!.cache.maxCachedQueries" :min="0" class="w-full"
            /></UFormField>
          </div>
          <UFormField
            label="Catalog TTL (seconds)"
            name="cache.catalogTtlSeconds"
            description="Applies when the configuration is saved and the catalog is refreshed. Zero uses a one-hour fallback."
            ><UInputNumber v-model="config!.cache.catalogTtlSeconds" :min="0" class="w-full"
          /></UFormField>
          <USwitch v-model="config!.cache.invalidateOnMutation" label="Invalidate on Mutation" />
          <USwitch v-model="config!.cache.invalidateOnCdc" label="Invalidate on CDC" />
        </section>
        <section class="space-y-5">
          <USeparator label="Cross-Origin Requests" position="start" />
          <UFormField
            label="Allowed Origins"
            name="origins"
            description="Applies to Data requests and WebSocket handshakes that send an Origin header. Use * to allow any origin without HTTP CORS credentials, or list exact origins to allow credentials."
            ><UInputTags
              v-model="origins"
              placeholder="Add https://app.example.com…"
              class="w-full"
          /></UFormField>
          <UFormField
            label="Allowed Request Headers"
            name="headers"
            description="HTTP CORS preflights only allow these request headers. Include Authorization and Publishable-Key for browser clients."
            ><UInputTags v-model="headers" placeholder="Add request header…" class="w-full" />
          </UFormField>
        </section>
        <section class="space-y-5">
          <USeparator label="Rate Limiting" position="start" />
          <USwitch
            v-model="config!.rateLimiting.enabled"
            label="Enable Rate Limiting"
            description="Shared across nodes, per authenticated caller or anonymous client IP. Public Data API requests count, including calls from API Explorer. Control-plane requests do not count."
          />
          <UFormField name="rateLimiting" label="Request Limits">
            <div class="grid gap-4 sm:grid-cols-2">
              <UFormField label="Requests per Minute" name="rateLimiting.requestsPerMinute">
                <UInputNumber
                  v-model="config!.rateLimiting.requestsPerMinute"
                  :min="1"
                  class="w-full"
                />
              </UFormField>
              <UFormField label="Burst per Second" name="rateLimiting.burst">
                <UInputNumber v-model="config!.rateLimiting.burst" :min="1" class="w-full" />
              </UFormField>
            </div>
          </UFormField>
        </section>
      </UForm>
      <UEmpty
        v-else
        :title="
          dataControlPlane.busy.value ? 'Loading Data Configuration…' : 'Configuration Unavailable'
        "
        :description="dataControlPlane.busy.value ? undefined : 'Retry loading the configuration.'"
        :loading="dataControlPlane.busy.value"
        icon="i-lucide-settings"
      />
      <template #footer
        ><UButton
          label="Reload"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="changes.length ? (reloadOpen = true) : load()" /><UButton
          v-if="auth.hasScope('data:config.write')"
          label="Save Data Config"
          type="submit"
          form="data-config"
          :loading="dataControlPlane.busy.value"
          :disabled="!config"
      /></template>
    </UCard>
    <UModal
      v-model:open="reloadOpen"
      title="Discard Unsaved Configuration?"
      description="Reloading replaces this draft with the saved server configuration."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #footer
        ><UButton
          label="Keep Editing"
          color="neutral"
          variant="outline"
          @click="reloadOpen = false" /><UButton
          label="Discard Changes"
          color="warning"
          @click="load"
      /></template>
    </UModal>
  </div>
</template>
