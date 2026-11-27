<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const route = useRoute();
const selected = ref(typeof route.query.table === "string" ? route.query.table : "");
const scope = ref("table");
const schema = ref("");
const schemaOptions = computed(() => [
  ...new Set(dataControlPlane.tables.value.map((entry) => entry.schema)),
]);
const flushOpen = ref(false);
const schemaOpen = ref(false);
const confirmation = ref("");
const options = computed(() =>
  dataControlPlane.tables.value.map((table) => ({
    label: `${table.schema}.${table.name}`,
    value: `${table.schema}.${table.name}`,
  })),
);
async function load() {
  if (auth.hasScope("data:schema.read")) await dataControlPlane.loadTables();
}
async function invalidate() {
  const table = dataControlPlane.tables.value.find(
    (table) => `${table.schema}.${table.name}` === selected.value,
  );
  if (scope.value === "schema") {
    if (
      schemaOptions.value.includes(schema.value) &&
      (await dataControlPlane.invalidateCache({ schema: schema.value }))
    )
      schemaOpen.value = false;
    return;
  }
  if (!table) return;
  await dataControlPlane.invalidateCache({ schema: table.schema, table: table.name });
}
async function flush() {
  if (await dataControlPlane.flushCache()) flushOpen.value = false;
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
    <UCard
      title="Query Cache Invalidation"
      description="Invalidate query results for a table or schema. Cached results are rebuilt as needed."
      :ui="{ body: 'space-y-5', footer: 'flex justify-end' }"
    >
      <UFormField v-if="auth.hasScope('data:schema.read')" label="Scope">
        <USelect
          v-model="scope"
          :items="[
            { label: 'Table', value: 'table' },
            { label: 'Schema', value: 'schema' },
          ]"
          :disabled="dataControlPlane.busy.value"
          class="w-full"
        />
      </UFormField>
      <UFormField v-if="auth.hasScope('data:schema.read') && scope === 'table'" label="Table"
        ><USelectMenu
          v-model="selected"
          :items="options"
          value-key="value"
          placeholder="Choose a table…"
          :search-input="{ placeholder: 'Search by schema or table name…' }"
          :disabled="dataControlPlane.busy.value"
          class="w-full"
      /></UFormField>
      <div v-else-if="auth.hasScope('data:schema.read')" class="space-y-5">
        <UFormField label="Schema"
          ><USelectMenu
            v-model="schema"
            :items="schemaOptions"
            placeholder="Choose a schema…"
            :disabled="dataControlPlane.busy.value"
            class="w-full"
        /></UFormField>
        <UAlert
          title="Global Query Cache Impact"
          description="Schema invalidation targets the schema catalog and the global query cache, not just the selected schema. It does not delete database rows."
          icon="i-lucide-triangle-alert"
          color="warning"
        />
      </div>
      <UAlert
        v-else
        title="Schema Access Required"
        description="Table selection requires data:schema.read. You can still flush the global cache below."
        color="neutral"
      />
      <template #footer
        ><UButton
          :label="scope === 'schema' ? 'Invalidate Schema Cache' : 'Invalidate Table Cache'"
          :disabled="
            (scope === 'schema' ? !schema : !selected) ||
            dataControlPlane.busy.value ||
            !auth.hasScope('data:cache.write')
          "
          @click="scope === 'schema' ? (schemaOpen = true) : invalidate()"
      /></template>
    </UCard>
    <UCard
      title="Global Cache"
      description="Invalidate the cached schema catalog and all query results. Database load may increase while caches rebuild."
      :ui="{ footer: 'flex justify-end' }"
    >
      <template #footer
        ><UButton
          label="Flush All Caches"
          color="warning"
          :disabled="dataControlPlane.busy.value || !auth.hasScope('data:cache.write')"
          @click="
            confirmation = '';
            flushOpen = true;
          "
      /></template>
    </UCard>
    <UModal
      v-model:open="schemaOpen"
      title="Invalidate Schema Cache?"
      :description="`Invalidating ${schema} targets the schema catalog and query results across all schemas.`"
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="schemaOpen = false" /><UButton
          label="Confirm Invalidation"
          color="warning"
          :loading="dataControlPlane.busy.value"
          @click="invalidate"
      /></template>
    </UModal>
    <UModal
      v-model:open="flushOpen"
      title="Flush All Data Caches?"
      description="This requests invalidation of all cached query results and the schema catalog. It does not delete database rows or application key–value entries."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
    >
      <template #body
        ><UFormField label="Type FLUSH to confirm"
          ><UInput v-model="confirmation" class="w-full" autocomplete="off" /></UFormField
      ></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="flushOpen = false" /><UButton
          label="Confirm Flush"
          color="warning"
          :loading="dataControlPlane.busy.value"
          :disabled="confirmation !== 'FLUSH'"
          @click="flush"
      /></template>
    </UModal>
  </div>
</template>
