<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const dataWorkspace = useDataWorkspace();
const search = ref("");
type Argument = { name: string; data_type: string; has_default: boolean };
const mode = ref("fields");
const fields = ref<Record<string, string>>({});
const args = computed(() => selected.value?.input_arguments as Argument[] | undefined);
const supportsFields = computed(
  () => Array.isArray(args.value) && args.value.length > 0 && args.value.every((arg) => arg.name),
);
const overloadCount = computed(
  () =>
    functions.value.filter(
      (item) => item.schema === selected.value?.schema && item.name === selected.value?.name,
    ).length,
);
const invocationSource = ref("{}");
const invoked = ref("");
const selectedId = ref("");
const functions = ref<Record<string, unknown>[]>([]);
const response = ref<unknown>(undefined);
const invocation = reactive({ arguments: "{}" });
const confirmOpen = ref(false);
const options = computed(() =>
  functions.value
    .filter((databaseFunction) =>
      `${databaseFunction.schema}.${databaseFunction.name} ${databaseFunction.arguments}`
        .toLowerCase()
        .includes(search.value.toLowerCase()),
    )
    .map((databaseFunction) => ({
      label: `${databaseFunction.schema}.${databaseFunction.name}(${databaseFunction.arguments})`,
      value: String(databaseFunction.id),
    })),
);
const selected = computed(() =>
  functions.value.find((databaseFunction) => databaseFunction.id === selectedId.value),
);
async function load() {
  if (!auth.hasScope("data:schema.write")) return;
  await dataControlPlane.loadTables();
  if (auth.hasScope("data:config.read")) await dataControlPlane.loadConfig();
  const schemas = [
    ...new Set([
      ...(dataControlPlane.config.value?.schemas ?? ["public"]),
      ...dataControlPlane.tables.value.map((table) => table.schema),
    ]),
  ];
  const literals = schemas.map((schema) => `'${schema.replaceAll("'", "''")}'`).join(",");
  if (
    await dataControlPlane.executeSQL(
      `SELECT p.oid::text AS id, n.nspname AS schema, p.proname AS name, pg_get_function_arguments(p.oid) AS arguments, pg_get_function_result(p.oid) AS returns, p.provolatile::text AS volatility, p.prosecdef AS security_definer, CASE WHEN p.proargmodes IS NULL THEN COALESCE((SELECT jsonb_agg(jsonb_build_object('name', p.proargnames[a.position], 'data_type', format_type(a.type_oid, NULL), 'has_default', a.position > p.pronargs - p.pronargdefaults) ORDER BY a.position) FROM unnest(p.proargtypes::oid[]) WITH ORDINALITY a(type_oid, position)), '[]'::jsonb) END AS input_arguments FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE p.prokind = 'f' AND n.nspname IN (${literals || "'public'"}) ORDER BY n.nspname, p.proname, p.oid;`,
    )
  )
    functions.value = dataControlPlane.result.value?.rows ?? [];
}
function argumentPayload(): Record<string, unknown> {
  if (mode.value !== "fields" || !supportsFields.value) return JSON.parse(invocation.arguments);
  const payload: Record<string, unknown> = {};
  for (const argument of args.value!) {
    const value = fields.value[argument.name];
    if (value === undefined) {
      if (!argument.has_default) throw new Error(`Enter ${argument.name}.`);
      continue;
    }
    const type = argument.data_type;
    if (type === "boolean") payload[argument.name] = value === "true";
    else if (["json", "jsonb"].includes(type) || type.endsWith("[]"))
      payload[argument.name] = JSON.parse(value);
    else payload[argument.name] = value;
  }
  return payload;
}
function prepareInvocation() {
  invocationSource.value = JSON.stringify(argumentPayload(), null, 2);
  confirmOpen.value = true;
}
function validate(): FormError[] {
  try {
    const value = argumentPayload();
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error();
  } catch (err) {
    return [
      {
        name: "arguments",
        message:
          mode.value === "fields" && supportsFields.value && err instanceof Error
            ? err.message
            : "Enter a JSON object with named function arguments.",
      },
    ];
  }
  return [];
}
async function invoke() {
  confirmOpen.value = false;
  response.value = undefined;
  const databaseFunction = selected.value!;
  invoked.value = `${databaseFunction.schema}.${databaseFunction.name}(${databaseFunction.arguments})`;
  await dataControlPlane.run(async () => {
    response.value = await client.value.data.rpc.mutation(
      String(databaseFunction.schema),
      String(databaseFunction.name),
      JSON.parse(invocationSource.value),
    );
  });
}
watch(selectedId, () => {
  response.value = undefined;
  invocation.arguments = "{}";
  invocationSource.value = "{}";
  fields.value = {};
  mode.value = "fields";
});
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
    <UCard
      title="Database Functions"
      description="Browse PostgreSQL functions in exposed schemas and invoke them through REST RPC."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UFormField
        label="Function"
        description="Select a signature to view its arguments and return type."
      >
        <USelectMenu
          v-model="selectedId"
          v-model:search-term="search"
          :items="options"
          :loading="dataControlPlane.busy.value"
          :disabled="dataControlPlane.busy.value"
          ignore-filter
          :search-input="{ placeholder: 'Search by name or arguments…' }"
          value-key="value"
          placeholder="Choose a database function…"
          class="w-full"
        >
          <template #empty>No matching functions. Try another search.</template>
        </USelectMenu>
      </UFormField>
      <UEmpty
        v-if="!functions.length"
        :title="dataControlPlane.busy.value ? 'Loading Functions…' : 'No Functions Found'"
        :description="
          dataControlPlane.busy.value
            ? undefined
            : 'Create a PostgreSQL function using the SQL editor.'
        "
        :loading="dataControlPlane.busy.value"
        icon="i-lucide-square-function"
      />
      <template v-if="selected">
        <USeparator label="Function Details" position="start" />
        <p class="break-all font-mono text-sm">
          {{ selected.schema }}.{{ selected.name }}({{ selected.arguments }})
        </p>
        <UAlert
          v-if="overloadCount > 1"
          title="Overloaded Function"
          description="The RPC endpoint resolves the overload from the argument names and PostgreSQL types. Selecting a signature here does not force that overload."
          color="warning"
        />
        <dl class="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-muted">Returns</dt>
            <dd class="font-mono">{{ selected.returns }}</dd>
          </div>
          <div>
            <dt class="text-muted">Volatility</dt>
            <dd>
              {{
                selected.volatility === "i"
                  ? "Immutable"
                  : selected.volatility === "s"
                    ? "Stable"
                    : "Volatile"
              }}
            </dd>
          </div>
          <div>
            <dt class="text-muted">Security</dt>
            <dd>{{ selected.security_definer ? "Definer privileges" : "Invoker privileges" }}</dd>
          </div>
          <div>
            <dt class="text-muted">RPC Endpoint</dt>
            <dd class="break-all font-mono">
              /v1/data/{{ selected.schema }}/rpc/{{ selected.name }}
            </dd>
          </div>
        </dl>
        <UAlert
          v-if="selected.security_definer"
          title="Security Definer"
          description="This function runs with its owner's database privileges. Review its implementation before invoking it."
          color="warning"
        />
        <UForm
          id="data-rpc"
          :state="invocation"
          :validate="validate"
          :disabled="dataControlPlane.busy.value || !auth.hasScope('data:query.write')"
          class="space-y-5"
          @submit="prepareInvocation"
        >
          <UTabs
            v-if="supportsFields"
            v-model="mode"
            :items="[
              { label: 'Argument fields', value: 'fields' },
              { label: 'Raw JSON', value: 'json' },
            ]"
            :content="false"
          />
          <UFormField
            v-if="supportsFields && mode === 'fields'"
            name="arguments"
            label="Named Arguments"
          >
            <div class="grid gap-4 sm:grid-cols-2">
              <UFormField
                v-for="arg in args"
                :key="arg.name"
                :label="arg.name"
                :description="`${arg.data_type}${arg.has_default ? ' · optional, uses default when omitted' : ''}`"
                :required="!arg.has_default"
              >
                <USelect
                  v-if="arg.data_type === 'boolean'"
                  v-model="fields[arg.name]"
                  :items="['true', 'false']"
                  placeholder="Choose…"
                  class="w-full"
                />
                <UInput
                  v-else
                  v-model="fields[arg.name]"
                  :type="arg.data_type === 'date' ? 'date' : 'text'"
                  class="w-full"
                />
                <UButton
                  v-if="arg.has_default && fields[arg.name] !== undefined"
                  label="Use default"
                  size="xs"
                  variant="ghost"
                  color="neutral"
                  @click="delete fields[arg.name]"
                />
              </UFormField>
            </div>
          </UFormField>
          <UFormField
            v-else
            label="Named Arguments"
            name="arguments"
            description="Argument names must match the function signature. Use {} for functions without arguments."
            ><UTextarea
              v-model="invocation.arguments"
              :rows="6"
              spellcheck="false"
              class="w-full font-mono"
          /></UFormField>
        </UForm>
      </template>
      <template #footer>
        <UButton
          label="Refresh"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="load" />
        <UButton label="Manage in SQL Editor" to="/data/sql" color="neutral" variant="outline" />
        <UButton
          v-if="selected"
          label="Invoke Function"
          type="submit"
          form="data-rpc"
          :disabled="!auth.hasScope('data:query.write')"
          :loading="dataControlPlane.busy.value"
      /></template>
    </UCard>
    <UCard
      v-if="response !== undefined"
      title="Function Response"
      :description="`Console Service Account · ${invoked}`"
      :ui="{ footer: 'flex justify-end' }"
    >
      <pre class="max-h-120 overflow-auto whitespace-pre-wrap break-all text-sm">{{
        JSON.stringify(response, null, 2)
      }}</pre>
      <template #footer>
        <UButton
          label="Copy response"
          icon="i-lucide-copy"
          color="neutral"
          variant="outline"
          @click="dataWorkspace.copy(JSON.stringify(response, null, 2))"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="confirmOpen"
      title="Invoke Database Function?"
      :description="`Review the arguments for ${selected?.schema}.${selected?.name}.`"
      :ui="{ body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <UAlert
          title="Console Service Account"
          description="This POST RPC request uses data:query.write, not application-user row-level security. Functions can change data or have external side effects."
          color="warning"
        />
        <USeparator label="Arguments" position="start" />
        <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{
          invocationSource
        }}</pre>
        <pre class="overflow-auto whitespace-pre-wrap break-all text-sm">{{
          invocationSource
        }}</pre>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          @click="confirmOpen = false" /><UButton label="Invoke" color="warning" @click="invoke"
      /></template>
    </UModal>
  </div>
</template>
