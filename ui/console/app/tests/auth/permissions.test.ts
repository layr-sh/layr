import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { AuthConfigurationEditor, AuthRecords, UModal, USelect, UInputNumber } from "#components";
import Layout from "../../pages/auth.vue";
import Overview from "../../pages/auth/index.vue";
import Users from "../../pages/auth/users.vue";
import Sessions from "../../pages/auth/sessions.vue";
import Applications from "../../pages/auth/applications.vue";
import APIs from "../../pages/auth/resource-servers.vue";
import Providers from "../../pages/auth/providers.vue";
import Configuration from "../../pages/auth/configurations.vue";
import SignInMethods from "../../pages/auth/sign-in-methods.vue";
import Security from "../../pages/auth/security.vue";
import HostedSignIn from "../../pages/auth/hosted-sign-in.vue";
import Email from "../../pages/auth/email.vue";
import SMS from "../../pages/auth/sms.vue";

import { authTestUser, rootAuth } from "./helpers";
import {
  action,
  field,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
let userIds: string[];
beforeEach(async () => {
  consoleUser = await signInConsoleUser();
  userIds = [];
});
afterEach(async () => {
  wrapper?.unmount();
  for (const id of userIds) await rootAuth.cp.users.delete(id);
  await useNuxtApp().$router.push("/");
  vi.restoreAllMocks();
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
function childState(component: typeof AuthConfigurationEditor | typeof AuthRecords) {
  return wrapper.findComponent(component).vm as unknown as typeof wrapper.setupState;
}
function modal(title: string) {
  return wrapper.findAllComponents(UModal).find((item) => item.props("title") === title)!;
}
async function createUser() {
  const user = await rootAuth.cp.users.create(authTestUser());
  userIds.push(user.id);
  return user;
}

describe("Auth scope boundaries and recoverable UI states", () => {
  it("does not load data without read scopes and restricts navigation", async () => {
    await signInConsoleUser([]);
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.items.value).toEqual([]);
    expect(pageDOM().text()).toContain("Access Restricted");
    for (const page of [
      Overview,
      Users,
      Sessions,
      Applications,
      APIs,
      Providers,
      Configuration,
      SignInMethods,
      Security,
      HostedSignIn,
      Email,
      SMS,
    ]) {
      wrapper.unmount();
      ({ wrapper } = await mountPage(page));
      const control = wrapper.setupState.authControlPlane;
      expect(control.users.value).toBeNull();
      expect(control.config.value).toBeNull();
      expect(control.busy.value).toBe(false);
      expect(control.error.value).toBe("");
    }
    wrapper.unmount();
    await signInConsoleUser(["auth:config.read"]);
    await useNuxtApp().$router.push("/auth/users");
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.canView.value).toBe(false);
    await useNuxtApp().$router.push("/auth/providers");
    expect(wrapper.setupState.canView.value).toBe(true);
  });

  it("renders directory-wide statistics independently of recent users and respects read scopes", async () => {
    const user = await createUser();
    ({ wrapper } = await mountPage(Overview));
    const state = wrapper.setupState;
    const control = state.authControlPlane;
    expect(pageDOM().text()).toContain("Directory-wide application user totals");
    expect(pageDOM().text()).not.toContain("Connect Your First Application");
    expect(pageDOM().text()).toContain(user.email);
    const snapshot = control.users.value;
    const statistics = control.stats.value;
    expect(statistics.totalUsers).toBeGreaterThanOrEqual(1);
    const config = control.config.value;
    control.users.value.data = [
      { ...user, isAnonymous: true, mfaEnabled: true, lockedUntil: new Date(Date.now() + 60000) },
      { ...user, id: crypto.randomUUID(), email: "second@example.com", lockedUntil: undefined },
    ];
    control.stats.value = {
      ...statistics,
      totalUsers: 42,
      anonymousUsers: 6,
      newUsersLast24Hours: 3,
      lockedUsers: 2,
      mfaUsers: 4,
      activeSessions: 8,
      activeSessionUsers: 5,
      passkeyUsers: 7,
      registeredUsers: 36,
      newUsersPrevious24Hours: 2,
      newUsersLast7Days: 15,
      emailUnverifiedUsers: 10,
      phoneUnverifiedUsers: 3,
      usersWithoutCredentials: 1,
      expiredSessions: 4,
      lockedSessions: 2,
      idleSessions: 3,
      providers: [
        {
          provider: "google",
          userCount: 5,
          identityCount: 5,
          lastSignedInAt: new Date("2026-01-01T12:00:00Z"),
        },
        { provider: "custom", userCount: 1, identityCount: 1, lastSignedInAt: undefined },
      ],
      roles: [
        { role: "authenticated", totalUsers: 41 },
        { role: "admin", totalUsers: 1 },
      ],
      registrations: [
        {
          day: new Date("2026-01-01T00:00:00Z"),
          totalUsers: 0,
          registeredUsers: 0,
          anonymousUsers: 0,
        },
        {
          day: new Date("2026-01-02T00:00:00Z"),
          totalUsers: 3,
          registeredUsers: 2,
          anonymousUsers: 1,
        },
      ],
    };
    config.oauthProviders = {
      google: {
        preset: "google",
        enabled: true,
        clientId: "google-client",
        clientSecretConfigured: true,
      },
      github: {
        preset: "github",
        enabled: false,
        clientId: "github-client",
        clientSecretConfigured: true,
      },
    };
    config.oidc.enabled = true;
    for (const method of ["password", "passkeys", "emailOtp", "smsOtp", "anonymous"])
      config[method].enabled = true;
    await nextTick();
    expect(state.summary.value.map((item: { value: number }) => item.value)).toEqual([
      42, 6, 3, 2, 4, 8, 5, 7,
    ]);
    expect(pageDOM().text()).toContain("Locked");
    expect(pageDOM().text()).toContain("Anonymous Sign-In");
    expect(pageDOM().text()).toContain("External Providers");
    expect(pageDOM().text()).toContain("Google");
    expect(pageDOM().text()).toContain("GitHub");
    expect(pageDOM().text()).toContain("Hosted OIDC");
    config.oauthProviders = {};
    await nextTick();
    expect(pageDOM().text()).toContain("No external identity providers configured");
    expect(pageDOM().text()).toContain("Account Health");
    expect(pageDOM().text()).toContain("Without Stored Credentials");
    expect(pageDOM().text()).toContain("Authentication Credentials");
    expect(pageDOM().text()).toContain("Stored Sessions");
    expect(pageDOM().text()).toContain("Expired");
    expect(pageDOM().text()).toContain("Idle Timeout");
    expect(pageDOM().text()).toContain("google");
    expect(pageDOM().text()).toContain("Not recorded");
    expect(pageDOM().text()).toContain("admin");
    expect(pageDOM().text()).toContain("2026-01-02");
    expect(pageDOM().text()).toContain("Deleted accounts are not reconstructed");
    expect(state.registrationPeak.value).toBe(3);
    control.stats.value = {
      ...control.stats.value,
      registrations: null,
      providers: null,
      roles: null,
    };
    await nextTick();
    expect(state.registrationPeak.value).toBe(1);
    expect(pageDOM().text()).toContain("No External Identities");
    expect(pageDOM().text()).toContain("No User Roles");
    control.users.value = null;
    control.stats.value = null;
    control.config.value = null;
    await nextTick();
    expect(state.methods.value).toEqual([]);
    expect(state.providers.value).toEqual([]);
    expect(state.summary.value.every((item: { value: number }) => item.value === 0)).toBe(true);
    for (const items of [state.accountHealth, state.credentials, state.sessionStates])
      expect(items.value.every((item: { value: number }) => item.value === 0)).toBe(true);
    expect(state.registrations.value).toEqual([]);
    expect(pageDOM().text()).toContain("Statistics Unavailable");
    expect(pageDOM().text()).toContain("Configuration Unavailable");
    control.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Statistics");
    control.busy.value = false;
    control.error.value = "Snapshot failed";
    control.users.value = { ...snapshot, data: null };
    await nextTick();
    expect(pageDOM().text()).toContain("Snapshot failed");
    await action("Refresh").trigger("click");
    await settle();
    expect(control.error.value).toBe("");
    await signInConsoleUser(["auth:config.read"]);
    expect(await state.load()).toBeUndefined();
    await signInConsoleUser(["auth:user.read"]);
    await state.load();
    await nextTick();
    expect(pageDOM().text()).not.toContain("Sign-In Configuration");
    await signInConsoleUser([]);
    await state.load();
    await signInConsoleUser(["auth:config.read"]);
    vi.spyOn(control, "listUsers").mockResolvedValueOnce(false);
    await signInConsoleUser();
    await state.load();
    vi.spyOn(control, "loadStats").mockResolvedValueOnce(false);
    await state.load();
    await signInConsoleUser(["auth:stats.read"]);
    await state.load();
    await nextTick();
    expect(pageDOM().text()).toContain("Authentication Statistics");
    expect(pageDOM().text()).not.toContain("Recent Users");
  });

  it("keeps configuration read-only and handles missing, invalid, and failed settings", async () => {
    await signInConsoleUser(["auth:config.read"]);
    ({ wrapper } = await mountPage(Configuration));
    const control = wrapper.setupState.authControlPlane;
    expect(field("Access Token Lifetime (Seconds)").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save Session Settings");
    expect(await control.saveConfig(control.config.value)).toBe(false);
    await nextTick();
    expect(pageDOM().text()).toContain("Request Failed");
    control.config.value = null;
    await nextTick();
    const editor = childState(AuthConfigurationEditor);
    expect(editor.validate()).toEqual([]);
    await editor.save();
    control.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Auth Configuration");
    control.busy.value = false;
    await nextTick();
    await action("Reload").trigger("click");
    await settle();
    expect(control.config.value).not.toBeNull();
    wrapper.unmount();
    await signInConsoleUser();
    ({ wrapper } = await mountPage(Configuration));
    const nextControl = wrapper.setupState.authControlPlane;
    nextControl.config.value.sessions.accessTokenExpirySeconds = 0;
    await childState(AuthConfigurationEditor).save();
    expect(childState(AuthConfigurationEditor).validate()).toHaveLength(1);
    wrapper.findComponent(UInputNumber).vm.$emit("update:modelValue", 901);
    await nextTick();
    expect(nextControl.config.value.sessions.accessTokenExpirySeconds).toBe(901);
  });

  it("supports optional delivery settings, all input callbacks, and blank select values", async () => {
    ({ wrapper } = await mountPage(Email));
    const control = wrapper.setupState.authControlPlane;
    const select = wrapper.findComponent(USelect);
    select.vm.$emit("update:modelValue", "smtp");
    await nextTick();
    expect(control.config.value.emailDispatcher.driver).toBe("smtp");
    select.vm.$emit("update:modelValue", "__unconfigured");
    await nextTick();
    expect(control.config.value.emailDispatcher.driver).toBe("");
    const template = control.config.value.emailDispatcher.templates.passwordReset;
    template.html = template.text = null;
    await nextTick();
    wrapper.findComponent({ name: "UTextarea" }).vm.$emit("update:modelValue", "<p>Updated</p>");
    await nextTick();
    expect(template.html).toBe("<p>Updated</p>");
    const editor = childState(AuthConfigurationEditor);
    editor.selectedTemplate = "not-a-message";
    await nextTick();
    expect(pageDOM().text()).not.toContain("Supported Placeholders");
    control.config.value = null;
    await nextTick();
  });

  it("handles busy guards, non-Error failures, denied requests, and expired console sessions", async () => {
    ({ wrapper } = await mountPage(Users));
    const control = wrapper.setupState.authControlPlane;
    control.busy.value = true;
    expect(await control.run(async () => {})).toBe(false);
    control.busy.value = false;
    expect(
      await control.run(async () => {
        throw "Rejected";
      }),
    ).toBe(false);
    expect(control.error.value).toBe("The request failed. Please try again.");
    expect(
      await control.run(async () => {
        throw new Error("Connection failed");
      }),
    ).toBe(false);
    expect(control.error.value).toBe("Connection failed");
    await signInConsoleUser(["auth:user.read"]);
    expect(await control.loadConfig()).toBe(false);
    expect(useNuxtApp().$auth.state.value).not.toBeNull();
    const current = useNuxtApp().$auth.state.value!;
    await consoleUser.serviceAccount.console.users.sessions.delete(current.user.id, current.id);
    expect(await control.listUsers()).toBe(false);
    expect(useNuxtApp().$auth.state.value).toBeNull();
    expect(useNuxtApp().$router.currentRoute.value.path).toBe("/sign-in");
  });

  it("keeps record pages safe with unavailable configuration and read-only credentials", async () => {
    await signInConsoleUser(["auth:config.read"]);
    ({ wrapper } = await mountPage(Providers));
    const control = wrapper.setupState.authControlPlane;
    const row = pageDOM()
      .findAll("tbody tr")
      .find((row) => row.text().includes("Google"))!;
    await menuAction("Details", row.element);
    expect(field("OAuth Client ID").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save Identity Provider");
    modal("Google Settings").vm.$emit("update:open", false);
    await nextTick();
    const records = childState(AuthRecords);
    control.config.value = null;
    expect(records.validate()).toEqual([]);
    await records.save();
    await records.remove();
    await nextTick();
    expect(pageDOM().text()).toContain("Configuration is unavailable");
    control.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Auth Configuration");
    control.busy.value = false;
    await action("Refresh").trigger("click");
    await settle();
    records.form = { key: "unknown-provider" };
    records.editing = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Custom OAuth Provider Settings");
    records.editing = false;
    records.form = {};
    records.editing = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Custom OAuth Provider Settings");
    records.editing = false;
  });
});
