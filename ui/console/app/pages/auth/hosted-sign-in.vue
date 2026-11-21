<script setup lang="ts">
import { authHostedSignInSections } from "../../utils/auth-forms";
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
const sections = computed(() => {
  const config = authControlPlane.config.value;
  const dependencies: Record<string, boolean> = {
    "oidc.ui.showPassword": !!config?.password.enabled,
    "oidc.ui.showSignUp": !!config?.password.enabled,
    "oidc.ui.showPasskeys": !!config?.passkeys.enabled,
    "oidc.ui.showEmailOtp": !!config?.emailOtp.enabled,
    "oidc.ui.showSmsOtp": !!config?.smsOtp.enabled,
    "oidc.ui.showOauth": Object.values(config?.oauthProviders || {}).some(
      (provider) => provider.enabled,
    ),
  };
  return authHostedSignInSections.map((section) => ({
    ...section,
    fields: section.fields.map((field) => ({
      ...field,
      disabled: dependencies[field.name] === false,
      description:
        dependencies[field.name] === false
          ? `${field.description} Enable the corresponding sign-in method or identity provider first.`
          : field.description,
    })),
  }));
});
onMounted(() => {
  if (auth.hasScope("auth:config.read")) return authControlPlane.loadConfig();
});
</script>
<template>
  <AuthConfigurationEditor
    :control="authControlPlane"
    :sections="sections"
    title="Hosted Sign-In"
    description="Configure Layr's hosted OpenID Connect sign-in experience: branding, legal links, and visible authentication methods. Applications start sign-in with an authorization request; this is not a standalone application login URL."
    save-label="Save Hosted Sign-In"
  />
</template>
