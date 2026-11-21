<script setup lang="ts">
import { authSecuritySections } from "../../utils/auth-forms";
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
    :sections="authSecuritySections"
    title="Security"
    description="Configure multi-factor authentication, sign-in throttling, bot protection, and new-device notifications for application users."
    save-label="Save Security Settings"
  />
</template>
