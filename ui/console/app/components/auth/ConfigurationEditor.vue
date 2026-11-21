<script setup lang="ts">
import type useAuthControlPlane from "../../composables/use-auth-control-plane";
import {
  validateAuthFields,
  type AuthSection,
  type AuthMessageTemplate,
} from "../../utils/auth-forms";
const props = defineProps<{
  control: ReturnType<typeof useAuthControlPlane>;
  sections: AuthSection[];
  title: string;
  description: string;
  saveLabel: string;
  templates?: AuthMessageTemplate[];
  templateHook?: string;
}>();
const auth = useAuth();
const selectedTemplate = ref(props.templates?.[0]?.label || "");
const template = computed(() =>
  props.templates?.find((item) => item.label === selectedTemplate.value),
);

const formTemplateRef = useTemplateRef("configurationFormElement");
function validate() {
  return props.control.config.value
    ? validateAuthFields(
        props.control.config.value,
        props.sections.flatMap((section) => section.fields),
      )
    : [];
}
async function save() {
  if (!props.control.config.value) return;
  const errors = validate();
  if (errors.length) {
    formTemplateRef.value?.setErrors(errors);
    return;
  }
  await props.control.saveConfig(props.control.config.value);
}
</script>
<template>
  <div class="space-y-6" :aria-busy="control.busy.value">
    <UAlert
      v-if="control.error.value"
      title="Request Failed"
      :description="control.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="control.notice.value"
      :title="control.notice.value"
      color="success"
      role="status"
    />
    <UCard
      :title="title"
      :description="description"
      :ui="{ footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <UForm
        v-if="control.config.value"
        id="auth-configuration"
        ref="configurationFormElement"
        :state="control.config.value"
        :validate="validate"
        :disabled="control.busy.value || !auth.hasScope('auth:config.write')"
        class="space-y-6"
        @submit="save"
      >
        <section v-for="section in sections" :key="section.label" class="space-y-5">
          <header v-if="sections.length + (templates?.length ? 1 : 0) > 1" class="space-y-2">
            <USeparator :label="section.label" position="start" />
            <p class="text-sm text-muted">{{ section.description }}</p>
          </header>
          <AuthFields :state="control.config.value" :fields="section.fields" />
        </section>
        <section v-if="templates?.length" class="space-y-5">
          <header class="space-y-2">
            <USeparator label="Message Templates" position="start" />
            <p class="text-sm text-muted">
              Choose a message to customize. Changes to all templates are saved together with the
              provider settings.
            </p>
          </header>
          <UFormField
            label="Message"
            description="Select the authentication message you want to edit."
          >
            <USelect
              v-model="selectedTemplate"
              :items="templates.map((item) => item.label)"
              class="w-full"
            />
          </UFormField>
          <template v-if="template">
            <p class="text-sm text-muted">{{ template.description }}</p>
            <div class="space-y-2">
              <p class="text-sm font-medium">Supported Placeholders</p>
              <div class="flex flex-wrap gap-2">
                <UBadge
                  v-for="variable in template.variables"
                  :key="variable"
                  :label="variable"
                  color="neutral"
                  variant="subtle"
                />
              </div>
              <p class="text-sm text-muted">
                Use placeholders exactly as shown. Other syntax, conditions, and loops are not
                evaluated.
              </p>
            </div>
            <UAlert
              title="Template Precedence"
              :description="`A valid result from ${templateHook} overrides these settings and is sent without placeholder substitution. Otherwise, Layr uses this template, then a built-in message if it is incomplete. Saving does not send a test message.`"
              color="warning"
              variant="subtle"
            />
            <AuthFields :state="control.config.value" :fields="template.fields" />
          </template>
        </section>
      </UForm>
      <UEmpty
        v-else
        :title="control.busy.value ? 'Loading Auth Configuration…' : 'Configuration Unavailable'"
        :loading="control.busy.value"
        icon="i-lucide-settings"
      />
      <template #footer>
        <UButton
          label="Reload"
          color="neutral"
          variant="outline"
          :disabled="control.busy.value || !auth.hasScope('auth:config.read')"
          @click="control.loadConfig()"
        />
        <UButton
          v-if="auth.hasScope('auth:config.write')"
          :label="saveLabel"
          type="submit"
          form="auth-configuration"
          :disabled="control.busy.value || !control.config.value"
        />
      </template>
    </UCard>
  </div>
</template>
