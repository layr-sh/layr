import { createServer, type Server } from "node:http";
import { createHmac } from "node:crypto";
import { afterAll, describe, expect, it, vi } from "vitest";
import { useNuxtApp } from "#app";
import { createClient, useAuth as useAuthService } from "layr-client";
import type { AuthControlPlaneModels as Models } from "layr-client";
import Email from "../../pages/auth/email.vue";
import SMS from "../../pages/auth/sms.vue";
import { authConfigInput } from "../../utils/auth-forms";
import { authEndpoint, authTestUser, rootAuth } from "./helpers";
import {
  field,
  flushPromises,
  mountPage,
  pageDOM,
  selectOption,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let webhookServer: Server;
let original: Models.Config;
let userId: string;
const messages: { body: Record<string, string>; raw: string; signature: string }[] = [];
const signingSecret = "auth-console-webhook-template-test";
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
afterAll(async () => {
  wrapper?.unmount();
  if (userId) await rootAuth.cp.users.delete(userId);
  if (original) await rootAuth.cp.config.update(authConfigInput(original));
  if (webhookServer)
    await new Promise<void>((resolve, reject) =>
      webhookServer.close((error) => (error ? reject(error) : resolve())),
    );
  await useNuxtApp().$router.push("/");
});

describe("Auth email and SMS message flow", () => {
  it("saves all message templates through the console and verifies actual backend interpolation and webhook signatures", async () => {
    await signInConsoleUser();
    original = await rootAuth.cp.config.get();
    webhookServer = createServer((request, response) => {
      const chunks: Buffer[] = [];
      request.on("data", (chunk: Buffer) => chunks.push(chunk));
      request.on("end", () => {
        const raw = Buffer.concat(chunks).toString();
        messages.push({
          body: JSON.parse(raw),
          raw,
          signature: String(request.headers["webhook-signature"] || ""),
        });
        response.writeHead(204);
        response.end();
      });
    });
    await new Promise<void>((resolve, reject) => {
      webhookServer.once("error", reject);
      webhookServer.listen(0, "127.0.0.1", resolve);
    });
    const address = webhookServer.address();
    if (!address || typeof address === "string") throw new Error("Expected a TCP listener");
    const webhookUrl = `http://127.0.0.1:${address.port}`;
    const codeTemplate = "{{.AppName}}|{{.Recipient}}|{{.To}}|{{.Code}}|{{.Unknown}}|{{ .Code }}";
    const alertTemplate = "{{.AppName}}|{{.Recipient}}|{{.To}}|{{.Device}}|{{.IP}}|{{.Code}}";

    ({ wrapper } = await mountPage(Email));
    await selectOption("Email Provider", "webhook");
    expect(pageDOM().text()).not.toContain("SMTP Host");
    await field("Email Webhook URL").setValue(webhookUrl);
    await field("Email Webhook Signing Secret").setValue(signingSecret);
    for (const label of [
      "Password Reset",
      "Sign-In Code",
      "Email Verification",
      "Suspicious Activity",
    ]) {
      await selectOption("Message", label);
      expect(pageDOM().text()).toContain("public.auth_email_template(text,text,text,uuid)");
      if (label === "Suspicious Activity") {
        expect(pageDOM().text()).toContain("{{.Device}}");
        expect(pageDOM().text()).toContain("{{.IP}}");
      }
      await field("Subject").setValue(`${label}: {{.Recipient}}`);
      await field("HTML Body").setValue("");
      await field("Plain-Text Body").setValue(
        label === "Suspicious Activity" ? alertTemplate : codeTemplate,
      );
    }
    await pageDOM().get("#auth-configuration").trigger("submit");
    await settle();
    expect(wrapper.setupState.authControlPlane.error.value).toBe("");
    expect(messages).toHaveLength(0);
    expect((await rootAuth.cp.config.get()).emailDispatcher.webhook).toMatchObject({
      signingSecretConfigured: true,
      url: webhookUrl,
    });
    expect(field("Email Webhook Signing Secret").element).toHaveProperty("value", "");
    await field("Sender Name").setValue("Auth Console Test");
    await pageDOM().get("#auth-configuration").trigger("submit");
    await settle();
    expect((await rootAuth.cp.config.get()).emailDispatcher.webhook.signingSecretConfigured).toBe(
      true,
    );
    wrapper.unmount();

    ({ wrapper } = await mountPage(SMS));
    await selectOption("SMS Provider", "webhook");
    expect(pageDOM().text()).not.toContain("Twilio Account SID");
    await field("SMS Webhook URL").setValue(webhookUrl);
    await field("SMS Webhook Signing Secret").setValue(signingSecret);
    for (const label of ["Password Reset", "Sign-In Code", "Phone Verification"]) {
      await selectOption("Message", label);
      expect(pageDOM().text()).toContain("{{.Recipient}}");
      expect(pageDOM().text()).toContain("{{.To}}");
      expect(pageDOM().text()).toContain("public.auth_sms_template(text,text,text,uuid)");
      await field("Message Body").setValue(codeTemplate);
    }
    await pageDOM().get("#auth-configuration").trigger("submit");
    await settle();
    expect(wrapper.setupState.authControlPlane.error.value).toBe("");
    expect(messages).toHaveLength(0);
    const config = await rootAuth.cp.config.get();
    expect(config.smsDispatcher.webhook.signingSecretConfigured).toBe(true);
    config.password.enabled = config.emailOtp.enabled = config.smsOtp.enabled = true;
    config.threat.botProtection.enabled = false;
    config.password.breachCheck.enabled = false;
    config.threat.notifyOnNewDevice = true;
    await rootAuth.cp.config.update(authConfigInput(config));

    const input = { ...authTestUser(), phone: `+1555${Date.now().toString().slice(-7)}` };
    const user = await rootAuth.cp.users.create(input);
    userId = user.id;
    const nuxtApp = useNuxtApp();
    const publicAuthService = useAuthService(
      createClient({
        endpoint: authEndpoint,
        publishableKey: nuxtApp.$manifest.state.value.publishableKey,
        headers: { "User-Agent": "Auth console template test" },
      }),
    );
    await publicAuthService.password_reset.request({ email: input.email });
    await publicAuthService.password_reset.request({ phone: input.phone });
    await publicAuthService.otp.send({ recipient: input.email, purpose: "sign_in" });
    await publicAuthService.otp.send({ recipient: input.phone, purpose: "sign_in" });
    await publicAuthService.user.email.verification.request({ email: input.email });
    await publicAuthService.user.phone.verification.request({ phone: input.phone });
    await publicAuthService.sign_in({ email: input.email, phone: "", password: input.password });
    expect(messages).toHaveLength(6);
    const secondDeviceAuthService = useAuthService(
      createClient({
        endpoint: authEndpoint,
        publishableKey: nuxtApp.$manifest.state.value.publishableKey,
        headers: { "User-Agent": "Auth console second device" },
      }),
    );
    await secondDeviceAuthService.sign_in({
      email: input.email,
      phone: "",
      password: input.password,
    });
    expect(messages).toHaveLength(7);
    const projectName = nuxtApp.$manifest.state.value.project?.name;
    for (const message of messages) {
      const timestamp = /t=(\d+),v1=/.exec(message.signature)![1];
      const expectedSignature = createHmac("sha256", signingSecret)
        .update(`${timestamp}.${message.raw}`)
        .digest("hex");
      expect(message.signature).toBe(`t=${timestamp},v1=${expectedSignature}`);
      expect(message.body.user_id || "").toBe(
        message.body.message_kind === "sign_in_otp" ||
          message.body.message_kind === "email_verification"
          ? ""
          : user.id,
      );
      if (message.body.message_kind === "suspicious_activity") {
        expect(message.body.text).toContain(
          `${projectName || "layr-app"}|${input.email}|${input.email}|Auth console second device|`,
        );
        expect(message.body.text).toContain("|{{.Code}}");
      } else {
        const appName = projectName || (message.body.kind === "sms" ? "Layr" : "layr-app");
        expect(message.body.text).toBe(
          `${appName}|${message.body.to}|${message.body.to}|${message.body.code}|{{.Unknown}}|{{ .Code }}`,
        );
      }
      if (message.body.kind === "email") expect(message.body.subject).toContain(message.body.to);
    }
    expect(messages.map((message) => `${message.body.kind}:${message.body.message_kind}`)).toEqual([
      "email:password_reset",
      "sms:password_reset",
      "email:sign_in_otp",
      "sms:sign_in_otp",
      "email:email_verification",
      "sms:phone_verification",
      "email:suspicious_activity",
    ]);
    const sessions = await rootAuth.cp.users.sessions.list(user.id);
    expect(sessions.data).toHaveLength(2);
    await rootAuth.cp.users.sessions.revoke(user.id);
    expect((await rootAuth.cp.users.sessions.list(user.id)).data).toEqual([]);
  });
});
