<script setup lang="ts">
import { authSignInMethodSections } from "../../utils/auth-forms";
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
onMounted(() => {
  if (auth.hasScope("auth:config.read")) return authControlPlane.loadConfig();
});
</script>
<template>
  <div class="space-y-6">
    <AuthConfigurationEditor
      :control="authControlPlane"
      :sections="authSignInMethodSections"
      title="Sign-In Methods"
      description="Configure passwords, passkeys, email codes, SMS codes, and guest access for your application. Use these methods directly through the Layr SDK; no application registration is required."
      save-label="Save Sign-In Methods"
    />
  </div>
</template>
