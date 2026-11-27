<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import type { DataBaseModels } from "layr-client";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const route = useRoute();
const tab = ref(route.query.tab === "rest" ? "rest" : "graphql");
const dataWorkspace = useDataWorkspace();
const tableName = ref(typeof route.query.table === "string" ? route.query.table : "");
const options = computed(() =>
  dataControlPlane.tables.value.map((entry) => ({
    label: `${entry.schema}.${entry.name}`,
    value: `${entry.schema}.${entry.name}`,
  })),
);
const selected = computed(() =>
  dataControlPlane.tables.value.find(
    (entry) => `${entry.schema}.${entry.name}` === tableName.value,
  ),
);
const method = ref("GET");
const recordId = ref("");
const parameters = ref<{ name: string; value: string }[]>([{ name: "limit", value: "25" }]);
const restBody = ref("{}");
const restConfirm = ref(false);
const responseHeadersOpen = ref(false);
const restResult = ref<{
  status: number;
  label: string;
  duration: number;
  headers: [string, string][];
  body: string;
  request: string;
} | null>(null);
const restUrl = computed(() => {
  if (!selected.value) return "";
  const { schema, name } = selected.value;
  const url = new URL(
    `${client.value.endpoint}/v1/data/${encodeURIComponent(schema)}/${encodeURIComponent(name)}${method.value !== "POST" && recordId.value ? `/${encodeURIComponent(recordId.value)}` : ""}`,
  );
  for (const param of parameters.value)
    if (param.name.trim()) url.searchParams.append(param.name.trim(), param.value);
  return url.toString();
});
const restRequest = computed(() => `${method.value} ${restUrl.value}`);
const writes = computed(() => method.value !== "GET");
const hasBody = computed(() => ["POST", "PATCH"].includes(method.value));
const canSend = computed(
  () =>
    !!restUrl.value &&
    (writes.value ? auth.hasScope("data:query.write") : auth.hasScope("data:query.read")) &&
    (!["PATCH", "DELETE"].includes(method.value) || !!recordId.value),
);
const quoteShell = (value: string) => `'${value.replaceAll("'", "'\\''")}'`;
const curl = computed(
  () =>
    `curl --request ${method.value} ${quoteShell(restUrl.value)} --header 'Authorization: Bearer <YOUR_TOKEN>'${hasBody.value ? ` --header 'Content-Type: application/json' --data ${quoteShell(restBody.value)}` : ""}`,
);
async function executeRest() {
  if (!canSend.value) return;
  restConfirm.value = false;
  const requestLabel = restRequest.value;
  await dataControlPlane.run(async () => {
    if (hasBody.value) JSON.parse(restBody.value);
    const start = performance.now();
    const reply = await fetch(restUrl.value, {
      method: method.value,
      credentials: "include",
      headers: {
        ...client.value.headers,
        ...(hasBody.value ? { "Content-Type": "application/json" } : {}),
      },
      body: hasBody.value ? restBody.value : undefined,
    });
    const duration = performance.now() - start;
    restResult.value = {
      status: reply.status,
      label: reply.statusText,
      duration,
      headers: [...reply.headers],
      body: await reply.text(),
      request: requestLabel,
    };
  });
}
onMounted(async () => {
  if (auth.hasScope("data:schema.read")) await dataControlPlane.loadTables();
  const handoff = dataWorkspace.handoff.value;
  if (handoff?.table === tableName.value) {
    parameters.value = Object.entries(handoff.params).map(([name, value]) => ({ name, value }));
    dataWorkspace.handoff.value = null;
  }
});
const state = reactive({
  query: "query { __schema { queryType { name } } }",
  variables: "{}",
  operationName: "",
});
const response = ref<DataBaseModels.ExecuteGraphQLResponse | null>(null);
const submitted = ref("");
const graphqlInput = computed(() => JSON.stringify(state));
const confirmOpen = ref(false);
function validate(): FormError[] {
  const errors: FormError[] = [];
  if (!state.query.trim()) errors.push({ name: "query", message: "Enter a GraphQL document." });
  try {
    const variables = JSON.parse(state.variables);
    if (!variables || typeof variables !== "object" || Array.isArray(variables)) throw new Error();
  } catch {
    errors.push({ name: "variables", message: "Enter a JSON object for variables." });
  }
  return errors;
}
function request() {
  confirmOpen.value = true;
}
async function execute() {
  confirmOpen.value = false;
  response.value = null;
  submitted.value = graphqlInput.value;
  await dataControlPlane.run(async () => {
    response.value = await client.value.data.graphql({
      query: state.query,
      variables: JSON.parse(state.variables),
      operationName: state.operationName || undefined,
    });
  });
}
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
      title="API Explorer"
      description="Build and inspect REST and GraphQL requests against this database."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UAlert
        title="Console Service Account"
        description="REST and GraphQL requests run as the Service Account linked to your console session. Requests with the relevant query scope use the server’s database privileges and can bypass row-level security. This is not an application-user preview. Never expose a Service Account key in browser applications."
        color="warning"
        icon="i-lucide-shield-alert"
      />
      <UTabs
        v-model="tab"
        :items="[
          { label: 'GraphQL', value: 'graphql' },
          { label: 'REST', value: 'rest' },
        ]"
        :content="false"
      />
      <UForm
        v-if="tab === 'graphql'"
        id="data-graphql"
        :state="state"
        :validate="validate"
        :disabled="dataControlPlane.busy.value || !auth.hasScope('data:query.write')"
        class="space-y-5"
        @submit="request"
      >
        <UAlert
          v-if="!auth.hasScope('data:query.write')"
          title="Execution Access Required"
          description="Running GraphQL in this explorer requires data:query.write because documents can contain mutations."
          color="neutral"
        />
        <UFormField
          label="GraphQL Document"
          name="query"
          description="Only the selected operation runs. To use the server’s database privileges, queries need data:query.read and mutations need data:query.write."
          required
          ><UTextarea v-model="state.query" :rows="12" spellcheck="false" class="w-full font-mono"
        /></UFormField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Variables" name="variables"
            ><UTextarea
              v-model="state.variables"
              :rows="5"
              spellcheck="false"
              class="w-full font-mono"
          /></UFormField>
          <UFormField label="Operation Name" name="operationName"
            ><UInput
              v-model="state.operationName"
              placeholder="Optional named operation"
              class="w-full"
          /></UFormField>
        </div>
      </UForm>
      <UForm
        v-else
        id="data-rest"
        :state="{ method, recordId, parameters, restBody }"
        :disabled="dataControlPlane.busy.value"
        class="space-y-5"
        @submit="canSend && (writes ? (restConfirm = true) : executeRest())"
      >
        <div class="grid gap-4" :class="selected && method !== 'POST' && 'sm:grid-cols-2'">
          <UFormField label="Table" description="Choose a table exposed through REST.">
            <USelectMenu
              v-model="tableName"
              :items="options"
              value-key="value"
              placeholder="Choose a table…"
              :search-input="{ placeholder: 'Search by schema or table name…' }"
              class="w-full"
              @update:model-value="recordId = ''"
            />
          </UFormField>
          <UFormField
            v-if="selected && method !== 'POST'"
            label="Record Primary Key"
            :required="method !== 'GET'"
            :description="
              method === 'GET'
                ? 'Leave blank to list rows.'
                : 'Enter the row’s single-column primary key.'
            "
          >
            <UInput v-model="recordId" class="w-full" />
          </UFormField>
        </div>
        <UEmpty
          v-if="!selected"
          title="Choose a Table"
          description="Select a table to build a REST request. Browsing tables requires schema access."
          icon="i-lucide-table-2"
          :loading="dataControlPlane.busy.value"
        />
        <template v-else>
          <UFieldGroup class="w-full" role="group" aria-label="REST Request">
            <USelect
              v-model="method"
              :items="['GET', 'POST', 'PATCH', 'DELETE']"
              aria-label="Request Method"
            />
            <UInput
              :model-value="restUrl"
              readonly
              aria-label="Request URL"
              class="min-w-0 flex-1 font-mono"
            />
            <UButton
              label="Send Request"
              type="submit"
              :disabled="!canSend"
              :loading="dataControlPlane.busy.value"
            />
          </UFieldGroup>
          <section class="space-y-5">
            <USeparator label="Query Parameters" position="start" />
            <div v-for="(param, index) in parameters" :key="index" class="flex items-end gap-4">
              <UFormField label="Name" class="min-w-0 flex-1">
                <UInput
                  v-model="param.name"
                  :aria-label="`Parameter ${index + 1} name`"
                  placeholder="e.g. select"
                  class="w-full"
                />
              </UFormField>
              <UFormField label="Value" class="min-w-0 flex-1">
                <UInput
                  v-model="param.value"
                  :aria-label="`Parameter ${index + 1} value`"
                  placeholder="e.g. id,name"
                  class="w-full"
                />
              </UFormField>
              <UButton
                square
                icon="i-lucide-x"
                :aria-label="`Remove parameter ${index + 1}`"
                color="neutral"
                variant="ghost"
                @click="parameters.splice(index, 1)"
              />
            </div>
            <UButton
              label="Add Parameter"
              icon="i-lucide-plus"
              color="neutral"
              variant="outline"
              @click="parameters.push({ name: '', value: '' })"
            />
          </section>
          <UFormField v-if="hasBody" label="JSON Body"
            ><UTextarea v-model="restBody" :rows="5" spellcheck="false" class="w-full font-mono"
          /></UFormField>
        </template>
      </UForm>
      <template v-if="tab === 'graphql' || selected" #footer>
        <template v-if="tab === 'rest' && selected">
          <UButton
            label="Review Policies"
            icon="i-lucide-shield"
            :to="{ path: '/data', query: { table: tableName, tab: 'policies' } }"
            color="neutral"
            variant="outline"
          />
          <UButton
            label="Copy curl"
            icon="i-lucide-copy"
            color="neutral"
            variant="outline"
            @click="dataWorkspace.copy(curl)"
          />
        </template>
        <UButton
          v-if="tab === 'graphql'"
          label="Execute Document"
          type="submit"
          form="data-graphql"
          :disabled="!auth.hasScope('data:query.write')"
          :loading="dataControlPlane.busy.value"
        />
      </template>
    </UCard>
    <UCard
      v-if="tab === 'graphql' && response"
      title="GraphQL Response"
      description="Response to the last GraphQL request sent using your console Service Account."
      :ui="{ body: 'space-y-5' }"
    >
      <UAlert
        v-if="graphqlInput !== submitted"
        title="Document Changed"
        description="This response is from the last submitted document, operation name, and variables."
        color="neutral"
      />
      <UAlert
        v-if="response.errors?.length"
        title="GraphQL Errors"
        :description="response.errors.map((error) => error.message).join('\n')"
        color="error"
        role="alert"
      />
      <pre class="max-h-120 overflow-auto whitespace-pre-wrap break-all text-sm">{{
        JSON.stringify(response, null, 2)
      }}</pre>
    </UCard>
    <UCard
      v-if="tab === 'rest' && restResult"
      title="REST Response"
      description="Response to the last REST request sent using your console Service Account."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <div class="flex flex-wrap items-center gap-3">
        <UBadge :color="restResult.status < 400 ? 'success' : 'error'" variant="subtle">
          {{ restResult.status }} {{ restResult.label }}
        </UBadge>
        <span class="text-sm text-muted"
          >{{ Math.round(restResult.duration) }} ms to response headers</span
        >
      </div>
      <p class="break-all text-sm text-muted">{{ restResult.request }}</p>
      <pre
        v-if="restResult.body"
        class="max-h-96 overflow-auto whitespace-pre-wrap break-all text-sm"
        >{{ restResult.body }}</pre>
      <UEmpty v-else title="No Response Body" icon="i-lucide-file-text" />
      <template #footer>
        <UButton
          label="Response Headers"
          color="neutral"
          variant="outline"
          @click="responseHeadersOpen = true"
        />
        <UButton
          label="Copy body"
          icon="i-lucide-copy"
          color="neutral"
          variant="outline"
          @click="dataWorkspace.copy(restResult.body)"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="responseHeadersOpen"
      title="Response Headers"
      description="Only headers available to the browser are shown."
      :ui="{ content: 'max-w-3xl', footer: 'flex justify-end' }"
    >
      <template #body>
        <UTable
          :data="restResult?.headers.map(([name, value]) => ({ name, value }))"
          :columns="[
            { accessorKey: 'name', header: 'Name' },
            { accessorKey: 'value', header: 'Value' },
          ]"
          :ui="{ td: 'whitespace-normal break-all' }"
        >
          <template #empty><UEmpty title="No Headers Available" icon="i-lucide-list" /></template>
        </UTable>
      </template>
      <template #footer>
        <UButton
          label="Close"
          color="neutral"
          variant="outline"
          @click="responseHeadersOpen = false"
        />
      </template>
    </UModal>
    <UModal
      v-model:open="restConfirm"
      title="Send Write Request?"
      description="This request can insert, update, or delete rows using your console Service Account privileges."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body><p class="break-all font-mono text-sm">{{ restRequest }}</p></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          @click="restConfirm = false" /><UButton
          label="Send write request"
          color="warning"
          @click="executeRest"
      /></template>
    </UModal>
    <UModal
      v-model:open="confirmOpen"
      title="Execute GraphQL Document?"
      description="The selected GraphQL operation may modify data using your Service Account's privileges. Review it before continuing."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <template #body>
        <pre class="max-h-48 overflow-auto whitespace-pre-wrap text-sm font-mono">{{
          state.query
        }}</pre>
      </template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          @click="confirmOpen = false" /><UButton label="Execute" color="warning" @click="execute"
      /></template>
    </UModal>
  </div>
</template>
