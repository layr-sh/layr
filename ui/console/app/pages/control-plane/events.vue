<script setup lang="ts">
import type { TableColumn } from "@nuxt/ui";
import type { CoreControlPlaneModels } from "layr-client";

const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);
const columns: TableColumn<CoreControlPlaneModels.Event>[] = [
  { accessorKey: "type", header: "Event Type" },
  { accessorKey: "createdAt", header: "Created" },
  { id: "actor", header: "Actor" },
  { accessorKey: "resourceType", header: "Resource Type" },
  { accessorKey: "resourceId", header: "Resource ID" },
  { id: "actions" },
];
const details = computed(() => {
  if (!controlPlane.selectedEvent.value) return [];
  return [
    { label: "Event ID", value: String(controlPlane.selectedEvent.value.id) },
    { label: "Type", value: controlPlane.selectedEvent.value.type },
    { label: "Action", value: controlPlane.selectedEvent.value.action },
    { label: "Created", value: controlPlane.selectedEvent.value.createdAt.toLocaleString() },
    { label: "Actor Type", value: controlPlane.selectedEvent.value.actor.type },
    {
      label: "Actor ID",
      value: String(controlPlane.selectedEvent.value.actor.id ?? "Not Recorded"),
    },
    { label: "Actor Role", value: controlPlane.selectedEvent.value.actor.role || "Not Recorded" },
    { label: "Resource Type", value: controlPlane.selectedEvent.value.resourceType },
    { label: "Resource ID", value: controlPlane.selectedEvent.value.resourceId || "Not Recorded" },
    {
      label: "IP Address",
      value: controlPlane.selectedEvent.value.context.ipAddress || "Not Recorded",
    },
    {
      label: "Request ID",
      value: controlPlane.selectedEvent.value.context.requestId || "Not Recorded",
    },
    {
      label: "User Agent",
      value: controlPlane.selectedEvent.value.context.userAgent || "Not Recorded",
    },
  ];
});
const detailColumns = [
  { accessorKey: "label", header: "Field" },
  { accessorKey: "value", header: "Value" },
];
const payloadColumns = [
  { accessorKey: "name", header: "Field" },
  { accessorKey: "value", header: "Value" },
];
const eventData = computed(() =>
  Object.entries(controlPlane.selectedEvent.value?.data || {}).map((entry) => ({
    name: entry[0],
    value: JSON.stringify(entry[1]),
  })),
);
const eventMetadata = computed(() =>
  Object.entries(controlPlane.selectedEvent.value?.metadata || {}).map((entry) => ({
    name: entry[0],
    value: JSON.stringify(entry[1]),
  })),
);
const hasNextPage = computed(
  () =>
    !!controlPlane.events.value &&
    controlPlane.events.value.count >= controlPlane.eventLimit &&
    (controlPlane.events.value.total === undefined ||
      controlPlane.eventOffset.value + controlPlane.events.value.count <
        controlPlane.events.value.total),
);
onMounted(() => controlPlane.load("events"));
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
    <UCard
      title="Event Filters"
      description="Filter platform activity by event, actor, or resource."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="event-filters"
        :state="controlPlane.eventFilters.value"
        :disabled="controlPlane.busy.value"
        class="grid gap-4 sm:grid-cols-2"
        @submit="controlPlane.listEvents()"
      >
        <UFormField label="Event Type"
          ><UInput
            v-model="controlPlane.eventFilters.value.type"
            placeholder="Exact Event Type"
            class="w-full"
        /></UFormField>
        <UFormField label="Actor Type"
          ><USelect
            v-model="controlPlane.eventFilters.value.actorType"
            placeholder="All Actors"
            :items="[
              { label: 'User', value: 'user' },
              { label: 'Service Account', value: 'service_account' },
              { label: 'System', value: 'system' },
            ]"
            class="w-full"
        /></UFormField>
        <UFormField label="Actor ID"
          ><UInput v-model="controlPlane.eventFilters.value.actorId" class="w-full"
        /></UFormField>
        <UFormField label="Resource Type"
          ><UInput v-model="controlPlane.eventFilters.value.resourceType" class="w-full"
        /></UFormField>
        <UFormField label="Resource ID"
          ><UInput v-model="controlPlane.eventFilters.value.resourceId" class="w-full"
        /></UFormField>
      </UForm>
      <template #footer>
        <UButton
          label="Clear Filters"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="
            controlPlane.eventFilters.value = {
              type: '',
              actorType: '',
              actorId: '',
              resourceType: '',
              resourceId: '',
            };
            controlPlane.listEvents();
          "
        />
        <UButton
          type="submit"
          form="event-filters"
          label="Apply Filters"
          :disabled="controlPlane.busy.value"
        />
      </template>
    </UCard>
    <UCard title="Audit Events" description="Inspect platform activity by actor and resource.">
      <UTable
        :data="controlPlane.events.value?.data || []"
        :columns="columns"
        :loading="controlPlane.busy.value"
      >
        <template #createdAt-cell="cell">{{
          cell.row.original.createdAt.toLocaleString()
        }}</template>
        <template #actor-cell="cell">{{ cell.row.original.actor.type }}</template>
        <template #actions-cell="cell"
          ><UButton
            label="Details"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="controlPlane.selectEvent(String(cell.row.original.id))"
        /></template>
      </UTable>
      <template #footer>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <UBadge color="neutral" variant="subtle"
            >{{ controlPlane.eventOffset.value + (controlPlane.events.value?.count ? 1 : 0) }}–{{
              controlPlane.eventOffset.value + (controlPlane.events.value?.count || 0)
            }}<span v-if="controlPlane.events.value?.total !== undefined">
              of {{ controlPlane.events.value.total }}</span
            ></UBadge
          >
          <div class="flex gap-2">
            <UButton
              label="Previous"
              color="neutral"
              variant="outline"
              :disabled="controlPlane.busy.value || controlPlane.eventOffset.value === 0"
              @click="
                controlPlane.listEvents(
                  Math.max(0, controlPlane.eventOffset.value - controlPlane.eventLimit),
                )
              "
            />
            <UButton
              label="Next"
              color="neutral"
              variant="outline"
              :disabled="controlPlane.busy.value || !hasNextPage"
              @click="
                controlPlane.listEvents(controlPlane.eventOffset.value + controlPlane.eventLimit)
              "
            />
          </div>
        </div>
      </template>
    </UCard>
    <UCard
      v-if="controlPlane.selectedEvent.value"
      title="Event Details"
      description="Review the event's identity, context, and payload."
      :ui="{ body: 'space-y-5', footer: 'flex justify-end' }"
    >
      <UTable
        :data="details"
        :columns="detailColumns"
        :ui="{ td: 'whitespace-normal break-all' }"
      />
      <UAccordion
        :items="[
          { label: 'Event Data', slot: 'data' },
          { label: 'Metadata', slot: 'metadata' },
        ]"
      >
        <template #data
          ><UTable
            :data="eventData"
            :columns="payloadColumns"
            :ui="{ td: 'whitespace-normal break-all' }"
        /></template>
        <template #metadata
          ><UTable
            :data="eventMetadata"
            :columns="payloadColumns"
            :ui="{ td: 'whitespace-normal break-all' }"
        /></template>
      </UAccordion>
      <template #footer
        ><UButton
          label="Close"
          color="neutral"
          variant="outline"
          @click="controlPlane.selectedEvent.value = null"
      /></template>
    </UCard>
  </div>
</template>
