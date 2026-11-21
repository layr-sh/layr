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
    kind="resource"
    title="APIs"
    description="Register the APIs that receive machine-to-machine access tokens. Each API defines an audience identifier and available scopes. Machine credentials come from Service Accounts. For external API audiences, the backend checks the API's scope list—not per-account scope grants; enforce per-client authorization in your API."
  />
</template>
