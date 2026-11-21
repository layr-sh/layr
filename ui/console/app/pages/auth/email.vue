<script setup lang="ts">
import { authEmailSections, authEmailTemplates } from "../../utils/auth-forms";
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
    :sections="authEmailSections"
    :templates="authEmailTemplates"
    template-hook="public.auth_email_template(text,text,text,uuid)"
    title="Email"
    description="Connect an SMTP server or email webhook and customize verification, sign-in, password reset, and security messages. Configure your provider before enabling email-code authentication."
    save-label="Save Email Settings"
  />
</template>
