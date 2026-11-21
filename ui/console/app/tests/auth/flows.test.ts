import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useNuxtApp } from "#app";
import { createClient, useAuth as useAuthService } from "layr-client";
import type { AuthControlPlaneModels as Models } from "layr-client";
import Users from "../../pages/auth/users.vue";
import Sessions from "../../pages/auth/sessions.vue";
import Configuration from "../../pages/auth/configurations.vue";
import SignInMethods from "../../pages/auth/sign-in-methods.vue";
import Security from "../../pages/auth/security.vue";
import HostedSignIn from "../../pages/auth/hosted-sign-in.vue";
import Layout from "../../pages/auth.vue";
import { authConfigInput } from "../../utils/auth-forms";
import { authEndpoint, authTestUser, rootAuth } from "./helpers";
import {
  action,
  activeDialog,
  field,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let original: Models.Config;
let userIds: string[];
beforeEach(async () => {
  await signInConsoleUser();
  original = await rootAuth.cp.config.get();
  userIds = [];
});
afterEach(async () => {
  wrapper?.unmount();
  for (const id of userIds) await rootAuth.cp.users.delete(id);
  await rootAuth.cp.config.update(authConfigInput(original));
  await useNuxtApp().$router.push("/");
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
function row(identity: string) {
  return pageDOM()
    .findAll("tbody tr")
    .find((item) => item.text().includes(identity))!;
}
async function createUser() {
  const input = authTestUser();
  const user = await rootAuth.cp.users.create(input);
  userIds.push(user.id);
  return { user, input };
}
function publicService() {
  return useAuthService(
    createClient({
      endpoint: authEndpoint,
      publishableKey: useNuxtApp().$manifest.state.value.publishableKey,
    }),
  );
}

describe("Auth console behavior against actual backend flows", () => {
  it("uses clear, scope-aware navigation with working routes", async () => {
    ({ wrapper } = await mountPage(Layout));
    const labels = pageDOM()
      .findAll('nav[aria-label="Auth Service Settings"] a')
      .map((link) => link.text());
    expect(labels).toEqual([
      "Overview",
      "Users",
      "Sessions",
      "Identity Providers",
      "Applications",
      "APIs",
      "Sign-In Methods",
      "Security",
      "Hosted Sign-In",
      "Email",
      "SMS",
      "Session Settings",
    ]);
    for (const item of wrapper.setupState.items.value) {
      expect(useNuxtApp().$router.resolve(item.to).matched.length).toBeGreaterThan(0);
    }
    wrapper.unmount();
    await signInConsoleUser(["auth:user.read"]);
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.items.value.map((item: { label: string }) => item.label)).toEqual([
      "Users",
      "Sessions",
    ]);
  });

  it("provisions users without sending verification and manages lock, unlock, details, and deletion", async () => {
    ({ wrapper } = await mountPage(Users));
    await action("New User").trigger("click");
    const input = authTestUser();
    await field("Email").setValue(input.email);
    await field("Initial Password").setValue(input.password);
    await field("Properties (JSON)").setValue('{"tier":"pro"}');
    await pageDOM().get("#auth-user-create").trigger("submit");
    await settle();
    expect(wrapper.setupState.authControlPlane.error.value).toBe("");
    const user = (await rootAuth.cp.users.list({ search: input.email })).data![0]!;
    userIds.push(user.id);
    expect(user.emailVerifiedAt).toBeUndefined();
    expect(user.properties).toEqual({ tier: "pro" });
    expect((await rootAuth.cp.users.sessions.list(user.id)).data).toEqual([]);
    await row(input.email).trigger("click");
    await settle();
    expect(activeDialog()?.textContent).toContain(input.email);
    expect(activeDialog()?.textContent).toContain("Not Verified");
    await action("Close", activeDialog()!).trigger("click");
    await menuAction("Lock User", row(input.email).element);
    await action("Lock User", activeDialog()!).trigger("click");
    await settle();
    expect((await rootAuth.cp.users.get(user.id)).lockedUntil!.getFullYear()).toBeGreaterThan(
      new Date().getFullYear() + 90,
    );
    expect(row(input.email).text()).toContain("Locked");
    await menuAction("Unlock User", row(input.email).element);
    await action("Unlock User", activeDialog()!).trigger("click");
    await settle();
    expect((await rootAuth.cp.users.get(user.id)).lockedUntil).toBeUndefined();
    await menuAction("Delete User", row(input.email).element);
    await action("Delete User", activeDialog()!).trigger("click");
    await settle();
    expect((await rootAuth.cp.users.list({ search: input.email })).data).toEqual([]);
    userIds = [];
  });

  it("loads real user sessions from navigation, responds to user changes, hides hashes, and revokes all sessions", async () => {
    const first = await createUser();
    const second = await createUser();
    const authService = publicService();
    await authService.sign_in({
      email: first.input.email,
      phone: "",
      password: first.input.password,
    });
    await authService.sign_in({
      email: second.input.email,
      phone: "",
      password: second.input.password,
    });
    await useNuxtApp().$router.push({ path: "/auth/sessions", query: { user_id: first.user.id } });
    ({ wrapper } = await mountPage(Sessions));
    expect(pageDOM().text()).toContain(first.input.email);
    const firstSession = (await rootAuth.cp.users.sessions.list(first.user.id)).data![0]!;
    expect(pageDOM().text()).not.toContain(firstSession.refreshTokenHash);
    await useNuxtApp().$router.push({ path: "/auth/sessions", query: { user_id: second.user.id } });
    await settle();
    expect(wrapper.setupState.authControlPlane.user.value.id).toBe(second.user.id);
    expect(pageDOM().text()).toContain(second.input.email);
    const session = (await rootAuth.cp.users.sessions.list(second.user.id)).data![0]!;
    await row(session.id).trigger("click");
    expect(activeDialog()?.textContent).toContain(session.id);
    expect(activeDialog()?.textContent).not.toContain(session.refreshTokenHash);
    await action("Close", activeDialog()!).trigger("click");
    await action("Revoke All Sessions").trigger("click");
    await action("Revoke Sessions", activeDialog()!).trigger("click");
    await settle();
    expect((await rootAuth.cp.users.sessions.list(second.user.id)).data).toEqual([]);
    expect((await rootAuth.cp.users.sessions.list(first.user.id)).data).toHaveLength(1);
    expect(pageDOM().text()).toContain("No Session Records");
  });

  it("allows editing backend-enforced password, session, and lockout policies", async () => {
    for (const [page, labels] of [
      [Configuration, ["Session Idle Timeout (Seconds)", "Single-Session Policy"]],
      [SignInMethods, ["Require Numbers", "Require Symbols"]],
      [Security, ["Lockout Duration (Seconds)"]],
    ] as const) {
      wrapper?.unmount();
      ({ wrapper } = await mountPage(page));
      for (const label of labels) expect(field(label).attributes("disabled")).toBeUndefined();
      expect(pageDOM().text()).not.toContain("Stored configuration only");
    }
  });

  it("reflects automatic hosted display changes and prevents enabling unavailable hosted methods", async () => {
    const config = authConfigInput(original);
    config.passkeys.enabled = false;
    config.oidc.ui.showPasskeys = false;
    await rootAuth.cp.config.update(config);
    ({ wrapper } = await mountPage(HostedSignIn));
    expect(field("Show Passkeys").attributes("disabled")).toBeDefined();
    wrapper.unmount();
    ({ wrapper } = await mountPage(SignInMethods));
    await field("Enable Passkeys").trigger("click");
    await pageDOM().get("#auth-configuration").trigger("submit");
    await settle();
    const saved = await rootAuth.cp.config.get();
    expect(saved.passkeys.enabled).toBe(true);
    expect(saved.oidc.ui.showPasskeys).toBe(true);
  });

  it("requires S256 PKCE for both public and confidential hosted applications", async () => {
    const config = authConfigInput(original);
    config.oidc.enabled = true;
    const clients = [true, false].map((isPublic) => ({
      name: "Console Flow Test",
      clientId: crypto.randomUUID(),
      _public: isPublic,
      clientSecret: isPublic ? "" : "confidential-secret",
      redirectUris: ["https://app.example.com/callback"],
    }));
    config.oidc.clients = [...(config.oidc.clients || []), ...clients];
    await rootAuth.cp.config.update(config);
    for (const client of clients) {
      const authorizeUrl = new URL("/v1/oauth/authorize", authEndpoint);
      authorizeUrl.search = new URLSearchParams({
        client_id: client.clientId,
        redirect_uri: client.redirectUris[0]!,
        response_type: "code",
        scope: "openid",
      }).toString();
      const browserRequest = await fetch(authorizeUrl, { redirect: "manual", credentials: "omit" });
      expect(browserRequest.status).toBe(302);
      expect(browserRequest.headers.get("location")).toContain("PKCE");
      const headers = { "Publishable-Key": useNuxtApp().$manifest.state.value.publishableKey! };
      const rejected = await fetch(authorizeUrl, {
        redirect: "manual",
        credentials: "omit",
        headers,
      });
      expect(rejected.status).toBe(302);
      expect(rejected.headers.get("location")).toContain("PKCE");
      authorizeUrl.searchParams.set("code_challenge", "a".repeat(43));
      authorizeUrl.searchParams.set("code_challenge_method", "S256");
      const accepted = await fetch(authorizeUrl, {
        redirect: "manual",
        credentials: "omit",
        headers,
      });
      expect(accepted.status).toBe(200);
      expect(accepted.headers.get("content-type")).toContain("text/html");
    }
  });
});
