<script setup lang="ts">
import type { FormError } from "@nuxt/ui";
import { validFunctionDate, validFunctionDomain } from "../../utils/function-forms";
const auth = useAuth();
const client = useClient();
const functionControlPlane = useFunctionControlPlane(client.value.function.cp);
const allowedKeys = ref<string[]>([]);
const compatibilityFlags = ref<string[]>([]);
const configFormTemplateRef = useTemplateRef("configFormElement");
const bytesPerMebibyte = 1024 * 1024;
const bundleSizeMib = computed({
  get: () => (functionControlPlane.config.value?.maxBundleSizeBytes || 0) / bytesPerMebibyte,
  set: (value: number) => {
    if (functionControlPlane.config.value)
      functionControlPlane.config.value.maxBundleSizeBytes = Math.round(value * bytesPerMebibyte);
  },
});
async function load() {
  if (!auth.hasScope("function:config.read")) return;
  await functionControlPlane.load("configuration");
  allowedKeys.value = [...(functionControlPlane.config.value?.allowedEnvironmentKeys || [])];
  compatibilityFlags.value = [
    ...(functionControlPlane.config.value?.workerdRuntime.compatibilityFlags || []),
  ];
}
function validateConfig(): FormError[] {
  const config = functionControlPlane.config.value;
  if (!config) return [];
  const errors: FormError[] = [];
  if (config.defaultRuntime !== "workerd")
    errors.push({ name: "defaultRuntime", message: "Only the workerd runtime is available." });
  for (const key of ["defaultTimeoutSeconds", "maxBundleSizeBytes"] as const)
    if (!Number.isSafeInteger(config[key]) || config[key] < 1)
      errors.push({ name: key, message: "Enter a positive whole number of seconds or bytes." });
  if (allowedKeys.value.some((key) => !/^[A-Za-z_][A-Za-z0-9_]*$/.test(key.trim())))
    errors.push({
      name: "allowedEnvironmentKeys",
      message: "Use environment variable names such as API_URL, without spaces or values.",
    });
  if (compatibilityFlags.value.some((flag) => !/^[a-z][a-z0-9_]*$/.test(flag.trim())))
    errors.push({
      name: "workerdRuntime.compatibilityFlags",
      message: "Use compatibility flag names such as nodejs_compat, without spaces.",
    });
  if (config.defaultTlsAcmeEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(config.defaultTlsAcmeEmail))
    errors.push({ name: "defaultTlsAcmeEmail", message: "Enter a valid ACME contact email." });
  if (
    config.customDomainCnameTarget &&
    config.customDomainCnameTarget !== "localhost" &&
    !validFunctionDomain(config.customDomainCnameTarget)
  )
    errors.push({
      name: "customDomainCnameTarget",
      message: "Enter a hostname without a scheme or path.",
    });
  if (config.customDomainApexIp) {
    const value = config.customDomainApexIp;
    const ipv4 =
      /^(?:\d{1,3}\.){3}\d{1,3}$/.test(value) &&
      value.split(".").every((part) => Number(part) <= 255);
    let ipv6 = false;
    try {
      if (value.includes(":")) {
        new URL(`http://[${value}]/`);
        ipv6 = true;
      }
    } catch {
      ipv6 = false;
    }
    if (!ipv4 && !ipv6)
      errors.push({ name: "customDomainApexIp", message: "Enter a valid IPv4 or IPv6 address." });
  }
  if (!validFunctionDate(config.workerdRuntime.compatibilityDate || ""))
    errors.push({
      name: "workerdRuntime.compatibilityDate",
      message: "Choose a valid compatibility date.",
    });
  return errors;
}
async function save() {
  if (!functionControlPlane.config.value) return;
  const errors = validateConfig();
  if (errors.length) {
    configFormTemplateRef.value?.setErrors(errors);
    return;
  }
  const allowedEnvironmentKeys = [...new Set(allowedKeys.value.map((key) => key.trim()))];
  const flags = [...new Set(compatibilityFlags.value.map((flag) => flag.trim()))];
  if (
    await functionControlPlane.saveConfig({
      ...functionControlPlane.config.value,
      allowedEnvironmentKeys,
      workerdRuntime: {
        ...functionControlPlane.config.value.workerdRuntime,
        compatibilityFlags: flags,
      },
    })
  ) {
    allowedKeys.value = allowedEnvironmentKeys;
    compatibilityFlags.value = flags;
  }
}
onMounted(load);
</script>
<template>
  <div class="space-y-6" :aria-busy="functionControlPlane.busy.value">
    <UAlert
      v-if="functionControlPlane.error.value"
      title="Request Failed"
      :description="functionControlPlane.error.value"
      color="error"
      role="alert"
    />
    <UAlert
      v-if="functionControlPlane.notice.value"
      :title="functionControlPlane.notice.value"
      color="success"
      role="status"
    />
    <UCard
      title="Function Configuration"
      description="Manage endpoint defaults, deployment policy, workerd compatibility, and custom-domain ingress. Code, variable values, and domain routes are managed on their own pages."
      :ui="{ footer: 'flex justify-end' }"
    >
      <UForm
        v-if="functionControlPlane.config.value"
        id="function-configuration"
        ref="configFormElement"
        :state="functionControlPlane.config.value"
        :validate="validateConfig"
        :disabled="functionControlPlane.busy.value || !auth.hasScope('function:config.write')"
        class="space-y-6"
        @submit="save"
      >
        <USeparator label="Endpoint Defaults" position="start" />
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            label="Default Runtime"
            name="defaultRuntime"
            description="Used when creating an endpoint without a runtime. workerd is the only registered runtime."
            required
          >
            <USelect
              v-model="functionControlPlane.config.value.defaultRuntime"
              :items="['workerd']"
              placeholder="Choose workerd"
              class="w-full"
            />
          </UFormField>
          <UFormField
            label="Default Timeout (Seconds)"
            name="defaultTimeoutSeconds"
            description="Copied to new endpoints unless overridden. Changing this default does not update existing endpoint timeouts."
            required
          >
            <UInputNumber
              v-model="functionControlPlane.config.value.defaultTimeoutSeconds"
              :min="1"
              class="w-full"
            />
          </UFormField>
        </div>
        <UAlert
          title="Memory Limits Are Not Enforced"
          description="workerd does not enforce the stored per-endpoint memory setting. It is not exposed here as a configurable limit; existing memory metadata is preserved when saving."
          color="warning"
        />
        <USeparator label="Deployment Policy" position="start" />
        <UFormField
          label="Maximum Bundle Size (MiB)"
          name="maxBundleSizeBytes"
          description="Maximum bundle bytes accepted on upload, and maximum total extracted file content when deploying or rolling back."
          :help="`${functionControlPlane.config.value.maxBundleSizeBytes.toLocaleString()} bytes. 1 MiB = 1,048,576 bytes.`"
          required
        >
          <UInputNumber
            v-model="bundleSizeMib"
            :min="1 / bytesPerMebibyte"
            :step="1"
            :step-snapping="false"
            :format-options="{ maximumFractionDigits: 6 }"
            class="w-full"
          />
        </UFormField>
        <UFormField
          label="Allowed Environment Keys"
          name="allowedEnvironmentKeys"
          description="Deployment variable names, not secret values. Empty allows any key. A non-empty list restricts new deployments and rollbacks; it does not rewrite variables in running deployments."
          help="Press Enter to add a key. Paste comma-separated or newline-separated names. Configure values on the Deployments page."
        >
          <UInputTags
            v-model="allowedKeys"
            placeholder="API_URL"
            :delimiter="/[\n,]/"
            add-on-paste
            add-on-blur
            class="w-full"
          />
        </UFormField>
        <USeparator label="Workerd Compatibility" position="start" />
        <UAlert
          title="Applied on Runtime Reload"
          description="Saving does not restart workerd. These defaults are read when the shared runtime is next rebuilt, including deployment, rollback, undeploy, or recovery. Existing deployments without overrides also inherit them on reload."
          color="info"
        />
        <UFormField
          label="Compatibility Date"
          name="workerdRuntime.compatibilityDate"
          description="Selects workerd API behavior. A deployment-specific date takes precedence over this service default."
          required
        >
          <UInput
            v-model="functionControlPlane.config.value.workerdRuntime.compatibilityDate"
            type="date"
            class="w-full"
          />
        </UFormField>
        <UFormField
          label="Compatibility Flags"
          name="workerdRuntime.compatibilityFlags"
          description="Explicit flags supported by the installed workerd version. Empty sends no explicit flags. A deployment-specific flag list replaces this default rather than merging with it."
          help="Press Enter to add a flag. Compatibility flags are checked by workerd when the runtime loads."
        >
          <UInputTags
            v-model="compatibilityFlags"
            placeholder="nodejs_compat"
            :delimiter="/[\n,]/"
            add-on-paste
            add-on-blur
            class="w-full"
          />
        </UFormField>
        <USeparator label="Custom Domain Ingress" position="start" />
        <UAlert
          title="DNS and TLS Must Be Configured Separately"
          description="These values provide DNS verification targets and an ACME contact. They do not create DNS records, change the gateway listening address, or enable gateway TLS. Register domains and manage their TLS and routes on the Domains page."
          color="info"
        />
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            label="Custom Domain CNAME Target"
            name="customDomainCnameTarget"
            description="Public hostname domains should resolve to. Empty or localhost inherits the gateway's first configured TLS domain, otherwise its base URL hostname."
          >
            <UInput
              v-model="functionControlPlane.config.value.customDomainCnameTarget"
              placeholder="Use gateway hostname"
              class="w-full"
            />
          </UFormField>
          <UFormField
            label="Custom Domain Apex IP"
            name="customDomainApexIp"
            description="Optional ingress IP for apex-domain DNS verification. If empty, an IP-valued CNAME target is used; otherwise the instructions use ALIAS/CNAME flattening."
          >
            <UInput
              v-model="functionControlPlane.config.value.customDomainApexIp"
              placeholder="Use CNAME target"
              class="w-full"
            />
          </UFormField>
        </div>
        <UFormField
          label="Default ACME Email"
          name="defaultTlsAcmeEmail"
          description="Fallback contact for domains without their own ACME email. Empty inherits the gateway ACME email. Certificate issuance still requires gateway TLS to be enabled."
        >
          <UInput
            v-model="functionControlPlane.config.value.defaultTlsAcmeEmail"
            type="email"
            placeholder="Use gateway ACME email"
            class="w-full"
          />
        </UFormField>
      </UForm>
      <UEmpty
        v-else
        :title="
          functionControlPlane.busy.value
            ? 'Loading Function Configuration…'
            : 'Configuration Unavailable'
        "
        :loading="functionControlPlane.busy.value"
        icon="i-lucide-settings"
      />
      <template v-if="auth.hasScope('function:config.write')" #footer>
        <UButton
          type="submit"
          form="function-configuration"
          label="Save Function Config"
          :disabled="functionControlPlane.busy.value || !functionControlPlane.config.value"
        />
      </template>
    </UCard>
  </div>
</template>
