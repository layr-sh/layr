<script setup lang="ts">
import { authConfigurationSections } from "../../utils/auth-forms";
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
onMounted(() => {
  if (auth.hasScope("auth:config.read")) return authControlPlane.loadConfig();
});
</script>
<template>
  <AuthConfigurationEditor
    :control="authControlPlane"
    :sections="authConfigurationSections"
    title="Session and Token Settings"
    description="Configure application access tokens, refresh sessions, and session caching. Changes apply to newly issued tokens; existing access tokens remain valid until expiry."
    save-label="Save Session Settings"
  />
</template>
