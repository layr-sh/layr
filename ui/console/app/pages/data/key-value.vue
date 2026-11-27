<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import { LayrAPIError, type DataBaseModels } from "layr-client";
const auth = useAuth();
const client = useClient();
const dataControlPlane = useDataControlPlane(client.value.data.cp);
const state = reactive({ key: "", value: "", ttl: 0, step: 1 });
const entry = ref<DataBaseModels.GetKVResponse | null>(null);
const recentKeys = ref<string[]>([]);
const deleteOpen = ref(false);
const confirmation = ref("");
const pendingKey = ref("");
const dataWorkspace = useDataWorkspace();
const missing = ref(false);
const expiration = ref<string | null>(null);
const valueFormat = ref("text");
const preview = computed(() => {
  if (valueFormat.value !== "json") return "";
  try {
    return JSON.stringify(JSON.parse(state.value), null, 2);
  } catch {
    return null;
  }
});
function expiryLabel(ttl: number) {
  return ttl > 0
    ? `Expiration set for ${ttl} seconds`
    : "No expiration requested; the configured store decides the lifetime.";
}
function validate(): FormError[] {
  const errors: FormError[] = [];
  if (
    !state.key.trim() ||
    new TextEncoder().encode(state.key).length > 512 ||
    ["mget", "mset", "increment"].includes(state.key)
  )
    errors.push({ name: "key", message: "Enter a non-reserved key of up to 512 bytes." });
  if (!Number.isSafeInteger(state.ttl) || state.ttl < 0)
    errors.push({ name: "ttl", message: "Enter a non-negative whole number of seconds." });
  if (!Number.isSafeInteger(state.step))
    errors.push({ name: "step", message: "Enter a whole-number increment or decrement." });
  return errors;
}
function remember(key: string) {
  recentKeys.value = [key, ...recentKeys.value.filter((item) => item !== key)].slice(0, 20);
}
async function lookup(key = state.key) {
  state.key = key;
  entry.value = null;
  missing.value = false;
  expiration.value = null;
  await dataControlPlane.run(async () => {
    try {
      entry.value = await client.value.data.kv.get(key);
      state.value = entry.value.value;
      remember(key);
    } catch (err) {
      if (!(err instanceof LayrAPIError) || err.status !== 404) throw err;
      missing.value = true;
    }
  });
}
async function save() {
  if (validate().length) return;
  await dataControlPlane.run(async () => {
    await client.value.data.kv.set(state.key, { value: state.value, ttl: state.ttl });
    entry.value = { key: state.key, value: state.value };
    missing.value = false;
    expiration.value = expiryLabel(state.ttl);
    remember(state.key);
  }, "Key Saved");
}
async function touch() {
  if (validate().length || state.ttl < 1) return;
  await dataControlPlane.run(async () => {
    await client.value.data.kv.touch(state.key, { ttl: state.ttl });
    expiration.value = expiryLabel(state.ttl);
  }, "Expiration Updated");
}
async function increment() {
  if (validate().length) return;
  await dataControlPlane.run(async () => {
    const result = await client.value.data.kv.increment({
      key: state.key,
      step: state.step,
      ttl: state.ttl,
    });
    state.value = String(result.value);
    entry.value = { key: result.key, value: state.value };
    missing.value = false;
    expiration.value = null;
    remember(state.key);
  }, "Counter Updated");
}
function requestDelete() {
  pendingKey.value = state.key;
  confirmation.value = "";
  deleteOpen.value = true;
}
async function remove() {
  if (
    await dataControlPlane.run(async () => {
      await client.value.data.kv.delete(pendingKey.value);
      entry.value = null;
      state.value = "";
      expiration.value = null;
      recentKeys.value = recentKeys.value.filter((key) => key !== pendingKey.value);
    }, "Key Delete Request Accepted")
  )
    deleteOpen.value = false;
}
watch(
  () => state.key,
  () => {
    entry.value = null;
    missing.value = false;
    expiration.value = null;
    state.value = "";
    dataControlPlane.error.value = "";
    dataControlPlane.notice.value = "";
  },
);
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
      title="Key–Value Store"
      description="Read and manage keys in your console Service Account namespace. Application users may see a different set of keys."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        id="data-kv"
        :state="state"
        :validate="validate"
        :disabled="dataControlPlane.busy.value"
        class="space-y-5"
        @submit="save"
      >
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            label="Key"
            name="key"
            :description="
              entry
                ? (expiration ?? 'The server does not return remaining expiration.')
                : 'Enter the key to read or update.'
            "
            required
          >
            <UInput
              v-model="state.key"
              placeholder="e.g. feature:checkout or counters:visits"
              class="w-full"
            />
          </UFormField>
          <UFormField label="Recent Keys" description="Kept only on this page.">
            <USelectMenu
              :model-value="recentKeys.includes(state.key) ? state.key : ''"
              :items="recentKeys"
              :placeholder="recentKeys.length ? 'Choose a recent key…' : 'No recent keys'"
              class="w-full"
              :disabled="!recentKeys.length || !auth.hasScope('data:query.read')"
              @update:model-value="$event && lookup($event)"
            />
          </UFormField>
        </div>
        <div class="flex justify-end">
          <UButton
            label="Read Key"
            color="neutral"
            variant="outline"
            :disabled="
              !state.key || dataControlPlane.busy.value || !auth.hasScope('data:query.read')
            "
            @click="lookup()"
          />
        </div>
        <UAlert
          v-if="missing"
          title="Key Not Found"
          description="This key does not exist in your console Service Account namespace or has expired."
          color="neutral"
        />
        <UTabs
          v-model="valueFormat"
          :items="[
            { label: 'Text', value: 'text' },
            { label: 'JSON preview', value: 'json' },
          ]"
          :content="false"
        />
        <UFormField
          v-if="valueFormat === 'text'"
          label="Value"
          name="value"
          description="Stored as text. Pasting JSON does not convert the stored value."
        >
          <UTextarea
            v-model="state.value"
            :rows="8"
            spellcheck="false"
            class="w-full font-mono"
            :disabled="!auth.hasScope('data:query.write')"
          />
        </UFormField>
        <UAlert
          v-else-if="preview === null"
          title="Invalid JSON"
          description="This value can still be read and saved as text."
          color="neutral"
        />
        <pre v-else class="max-h-72 overflow-auto whitespace-pre-wrap break-all text-sm">{{
          preview
        }}</pre>
        <div class="flex justify-end">
          <UButton
            label="Copy Value"
            icon="i-lucide-copy"
            color="neutral"
            variant="outline"
            @click="dataWorkspace.copy(state.value)"
          />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            label="Expiration (seconds)"
            name="ttl"
            description="A positive value sets the lifetime in seconds. Zero leaves expiration to the configured store. Updating expiration on an existing key requires a positive value."
          >
            <UInputNumber v-model="state.ttl" :min="0" class="w-full" />
          </UFormField>

          <UFormField
            label="Counter Step"
            name="step"
            description="A negative step decrements. The stored value must be an integer."
          >
            <UInputNumber v-model="state.step" class="w-full" />
          </UFormField>
        </div>
      </UForm>
      <template #footer>
        <UDropdownMenu
          :items="[
            [
              {
                label: 'Update Expiration',
                icon: 'i-lucide-timer',
                disabled: !entry || state.ttl < 1,
                onSelect: touch,
              },
              {
                label: 'Apply Counter Step',
                icon: 'i-lucide-plus',
                disabled: !state.key,
                onSelect: increment,
              },
            ],
            [
              {
                label: 'Delete Key',
                icon: 'i-lucide-trash-2',
                color: 'error',
                disabled: !entry,
                onSelect: requestDelete,
              },
            ],
          ]"
          :content="{ align: 'end' }"
        >
          <UButton
            label="Key Actions"
            trailing-icon="i-lucide-chevron-down"
            color="neutral"
            variant="outline"
            :disabled="dataControlPlane.busy.value || !auth.hasScope('data:query.write')"
          />
        </UDropdownMenu>

        <UButton
          label="Save Key"
          type="submit"
          form="data-kv"
          :disabled="!auth.hasScope('data:query.write')"
          :loading="dataControlPlane.busy.value"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="deleteOpen"
      title="Delete Key?"
      description="This requests permanent removal from the current key namespace. The delete response does not verify whether a value was removed."
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
      :dismissible="!dataControlPlane.busy.value"
      :close="{ disabled: dataControlPlane.busy.value }"
    >
      <template #body
        ><UFormField :label="`Type ${pendingKey} to confirm`"
          ><UInput v-model="confirmation" class="w-full" autocomplete="off" /></UFormField
      ></template>
      <template #footer
        ><UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="dataControlPlane.busy.value"
          @click="deleteOpen = false" /><UButton
          label="Confirm Delete"
          color="error"
          :disabled="confirmation !== pendingKey"
          :loading="dataControlPlane.busy.value"
          @click="remove"
      /></template>
    </UModal>
  </div>
</template>
