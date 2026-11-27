<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const route = useRoute();
const dataWorkspace = useDataWorkspace();
const state = reactive({
  table: typeof route.query.table === "string" ? route.query.table : "",
  event: "*",
  filter: "",
});
const skipped = ref(0);
const interrupted = ref(false);
const changes = computed(() => {
  const before = selected.value?.old_record;
  const after = selected.value?.record;
  if (!before || !after || typeof before !== "object" || typeof after !== "object") return [];
  return [...new Set([...Object.keys(before), ...Object.keys(after)])]
    .map((name) => ({
      name,
      before: JSON.stringify((before as Record<string, unknown>)[name]),
      after: JSON.stringify((after as Record<string, unknown>)[name]),
    }))
    .filter((item) => item.before !== item.after);
});
const options = computed(() =>
  dataControlPlane.tables.value.map((table) => ({
    label: `${table.schema}.${table.name}`,
    value: `${table.schema}.${table.name}`,
  })),
);
const status = ref("Disconnected");
const error = ref("");
const paused = ref(false);
const messages = ref<
  {
    id: number;
    received: string;
    event: string;
    schema: string;
    table: string;
    payload: Record<string, unknown>;
  }[]
>([]);
const selected = ref<Record<string, unknown> | null>(null);
let sequence = 0;
let webSocket: WebSocket | null = null;
let heartbeat: ReturnType<typeof setInterval> | undefined;
let connectionTimeout: ReturnType<typeof setTimeout> | undefined;
const connected = computed(() => status.value === "Connecting" || status.value === "Listening");
function validate(): FormError[] {
  return dataControlPlane.tables.value.some(
    (table) => `${table.schema}.${table.name}` === state.table,
  )
    ? []
    : [{ name: "table", message: "Choose a table to subscribe to." }];
}
function disconnect() {
  if (connectionTimeout) clearTimeout(connectionTimeout);
  connectionTimeout = undefined;
  if (heartbeat) clearInterval(heartbeat);
  heartbeat = undefined;
  if (webSocket) {
    webSocket.onopen = null;
    webSocket.onmessage = null;
    webSocket.onerror = null;
    webSocket.onclose = null;
    webSocket.close();
    webSocket = null;
  }
  status.value = "Disconnected";
}
function connect() {
  if (validate().length || !auth.hasScope("data:query.read")) return;
  disconnect();
  error.value = "";
  status.value = "Connecting";
  const table = dataControlPlane.tables.value.find(
    (table) => `${table.schema}.${table.name}` === state.table,
  )!;
  const url = new URL(`${client.value.endpoint}/v1/realtime`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  if (client.value.sessionToken)
    url.searchParams.set("token", client.value.sessionToken.replace(/^Bearer\s+/i, ""));
  try {
    webSocket = new WebSocket(url.toString());
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Could not connect to realtime.";
    status.value = "Disconnected";
    return;
  }
  const connection = webSocket;
  connectionTimeout = setTimeout(() => {
    error.value = "The subscription was not acknowledged. Check the connection and try again.";
    disconnect();
  }, 10000);
  connection.onopen = () => {
    connection.send(
      JSON.stringify({
        action: "subscribe",
        channel: "console-data-inspector",
        schema: table.schema,
        table: table.name,
        event: state.event,
        filter: state.filter,
      }),
    );
    heartbeat = setInterval(() => {
      if (connection.readyState === WebSocket.OPEN)
        connection.send(JSON.stringify({ action: "ping" }));
    }, 15000);
  };
  connection.onmessage = (event) => {
    try {
      const payload = JSON.parse(String(event.data)) as Record<string, unknown>;
      if (payload.status === "error" || payload.error) {
        error.value = String(payload.error);
        disconnect();
        return;
      }
      if (payload.status === "subscribed" || payload.type === "subscribed") {
        if (connectionTimeout) clearTimeout(connectionTimeout);
        connectionTimeout = undefined;
        status.value = "Listening";
        return;
      }
      if (!payload.event || status.value !== "Listening") return;
      if (paused.value) {
        skipped.value++;
        return;
      }
      messages.value = [
        {
          id: ++sequence,
          received: new Date().toLocaleTimeString(),
          event: String(payload.event),
          schema: String(payload.schema ?? table.schema),
          table: String(payload.table ?? table.name),
          payload,
        },
        ...messages.value,
      ].slice(0, 100);
    } catch {
      error.value = "Received an invalid realtime message.";
    }
  };
  connection.onerror = () => {
    interrupted.value = status.value === "Listening" || interrupted.value;
    error.value =
      "Realtime connection failed. Check realtime configuration, allowed origins, and your session, then reconnect.";
    disconnect();
  };
  connection.onclose = (event) => {
    interrupted.value = status.value === "Listening" || interrupted.value;
    error.value = `Realtime connection closed (code ${event.code}${event.reason ? `: ${event.reason}` : ""}). Reconnect to resume receiving events.`;
    disconnect();
  };
}
onMounted(() => {
  if (auth.hasScope("data:schema.read")) return dataControlPlane.loadTables();
});
function setEventOpen(open: boolean) {
  if (!open) selected.value = null;
}
onBeforeUnmount(disconnect);
</script>
<template>
  <div class="space-y-6" :aria-busy="dataControlPlane.busy.value">
    <UAlert
      v-if="error || dataControlPlane.error.value"
      title="Realtime Error"
      :description="error || dataControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UCard
      title="Realtime Inspector"
      description="Subscribe to table changes with the console Service Account. This is not an application-user RLS preview."
      :ui="{ footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UForm
        id="data-realtime"
        :state="state"
        :validate="validate"
        :disabled="connected || dataControlPlane.busy.value"
        class="space-y-5"
        @submit="connect"
      >
        <UFormField
          label="Table"
          name="table"
          description="Choose the table to subscribe to."
          required
          ><USelectMenu
            v-model="state.table"
            :items="options"
            value-key="value"
            placeholder="Choose a table…"
            :search-input="{ placeholder: 'Search by schema or table name…' }"
            class="w-full"
        /></UFormField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Events" name="event"
            ><USelect
              v-model="state.event"
              :items="[
                { label: 'All changes', value: '*' },
                { label: 'Inserts', value: 'INSERT' },
                { label: 'Updates', value: 'UPDATE' },
                { label: 'Deletes', value: 'DELETE' },
              ]"
              class="w-full"
          /></UFormField>
          <UFormField label="Row Filter" name="filter"
            ><UInput v-model="state.filter" placeholder="Optional column=eq.value" class="w-full"
          /></UFormField>
        </div>
      </UForm>
      <template #footer
        ><UBadge
          :color="status === 'Listening' ? 'success' : 'neutral'"
          variant="subtle"
          role="status"
          class="mr-auto"
          >{{ status }}</UBadge
        ><UButton
          v-if="connected"
          label="Disconnect"
          color="neutral"
          variant="outline"
          @click="disconnect" /><UButton
          v-else
          label="Connect"
          type="submit"
          form="data-realtime"
          :disabled="
            !state.table || !auth.hasScope('data:query.read') || !auth.hasScope('data:schema.read')
          "
      /></template>
    </UCard>
    <UCard
      title="Change Events"
      description="Shows the newest 100 displayed events. Events skipped while disconnected or paused are not replayed."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <div v-if="paused || skipped || interrupted" class="flex flex-wrap items-center gap-3">
        <UBadge v-if="paused" color="warning" variant="subtle">Display Paused</UBadge>
        <UBadge v-if="skipped" color="warning" variant="subtle"
          >{{ skipped }} events skipped while paused</UBadge
        >
        <UBadge v-if="interrupted" color="warning" variant="subtle"
          >Stream interrupted · events may be missing</UBadge
        >
      </div>
      <UTable
        :data="messages"
        :columns="[
          { accessorKey: 'received', header: 'Received' },
          { accessorKey: 'event', header: 'Event' },
          { accessorKey: 'schema', header: 'Schema' },
          { accessorKey: 'table', header: 'Table' },
          { id: 'actions' },
        ]"
        caption="Realtime table changes. Select an event to view its payload."
        sticky
        class="max-h-120"
        @select="(_event, row) => (selected = row.original.payload)"
        ><template #actions-cell="{ row }">
          <div class="flex justify-end">
            <UDropdownMenu
              :items="[
                {
                  label: 'Inspect',
                  icon: 'i-lucide-info',
                  onSelect: () => (selected = row.original.payload),
                },
              ]"
              :content="{ align: 'end' }"
            >
              <UButton
                square
                icon="i-lucide-ellipsis"
                color="neutral"
                variant="ghost"
                aria-label="Event actions"
              />
            </UDropdownMenu>
          </div> </template
        ><template #empty>
          <UEmpty
            :title="paused ? 'Display Paused' : 'No Change Events Yet'"
            description="Connect to a table and wait for matching changes. The stream does not replay past events."
            icon="i-lucide-radio"
          /> </template
      ></UTable>
      <template #footer
        ><UButton
          :label="paused ? 'Resume Display' : 'Pause Display'"
          color="neutral"
          variant="outline"
          @click="paused = !paused" /><UButton
          label="Clear Events"
          color="neutral"
          variant="outline"
          :disabled="!messages.length"
          @click="
            messages = [];
            skipped = 0;
            interrupted = false;
          "
      /></template>
    </UCard>
    <UModal
      :open="!!selected"
      title="Change Event"
      description="Inspect the payload received for this event."
      :ui="{ content: 'max-w-4xl', body: 'space-y-6', footer: 'flex flex-wrap justify-end gap-3' }"
      @update:open="setEventOpen"
    >
      <template #body>
        <UAlert
          v-if="selected?.event === 'UPDATE' && !selected.old_record"
          title="Previous Values Unavailable"
          description="The server did not send a previous row image for this event."
          color="neutral"
        />
        <UAlert
          v-if="selected?.truncated"
          title="Partial row payload"
          description="This event exceeded the stream payload limit and does not contain the complete row."
          color="warning"
        />
        <UTable
          v-if="changes.length"
          :data="changes"
          :columns="[
            { accessorKey: 'name', header: 'Column' },
            { accessorKey: 'before', header: 'Before' },
            { accessorKey: 'after', header: 'After' },
          ]"
        />
        <USeparator label="Event Payload" position="start" />
        <pre class="max-h-120 overflow-auto whitespace-pre-wrap break-all text-sm">{{
          JSON.stringify(selected, null, 2)
        }}</pre>
      </template>
      <template #footer
        ><UButton
          label="Close"
          color="neutral"
          variant="outline"
          @click="selected = null" /><UButton
          label="Copy Event JSON"
          icon="i-lucide-copy"
          color="neutral"
          variant="outline"
          @click="dataWorkspace.copy(JSON.stringify(selected, null, 2))"
      /></template>
    </UModal>
  </div>
</template>
