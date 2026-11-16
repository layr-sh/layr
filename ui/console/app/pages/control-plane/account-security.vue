<script setup lang="ts">
const auth = useAuth();
const client = useClient();
const controlPlane = useControlPlane(client.value.core.cp, client.value.console.cp);

const enrollment = ref<{ secret: string; uri: string } | null>(null);
const code = ref<number[]>([]);
const confirmingDisable = ref(false);

function cancelEnrollment() {
  enrollment.value = null;
  code.value = [];
}
async function setupMFA() {
  await controlPlane.run(async () => {
    enrollment.value = await auth.setupMFA();
    code.value = [];
  });
}
async function enableMFA() {
  if (
    await controlPlane.run(
      () => auth.enableMFA(code.value.join("")),
      "Multi-Factor Authentication Enabled",
    )
  )
    cancelEnrollment();
}
async function disableMFA() {
  if (await controlPlane.run(() => auth.disableMFA(), "Multi-Factor Authentication Disabled"))
    confirmingDisable.value = false;
}
async function signOut() {
  await controlPlane.run(async () => {
    const result = await auth.signOut();
    if (result.error) throw result.error;
    await navigateTo("/sign-in");
  });
}
</script>

<template>
  <div class="space-y-6" :aria-busy="controlPlane.busy.value">
    <UAlert
      v-if="controlPlane.error.value"
      color="error"
      title="Request Failed"
      :description="controlPlane.error.value"
      role="alert"
    />
    <UAlert
      v-if="controlPlane.notice.value"
      color="success"
      :title="controlPlane.notice.value"
      role="status"
    />
    <UAlert
      v-if="auth.state.value?.mfa_required"
      color="warning"
      title="Multi-Factor Authentication Required"
      :description="
        auth.state.value.user.mfa_enabled
          ? 'Sign out and sign in again with your authenticator code to restore platform access.'
          : 'Enroll your authenticator below to restore platform access.'
      "
    />
    <UCard
      title="Account Security"
      description="Protect your Console User and manage this session."
      :ui="{ footer: 'flex justify-end' }"
    >
      <div class="grid gap-4 sm:grid-cols-2">
        <UFormField label="Console User"
          ><UInput :model-value="auth.state.value?.user.email" readonly class="w-full"
        /></UFormField>
        <UFormField label="Session Expires"
          ><UInput
            :model-value="
              auth.state.value ? new Date(auth.state.value.expires_at).toLocaleString() : ''
            "
            readonly
            class="w-full"
        /></UFormField>
      </div>
      <template #footer
        ><UButton
          label="Sign Out This Session"
          color="neutral"
          variant="outline"
          :loading="controlPlane.busy.value"
          @click="signOut"
      /></template>
    </UCard>
    <UCard
      title="Multi-Factor Authentication"
      description="Use a time-based code from your authenticator app when signing in."
      :ui="{ body: 'space-y-5', footer: 'flex flex-wrap justify-end gap-3' }"
    >
      <div class="flex">
        <UBadge
          :color="auth.state.value?.user.mfa_enabled ? 'success' : 'warning'"
          variant="subtle"
          >{{ auth.state.value?.user.mfa_enabled ? "Enrolled" : "Not Enrolled" }}</UBadge
        >
      </div>
      <template v-if="enrollment">
        <UAlert
          color="info"
          title="Add Your Authenticator"
          description="Enter this setup key in your authenticator app, then verify a six-digit code. Setup expires after ten minutes."
        />
        <UFormField label="Setup Key"
          ><UInput :model-value="enrollment.secret" readonly autocomplete="off" class="w-full"
        /></UFormField>
        <UFormField label="Authenticator URI"
          ><UInput :model-value="enrollment.uri" readonly class="w-full"
        /></UFormField>
        <UForm
          id="mfa-enrollment"
          :state="{ code }"
          :disabled="controlPlane.busy.value"
          class="space-y-4"
          @submit="enableMFA"
        >
          <UFormField label="Verification Code" required
            ><UPinInput v-model="code" type="number" :length="6" otp
          /></UFormField>
        </UForm>
      </template>
      <template #footer>
        <template v-if="enrollment">
          <UButton
            label="Cancel Enrollment"
            color="neutral"
            variant="outline"
            :disabled="controlPlane.busy.value"
            @click="cancelEnrollment"
          />
          <UButton
            type="submit"
            form="mfa-enrollment"
            label="Verify and Enable MFA"
            :disabled="controlPlane.busy.value || code.length !== 6"
          />
        </template>
        <UButton
          v-else-if="!auth.state.value?.user.mfa_enabled"
          label="Set Up MFA"
          :loading="controlPlane.busy.value"
          @click="setupMFA"
        />
        <UButton
          v-else
          label="Disable MFA"
          color="error"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDisable = true"
        />
      </template>
    </UCard>
    <UModal
      v-model:open="confirmingDisable"
      title="Disable Multi-Factor Authentication?"
      description="Your Console User will no longer require an authenticator code at sign-in."
      :dismissible="!controlPlane.busy.value"
    >
      <template #footer>
        <UButton
          label="Cancel"
          color="neutral"
          variant="outline"
          :disabled="controlPlane.busy.value"
          @click="confirmingDisable = false"
        />
        <UButton
          label="Disable MFA"
          color="error"
          :loading="controlPlane.busy.value"
          @click="disableMFA"
        />
      </template>
    </UModal>
  </div>
</template>
