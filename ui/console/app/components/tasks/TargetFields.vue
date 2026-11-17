<script setup lang="ts">
import { loadTaskTarget, resolveTaskTarget, type TaskTargetForm } from "../../utils/task-forms";
const props = defineProps<{ targetType: string }>();
const taskTargetForm = defineModel<TaskTargetForm>({ required: true });
const modeError = ref("");
function setAdvanced(advanced: boolean) {
  const taskTarget = resolveTaskTarget(taskTargetForm.value, props.targetType);
  // Missing required URL/query fields are safe to carry into JSON. Block only
  // forward conversions that would discard input, or invalid JSON-to-fields conversions.
  if (
    taskTarget.errors.length &&
    (!advanced ||
      taskTarget.errors.some((error) =>
        ["target.headers", "target.body", "target.params"].includes(error.name),
      ))
  ) {
    modeError.value =
      "Fix the target fields before switching editor modes. Your input has been kept.";
    return;
  }
  modeError.value = "";
  if (advanced) taskTargetForm.value.raw = JSON.stringify(taskTarget.payload, null, 2);
  else loadTaskTarget(taskTargetForm.value, taskTarget.payload);
  taskTargetForm.value = { ...taskTargetForm.value, advanced };
}
</script>

<template>
  <div class="space-y-5">
    <USwitch
      :model-value="taskTargetForm.advanced"
      label="Advanced JSON"
      description="Edit the complete target payload as JSON. Switching modes preserves your target settings."
      @update:model-value="setAdvanced"
    />
    <UAlert
      v-if="modeError"
      title="Target Needs Attention"
      :description="modeError"
      color="warning"
      role="alert"
    />
    <UFormField
      v-if="taskTargetForm.advanced"
      label="Target Payload (JSON)"
      name="target.raw"
      description="The complete target object. HTTP uses url, headers, and body; SQL uses query and params."
      required
    >
      <UTextarea v-model="taskTargetForm.raw" :rows="8" class="w-full font-mono" />
    </UFormField>
    <template v-else-if="targetType === 'http'">
      <UFormField
        label="Webhook URL"
        name="target.url"
        description="A complete HTTP or HTTPS URL. The task sends an HTTP POST request."
        required
      >
        <UInput
          v-model="taskTargetForm.url"
          placeholder="https://example.com/webhooks/task"
          class="w-full"
        />
      </UFormField>
      <UFormField
        label="Request Headers"
        name="target.headers"
        description="Optional. One Header-Name: value per line. Do not repeat header names."
        help="For example: Authorization: Bearer your-token"
      >
        <UTextarea
          v-model="taskTargetForm.headers"
          :rows="3"
          placeholder="X-Source: layr-tasks"
          class="w-full font-mono"
        />
      </UFormField>
      <UFormField
        label="Request Body (JSON)"
        name="target.body"
        description="Optional JSON sent in the POST body. Leave blank for an empty body."
      >
        <UTextarea
          v-model="taskTargetForm.body"
          :rows="5"
          placeholder='{"event": "sync"}'
          class="w-full font-mono"
        />
      </UFormField>
    </template>
    <template v-else>
      <UFormField
        label="SQL Query"
        name="target.query"
        description="Runs against the project database. Use $1, $2, and so on for parameters."
        required
      >
        <UTextarea
          v-model="taskTargetForm.query"
          :rows="5"
          placeholder="SELECT my_function($1)"
          class="w-full font-mono"
        />
      </UFormField>
      <UFormField
        label="SQL Parameters (JSON)"
        name="target.params"
        description="Optional JSON array in placeholder order. Leave blank when there are no parameters."
      >
        <UTextarea
          v-model="taskTargetForm.params"
          :rows="3"
          placeholder="[42]"
          class="w-full font-mono"
        />
      </UFormField>
      <UAlert
        title="SQL Can Change Project Data"
        description="Only run queries you trust. A successful request may have side effects that cannot be undone."
        color="warning"
      />
    </template>
  </div>
</template>
