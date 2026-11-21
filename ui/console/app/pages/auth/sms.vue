<script setup lang="ts">
import { authSMSSections, authSMSTemplates } from "../../utils/auth-forms";
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
    :sections="authSMSSections"
    :templates="authSMSTemplates"
    template-hook="public.auth_sms_template(text,text,text,uuid)"
    title="SMS"
    description="Connect Twilio or an SMS webhook and customize phone verification, sign-in, and password reset messages. Configure your provider before enabling SMS-code authentication."
    save-label="Save SMS Settings"
  />
</template>
