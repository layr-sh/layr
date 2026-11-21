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
    kind="client"
    title="Applications"
    description="OpenID Connect client registrations for hosted sign-in. Direct authentication through the Layr SDK does not require a registration. All OIDC clients require S256 PKCE; confidential server clients additionally require a secret. Machine-to-machine credentials are managed through Service Accounts."
  />
</template>
