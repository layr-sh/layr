<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { CoreControlPlaneModels } from "layr-client";

const auth = useAuth();
const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);
const hookForm = reactive({
  name: "",
  driver: "http",
  eventTypes: [] as string[],
  httpTargetUrl: "",
  sqlFunctionName: "",
  signingSecret: "",
  maxRetries: 3,
  timeoutSeconds: 30,
  isEnabled: true,
});
const confirmingDelete = ref(false);
const eventTypeOptions = computed(() =>
  [
    ...new Set([
      "*",
      "core.*",
      "console.*",
      "auth.*",
      "data.*",
      "filestorage.*",
      "function.*",
      "image.*",
      "tasks.*",
      "core.config.updated",
      "core.service_account.created",
      "core.service_account.updated",
      "core.service_account.deleted",
      "core.event_hook.created",
      "core.event_hook.updated",
      "core.event_hook.deleted",
      "core.event_hook.delivery_retried",
      "core.node.registered",
      "core.node.unregistered",
      "console.config.updated",
      "console.user.created",
      "console.user.updated",
      "console.user.deleted",
      "console.user.password_reset",
      "console.session.created",
      "console.session.revoked",
      "auth.config.updated",
      "auth.session.created",
      "auth.session.deleted",
      "auth.passkey.created",
      "auth.passkey.deleted",
      "auth.user.signed_up",
      "auth.user.converted",
      "auth.user.email_verified",
      "auth.user.phone_verified",
      "auth.user.updated",
      "auth.user.deleted",
      "auth.password.reset_requested",
      "auth.password.reset",
      "auth.password.changed",
      "auth.mfa.enabled",
      "auth.mfa.disabled",
      "auth.otp.sent",
      "auth.otp.verified",
      "auth.user.created",
      "auth.user.locked",
      "auth.user.unlocked",
      "auth.threat.password_breach_blocked",
      "auth.threat.bot_challenge_failed",
      "auth.user.suspicious_sign_in",
      "auth.user.sign_in_failed",
      "auth.mfa.challenge_failed",
      "auth.otp.verification_failed",
      "auth.threat.rate_limit_exceeded",
      "auth.user.exported",
      "auth.identity.linked",
      "auth.token.refreshed",
      "data.config.updated",
      "data.cache.flushed",
      "data.cache.invalidated",
      "data.table.created",
      "data.table.updated",
      "data.table.deleted",
      "data.row.created",
      "data.row.updated",
      "data.row.deleted",
      "data.table.truncated",
      "data.sql.executed",
      "data.function.executed",
      "data.kv.set",
      "data.kv.deleted",
      "data.kv.touched",
      "filestorage.config.updated",
      "filestorage.bucket.created",
      "filestorage.bucket.updated",
      "filestorage.bucket.deleted",
      "filestorage.object.uploaded",
      "filestorage.object.downloaded",
      "filestorage.object.deleted",
      "filestorage.object.upload_failed",
      "filestorage.multipart.initiated",
      "filestorage.multipart.completed",
      "filestorage.multipart.aborted",
      "filestorage.url.presigned",
      "function.config.updated",
      "function.endpoint.created",
      "function.endpoint.updated",
      "function.endpoint.deleted",
      "function.deployment.created",
      "function.deployment.rolled_back",
      "function.execution.completed",
      "function.execution.failed",
      "function.custom_domain.created",
      "function.custom_domain.deleted",
      "function.custom_domain.updated",
      "function.custom_domain_route.created",
      "function.custom_domain_route.deleted",
      "image.config.updated",
      "image.preset.created",
      "image.preset.updated",
      "image.preset.deleted",
      "image.transform.completed",
      "image.transform.failed",
      "image.inspect.completed",
      "image.cache.flushed",
      "image.cache.invalidated",
      "image.inspect.failed",
      "image.threat.ssrf_blocked",
      "image.url.signed",
      "tasks.job.created",
      "tasks.job.updated",
      "tasks.job.deleted",
      "tasks.job.paused",
      "tasks.job.resumed",
      "tasks.job.scheduled",
      "tasks.execution.enqueued",
      "tasks.execution.started",
      "tasks.execution.completed",
      "tasks.execution.failed",
      "tasks.execution.timed_out",
      "tasks.execution.retried",
      "tasks.execution.cancelled",
      "tasks.dlq.isolated",
      "tasks.dlq.retried",
      "tasks.dlq.purged",
      "tasks.threat.ssrf_blocked",
      "tasks.threat.signature_failed",
      "tasks.config.updated",
      ...hookForm.eventTypes,
    ]),
  ].sort(),
);
const hookColumns: TableColumn<CoreControlPlaneModels.EventHook>[] = [
  { accessorKey: "name", header: "Name" },
  { accessorKey: "driver", header: "Driver" },
  { accessorKey: "isEnabled", header: "Status" },
  { id: "actions" },
];
const deliveryColumns: TableColumn<CoreControlPlaneModels.EventHookDelivery>[] = [
  { accessorKey: "eventType", header: "Event" },
  { accessorKey: "isDelivered", header: "Status" },
  { accessorKey: "attemptCount", header: "Attempts" },
  { accessorKey: "httpResponseStatus", header: "HTTP Status" },
  { accessorKey: "errorMessage", header: "Error" },
  { accessorKey: "result", header: "Result" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actions" },
];
async function reload() {
  await controlPlane.load("event-hooks");
}
function clearHook() {
  controlPlane.selectedHook.value = null;
  Object.assign(hookForm, {
    name: "",
    driver: "http",
    eventTypes: [],
    httpTargetUrl: "",
    sqlFunctionName: "",
    signingSecret: "",
    maxRetries: 3,
    timeoutSeconds: 30,
    isEnabled: true,
  });
}
watch(controlPlane.selectedHook, () => {
  if (!controlPlane.selectedHook.value) return;
  Object.assign(hookForm, {
    name: controlPlane.selectedHook.value.name,
    driver: controlPlane.selectedHook.value.driver,
    eventTypes: [...(controlPlane.selectedHook.value.eventTypes || [])],
    httpTargetUrl: controlPlane.selectedHook.value.httpTargetUrl || "",
    sqlFunctionName: controlPlane.selectedHook.value.sqlFunctionName || "",
    signingSecret: "",
    maxRetries: controlPlane.selectedHook.value.maxRetries,
    timeoutSeconds: controlPlane.selectedHook.value.timeoutSeconds,
    isEnabled: controlPlane.selectedHook.value.isEnabled,
  });
});
async function editHook(hookId: string) {
  hookForm.signingSecret = "";
  await controlPlane.selectHook(hookId);
}
async function saveHook() {
  if (!hookForm.eventTypes.length) {
    controlPlane.error.value = "At least one event pattern is required. Use * to match all events.";
    return;
  }
  const input = {
    name: hookForm.name.trim(),
    driver: hookForm.driver,
    eventTypes: hookForm.eventTypes,
    isEnabled: hookForm.isEnabled,
    maxRetries: hookForm.maxRetries,
    timeoutSeconds: hookForm.timeoutSeconds,
    ...(hookForm.driver === "http"
      ? {
          httpTargetUrl: hookForm.httpTargetUrl,
          ...(hookForm.signingSecret ? { signingSecret: hookForm.signingSecret } : {}),
        }
      : { sqlFunctionName: hookForm.sqlFunctionName }),
  };
  if (
    await (controlPlane.selectedHook.value
      ? controlPlane.updateHook(String(controlPlane.selectedHook.value.id), input)
      : controlPlane.createHook(input))
  )
    clearHook();
}
async function deleteHook() {
  if (
    controlPlane.selectedHook.value &&
    (await controlPlane.deleteHook(String(controlPlane.selectedHook.value.id)))
  ) {
    confirmingDelete.value = false;
    clearHook();
  }
}
onMounted(reload);
</script>

<template>
  <div class="space-y-6" :aria-busy="controlPlane.busy.value">
    <UAlert
      v-if="controlPlane.error.value"
      color="error"
      title="Request Failed"
      :description="controlPlane.error.value"
      role="alert"
    />
    <UAlert
      v-if="controlPlane.notice.value"
      color="success"
      :title="controlPlane.notice.value"
      role="status"
    />
    <UCard
      title="Hook Filters"
      description="Filter event hooks by driver and status."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UForm
        id="event-hook-filters"
        :state="controlPlane.hookFilters.value"
        :disabled="controlPlane.busy.value"
        class="flex flex-wrap items-end gap-4"
        @submit="reload"
      >
        <UFormField label="Driver Filter"
          ><USelect
            v-model="controlPlane.hookFilters.value.driver"
            :items="[
              { label: 'All Drivers', value: 'all' },
              { label: 'HTTP', value: 'http' },
              { label: 'SQL', value: 'sql' },
            ]"
            class="w-40"
        /></UFormField>
        <UFormField label="Status Filter"
          ><USelect
            v-model="controlPlane.hookFilters.value.status"
            :items="[
              { label: 'All Statuses', value: 'all' },
              { label: 'Enabled', value: 'enabled' },
              { label: 'Disabled', value: 'disabled' },
            ]"
            class="w-40"
        /></UFormField>
      </UForm>
      <template #footer
        ><UButton
          type="submit"
          form="event-hook-filters"
          label="Apply Filters"
          :disabled="controlPlane.busy.value"
      /></template>
    </UCard>
    <UCard
      title="Event Hooks"
      description="Deliver platform events to HTTP endpoints or SQL functions."
    >
      <UTable
        :data="controlPlane.hooks.value"
        :columns="hookColumns"
        :loading="controlPlane.busy.value"
      >
        <template #isEnabled-cell="cell"
          ><UBadge :color="cell.row.original.isEnabled ? 'success' : 'neutral'" variant="subtle">{{
            cell.row.original.isEnabled ? "Enabled" : "Disabled"
          }}</UBadge></template
        >
        <template #actions-cell="cell"
          ><UButton
            label="Manage"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="editHook(String(cell.row.original.id))"
        /></template>
      </UTable>
    </UCard>
    <UCard
      v-if="controlPlane.selectedHook.value || auth.hasScope('core:event-hook.write')"
      :title="controlPlane.selectedHook.value ? 'Manage Event Hook' : 'Create Event Hook'"
      description="Configure event subscriptions and delivery."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap items-center justify-end gap-3' }"
    >
      <UForm
        id="event-hook-form"
        :state="hookForm"
        :disabled="controlPlane.busy.value || !auth.hasScope('core:event-hook.write')"
        class="space-y-5"
        @submit="saveHook"
      >
        <UFormField label="Name" required
          ><UInput v-model="hookForm.name" required class="w-full"
        /></UFormField>
        <UFormField label="Delivery Driver"
          ><USelect
            v-model="hookForm.driver"
            :items="[
              { label: 'HTTP Webhook', value: 'http' },
              { label: 'SQL Function', value: 'sql' },
            ]"
            class="w-full"
        /></UFormField>
        <UFormField
          label="Event Types"
          description="Select or enter event patterns. Use * to subscribe to all events."
          required
          ><USelectMenu
            v-model="hookForm.eventTypes"
            :items="eventTypeOptions"
            multiple
            create-item
            required
            class="w-full"
            placeholder="Select Event Types"
            @create="(eventType) => hookForm.eventTypes.push(eventType)"
        /></UFormField>
        <template v-if="hookForm.driver === 'http'">
          <UFormField label="Target URL" required
            ><UInput v-model="hookForm.httpTargetUrl" type="url" required class="w-full"
          /></UFormField>
          <UFormField
            label="Signing Secret"
            :description="
              controlPlane.selectedHook.value?.signingSecretConfigured
                ? 'A secret is configured. Leave empty to preserve it.'
                : 'Optional HMAC signing secret.'
            "
            ><UInput
              v-model="hookForm.signingSecret"
              type="password"
              autocomplete="new-password"
              class="w-full"
          /></UFormField>
        </template>
        <UFormField v-else label="SQL Function Name" required
          ><UInput
            v-model="hookForm.sqlFunctionName"
            required
            class="w-full"
            placeholder="public.handle_event"
        /></UFormField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="Maximum Retries"
            ><UInput
              v-model.number="hookForm.maxRetries"
              type="number"
              :min="0"
              :max="10"
              required
              class="w-full"
          /></UFormField>
          <UFormField label="Timeout (Seconds)"
            ><UInput
              v-model.number="hookForm.timeoutSeconds"
              type="number"
              :min="1"
              :max="60"
              required
              class="w-full"
          /></UFormField>
        </div>
        <USwitch v-model="hookForm.isEnabled" label="Hook Enabled" />
      </UForm>
      <template #footer>
        <UButton
          v-if="controlPlane.selectedHook.value && auth.hasScope('core:event-hook.write')"
          label="Delete Hook"
          color="error"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDelete = true"
        />
        <UButton
          v-if="controlPlane.selectedHook.value"
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="clearHook"
        />
        <UButton
          v-if="auth.hasScope('core:event-hook.write')"
          type="submit"
          form="event-hook-form"
          :label="controlPlane.selectedHook.value ? 'Save Hook' : 'Create Hook'"
          :disabled="controlPlane.busy.value"
        />
      </template>
    </UCard>
    <UCard
      v-if="controlPlane.selectedHook.value"
      title="Delivery History"
      description="Review delivery attempts and retry failed events."
    >
      <UTable
        :data="controlPlane.deliveries.value"
        :columns="deliveryColumns"
        :loading="controlPlane.busy.value"
      >
        <template #isDelivered-cell="cell"
          ><UBadge
            :color="cell.row.original.isDelivered ? 'success' : 'warning'"
            variant="subtle"
            >{{ cell.row.original.isDelivered ? "Delivered" : "Pending / Failed" }}</UBadge
          ></template
        >
        <template #createdAt-cell="cell">{{
          cell.row.original.createdAt.toLocaleString()
        }}</template>
        <template #actions-cell="cell"
          ><UButton
            v-if="!cell.row.original.isDelivered && auth.hasScope('core:event-hook.write')"
            label="Retry Delivery"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="
              controlPlane.retryDelivery(
                String(controlPlane.selectedHook.value.id),
                String(cell.row.original.id),
              )
            "
        /></template>
      </UTable>
    </UCard>
    <UModal
      v-model:open="confirmingDelete"
      title="Delete Event Hook?"
      description="This stops future deliveries and removes the hook. This cannot be undone."
      :dismissible="!controlPlane.busy.value"
    >
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDelete = false" /><UButton
          label="Delete Hook"
          color="error"
          :loading="controlPlane.busy.value"
          @click="deleteHook"
      /></template>
    </UModal>
  </div>
</template>
