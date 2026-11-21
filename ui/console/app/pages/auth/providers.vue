<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const authControlPlane = useAuthControlPlane(client.value.auth.cp);
onMounted(() => {
  if (auth.hasScope("auth:config.read")) return authControlPlane.loadConfig();
});
</script>
<template>
  <AuthRecords
    :control="authControlPlane"
    kind="provider"
    title="Identity Providers"
    description="Connect Google, GitHub, Apple, Discord, or a custom OAuth provider so users can sign in with an existing account. Register the callback URL shown in each connection with your provider."
  />
</template>
