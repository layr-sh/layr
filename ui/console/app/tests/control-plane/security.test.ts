import { createHmac } from "node:crypto";
import { readFileSync } from "node:fs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UModal, UPinInput } from "#components";

import useAuth from "../../composables/use-auth";
import useControlPlane from "../../composables/use-control-plane";
import Layout from "../../pages/control-plane.vue";
import Security from "../../pages/control-plane/account-security.vue";
import Settings from "../../pages/control-plane/index.vue";

import Users from "../../pages/control-plane/users.vue";
import Accounts from "../../pages/control-plane/service-accounts.vue";
import Hooks from "../../pages/control-plane/event-hooks.vue";
import { action, flushPromises, mountPage, settle, signInConsoleUser } from "./mount-page";

type Wrapper = Awaited<ReturnType<typeof mountPage>>["wrapper"];
let wrapper: Wrapper;
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
beforeEach(async () => {
  consoleUser = await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());

function verificationCode(secret: string) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...secret]
    .map((letter) => alphabet.indexOf(letter).toString(2).padStart(5, "0"))
    .join("");
  const bytes = Buffer.from(
    Array.from({ length: Math.floor(bits.length / 8) }, (_, index) =>
      parseInt(bits.slice(index * 8, index * 8 + 8), 2),
    ),
  );
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac("sha1", bytes).update(counter).digest();
  const offset = digest[digest.length - 1]! & 15;
  return String((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, "0");
}

describe("Console security and permissions", () => {
  it("matches the backend scope rules for navigation and write actions", async () => {
    const auth = useAuth();
    const original = auth.state.value!;
    for (const [scopes, required, expected] of [
      [[], "console:user.read", false],
      [["console:user.read"], "", true],
      [["*"], "console:user.write", true],
      [["console:user.read"], "console:user.read", true],
      [["console:*"], "console:user.write", true],
      [["console:user.write"], "console:user.read", true],
      [["console:*.read"], "console:user.read", true],
      [["*:user.read"], "console:user.read", true],
      [["console:user"], "console:user.write", true],
      [["console:user.read"], "console:user.write", false],
      [["core:event.write"], "console:user.read", false],
      [["console:config.read"], "console:user.read", false],
      [["console"], "console:user.write", true],
      [["console:user.read"], "console", false],
      [[" core:event.read "], "core:event.read", true],
      [["console:user.read"], "console:user", false],
    ] as const) {
      auth.state.value = { ...original, scopes: [...scopes] };
      expect(auth.hasScope(required)).toBe(expected);
    }
    auth.state.value = null;
    expect(auth.hasScope("console:user.read")).toBe(false);
    auth.state.value = original;
    const limited = await signInConsoleUser(["console:config.read"]);
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.text()).not.toContain("Audit Events");
    expect(wrapper.text()).toContain("Account Security");
    wrapper.unmount();
    ({ wrapper } = await mountPage(Settings));
    expect(
      wrapper.findAll("button").some((button) => button.text() === "Save Console Config"),
    ).toBe(false);
    const client = limited.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(await controlPlane.load("service-accounts")).toBe(false);
    expect(limited.nuxtApp.$auth.state.value?.user.id).toBe(limited.user.id);
  });

  it("checks direct section navigation against the current permissions", async () => {
    const router = consoleUser.nuxtApp.$router;
    for (const path of ["/control-plane", "/control-plane/account-security"]) {
      await router.push(path);
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(true);
      wrapper.unmount();
    }
    await signInConsoleUser(["console:user.read"]);
    await router.push("/control-plane/events");
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.text()).toContain("Access Restricted");
    wrapper.unmount();
    await router.push("/");
  });

  it("shows Settings for Core-only access and hides inaccessible sections", async () => {
    const router = consoleUser.nuxtApp.$router;
    await signInConsoleUser(["core:config.read"]);
    await router.push("/control-plane");
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.canView.value).toBe(true);
    expect(wrapper.text()).toContain("Configuration");
    wrapper.unmount();
    await signInConsoleUser(["console:user.read"]);
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.canView.value).toBe(false);
    expect(wrapper.text()).toContain("Access Restricted");
    wrapper.unmount();
    await router.push("/");
  });

  it("keeps scope options available when authentication expires and accounts have no scopes", async () => {
    const page = await mountPage(Accounts);
    wrapper = page.wrapper;
    const original = consoleUser.nuxtApp.$auth.state.value;
    page.controlPlane.accounts.value = [{ ...page.controlPlane.accounts.value[0]!, scopes: null }];
    consoleUser.nuxtApp.$auth.state.value = null;
    await nextTick();
    expect(wrapper.setupState.scopeOptions.value).toEqual(
      [...readFileSync("../../core/scopes.go", "utf8").matchAll(/Scope\w+\s*=\s*"([^"]+)"/g)]
        .map((match) => match[1])
        .sort(),
    );
    consoleUser.nuxtApp.$auth.state.value = original;
  });

  it("loads and creates users without requiring Service Account visibility", async () => {
    const limited = await signInConsoleUser(["console:user.write"]);
    const client = limited.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(await controlPlane.load("users")).toBe(true);
    expect(controlPlane.accounts.value).toEqual([]);
    const email = `scoped-${crypto.randomUUID()}@example.com`;
    expect(await controlPlane.createUser({ email, password: limited.password })).toBe(true);
    const user = controlPlane.users.value.find((user) => user.email === email)!;
    ({ wrapper } = await mountPage(Users));
    await action("Manage").trigger("click");
    await settle(wrapper.setupState.controlPlane);
    expect(wrapper.text()).toContain("Not Visible");
    await limited.serviceAccount.console.users.delete(user.id);
  });

  it("clears expired authentication after a successful key-authorized account update", async () => {
    const state = consoleUser.nuxtApp.$auth.state.value!;
    const serviceAccount = consoleUser.serviceAccount;
    const account = (await serviceAccount.core.service_accounts.list()).find(
      (account) => account.consoleUserId === consoleUser.user.id,
    )!;
    await serviceAccount.console.users.sessions.delete(consoleUser.user.id, state.id);
    const controlPlane = useControlPlane(serviceAccount.core, serviceAccount.console);
    expect(await controlPlane.updateAccount(account.id, { description: "Updated" })).toBe(true);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
  });

  it("revokes all sessions belonging to the current user", async () => {
    const client = consoleUser.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(await controlPlane.revokeSession(consoleUser.user.id)).toBe(true);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
  });

  it("allows reading without exposing mutation controls", async () => {
    const limited = await signInConsoleUser([
      "console:user.read",
      "core:service-account.read",
      "core:event-hook.read",
      "core:config.read",
    ]);
    for (const page of [Users, Accounts, Hooks, Settings]) {
      wrapper?.unmount();
      ({ wrapper } = await mountPage(page));
      expect(
        wrapper
          .findAll("button")
          .some((button) =>
            [
              "Create User",
              "Create Account",
              "Create Hook",
              "Save User",
              "Save Account",
              "Save Hook",
              "Save Platform Config",
              "Save Console Config",
            ].includes(button.text()),
          ),
      ).toBe(false);
    }
    wrapper.unmount();
    ({ wrapper } = await mountPage(Users));
    await wrapper.setupState.editUser(limited.user.id);
    expect(wrapper.find("#console-password-form").exists()).toBe(false);
    expect(wrapper.text()).toContain("Active Sessions");
    expect(limited.nuxtApp.$auth.state.value?.scopes).not.toContain("*");
  });

  it("opens linked records and displays last-root protection", async () => {
    const serviceAccount = consoleUser.serviceAccount;
    const other = await serviceAccount.console.users.create({
      email: `root-${crypto.randomUUID()}@example.com`,
      password: consoleUser.password,
    });
    const account = (await serviceAccount.core.service_accounts.list()).find(
      (account) => account.consoleUserId === other.id,
    )!;
    await serviceAccount.core.service_accounts.update(account.id, { scopes: ["*"] });
    const router = consoleUser.nuxtApp.$router;
    await router.push({ path: "/control-plane/users", query: { user: other.id } });
    const page = await mountPage(Users);
    wrapper = page.wrapper;
    expect(page.controlPlane.selectedUser.value?.id).toBe(other.id);
    page.controlPlane.accounts.value = [{ ...account, scopes: ["*"] }];
    page.controlPlane.users.value = [other, { ...consoleUser.user, isEnabled: false }];
    await nextTick();
    expect(wrapper.text()).toContain("The last enabled root Console User must retain access.");
    page.controlPlane.users.value = [other, consoleUser.user];
    await nextTick();
    expect(wrapper.setupState.isLastRoot.value).toBe(true);
    const ownAccount = (await serviceAccount.core.service_accounts.list()).find(
      (account) => account.consoleUserId === consoleUser.user.id,
    )!;
    page.controlPlane.accounts.value.push(ownAccount);
    await nextTick();
    expect(wrapper.setupState.isLastRoot.value).toBe(false);
    page.controlPlane.accounts.value = [
      { ...account, scopes: ["*"], expiresAt: new Date(Date.now() + 60000) },
    ];
    await nextTick();
    expect(wrapper.setupState.isLastRoot.value).toBe(true);
    page.controlPlane.accounts.value = [{ ...account, scopes: ["*"], expiresAt: new Date(0) }];
    await nextTick();
    expect(wrapper.setupState.isLastRoot.value).toBe(false);
    wrapper.unmount();
    await router.push({ path: "/control-plane/service-accounts", query: { account: account.id } });
    ({ wrapper } = await mountPage(Accounts));
    expect(wrapper.setupState.controlPlane.selectedAccount.value?.id).toBe(account.id);
    wrapper.unmount();
    await router.push("/");
    await serviceAccount.console.users.delete(other.id);
  });

  it("displays selected accounts and hooks without write controls", async () => {
    const account = await consoleUser.serviceAccount.core.service_accounts.create({
      name: "Read Only Disabled Account",
      scopes: ["core:event.read"],
    });
    await consoleUser.serviceAccount.core.service_accounts.update(account.id, { isEnabled: false });
    const hook = await consoleUser.serviceAccount.core.event_hooks.create({
      name: "Read Only Hook",
      driver: "http",
      eventTypes: ["console.user.created"],
      httpTargetUrl: `${process.env.LAYR__SERVER__BASE_URL}/readyz`,
    });
    const limited = await signInConsoleUser(["core:service-account.read", "core:event-hook.read"]);
    const page = await mountPage(Accounts);
    wrapper = page.wrapper;
    const owned = page.controlPlane.accounts.value.find(
      (account) => account.consoleUserId === limited.user.id,
    )!;
    await wrapper.setupState.editAccount(owned.id);
    expect(wrapper.findAll("a").some((link) => link.text() === "View Console User")).toBe(false);
    await wrapper.setupState.editAccount(account.id);
    expect(wrapper.text()).toContain("Disabled");
    expect(wrapper.findAll("button").some((button) => button.text() === "Save Account")).toBe(
      false,
    );
    wrapper.unmount();
    ({ wrapper } = await mountPage(Hooks));
    await wrapper.setupState.editHook(String(hook.id));
    expect(wrapper.findAll("button").some((button) => button.text() === "Save Hook")).toBe(false);
    await consoleUser.serviceAccount.core.event_hooks.delete(String(hook.id));
    await consoleUser.serviceAccount.core.service_accounts.delete(account.id);
  });

  it("enforces MFA across services and keeps enrollment available", async () => {
    const original = await consoleUser.serviceAccount.console.config.get();
    const client = consoleUser.nuxtApp.$client.value;
    const oldToken = client.sessionToken!;
    expect(
      (await consoleUser.nuxtApp.$auth.signIn(consoleUser.user.email, consoleUser.password)).error,
    ).toBeNull();
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    try {
      expect(await controlPlane.saveConfig({ ...original, mfaEnforced: true })).toBe(true);
      expect(consoleUser.nuxtApp.$router.currentRoute.value.path).toBe(
        "/control-plane/account-security",
      );
      expect(consoleUser.nuxtApp.$auth.state.value?.mfa_required).toBe(true);
      const page = await mountPage(Security);
      wrapper = page.wrapper;
      expect(wrapper.text()).toContain("Enroll your authenticator below");
      expect(await controlPlane.load("platform")).toBe(false);
      await action("Set Up MFA").trigger("click");
      await settle(page.controlPlane);
      const secret = wrapper.setupState.enrollment.value.secret;
      const code = verificationCode(secret);
      wrapper.findComponent(UPinInput).vm.$emit("update:modelValue", [...code].map(Number));
      await nextTick();
      await wrapper.get("#mfa-enrollment").trigger("submit");
      await settle(page.controlPlane);
      expect(consoleUser.nuxtApp.$auth.state.value?.mfa_required).toBe(false);
      expect(await controlPlane.load("platform")).toBe(true);
      await wrapper.setupState.disableMFA();
      expect(wrapper.text()).toContain("MFA is required by the Console policy");
      client.setSession(oldToken);
      await consoleUser.nuxtApp.$auth.refresh();
      await nextTick();
      expect(consoleUser.nuxtApp.$auth.state.value?.mfa_required).toBe(true);
      expect(wrapper.text()).toContain("Sign out and sign in again with your authenticator code");
      expect(await controlPlane.load("platform")).toBe(false);
      const challenge = await consoleUser.nuxtApp.$auth.signIn(
        consoleUser.user.email,
        consoleUser.password,
      );
      expect(challenge.response?.mfa_required).toBe(true);
      expect(
        (await consoleUser.nuxtApp.$auth.verifyMFA(verificationCode(secret))).error,
      ).toBeNull();
      expect(await controlPlane.load("platform")).toBe(true);
    } finally {
      await consoleUser.serviceAccount.console.config.update(original);
    }
  });

  it("enrolls and disables MFA using actual authenticator codes", async () => {
    const page = await mountPage(Security);
    wrapper = page.wrapper;
    await wrapper.setupState.enableMFA();
    expect(wrapper.text()).toContain("Request Failed");
    await action("Set Up MFA").trigger("click");
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Setup Key");
    await action("Cancel Enrollment").trigger("click");
    await action("Set Up MFA").trigger("click");
    await settle(page.controlPlane);
    const secret = wrapper.setupState.enrollment.value.secret;
    const code = verificationCode(secret);
    const invalidCode = String((Number(code) + 1) % 1000000).padStart(6, "0");
    wrapper.findComponent(UPinInput).vm.$emit("update:modelValue", [...invalidCode].map(Number));
    await nextTick();
    await wrapper.get("#mfa-enrollment").trigger("submit");
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Invalid MFA code");
    wrapper.findComponent(UPinInput).vm.$emit("update:modelValue", [...code].map(Number));
    await nextTick();
    await wrapper.get("#mfa-enrollment").trigger("submit");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.console.users.get(consoleUser.user.id)).mfaEnabled,
    ).toBe(true);
    expect(wrapper.text()).toContain("Multi-Factor Authentication Enabled");
    await action("Disable MFA").trigger("click");
    await flushPromises();
    wrapper.findComponent(UModal).vm.$emit("update:open", false);
    await flushPromises();
    await action("Disable MFA").trigger("click");
    await flushPromises();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await action("Disable MFA").trigger("click");
    await flushPromises();
    await action("Disable MFA", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.console.users.get(consoleUser.user.id)).mfaEnabled,
    ).toBe(false);
    await action("Sign Out This Session").trigger("click");
    await settle(page.controlPlane);
    expect(useNuxtApp().$auth.state.value).toBeNull();
  });

  it("preserves authentication when the sign-out transport fails", async () => {
    const page = await mountPage(Security);
    wrapper = page.wrapper;
    const fetch = globalThis.fetch;
    const transport = vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
      const url = input instanceof Request ? input.url : String(input);
      if (url.includes("/console/sign-out")) return Promise.reject(new Error("Connection lost"));
      return fetch(input, init);
    });
    try {
      await action("Sign Out This Session").trigger("click");
      await settle(page.controlPlane);
      expect(wrapper.text()).toContain("Request Failed");
      expect(consoleUser.nuxtApp.$auth.state.value?.user.id).toBe(consoleUser.user.id);
    } finally {
      transport.mockRestore();
    }
  });

  it("handles rejected MFA changes after session revocation", async () => {
    const page = await mountPage(Security);
    wrapper = page.wrapper;
    const state = consoleUser.nuxtApp.$auth.state.value!;
    await consoleUser.serviceAccount.console.users.sessions.delete(consoleUser.user.id, state.id);
    await wrapper.setupState.disableMFA();
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Request Failed");
    await wrapper.setupState.setupMFA();
    await settle(page.controlPlane);
    expect(wrapper.setupState.enrollment.value).toBeNull();
  });

  it("handles a revoked session without treating the caller as still signed in", async () => {
    const state = consoleUser.nuxtApp.$auth.state.value!;
    await consoleUser.serviceAccount.console.users.sessions.delete(consoleUser.user.id, state.id);
    const client = consoleUser.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(await controlPlane.load("settings")).toBe(false);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
    expect(consoleUser.nuxtApp.$router.currentRoute.value.path).toBe("/sign-in");
  });

  it("revokes the current session without making another protected list request", async () => {
    const client = consoleUser.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    const state = consoleUser.nuxtApp.$auth.state.value!;
    expect(await controlPlane.revokeSession(consoleUser.user.id, state.id)).toBe(true);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
    expect(controlPlane.sessions.value).toEqual([]);
  });

  it("resets the current password and clears its now-revoked authentication", async () => {
    const client = consoleUser.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(await controlPlane.resetPassword(consoleUser.user.id, "Replacement-password-123!")).toBe(
      true,
    );
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
  });

  it("refreshes the current user's identity and Service Account permissions", async () => {
    const client = consoleUser.nuxtApp.$client.value;
    const controlPlane = useControlPlane(client.core.cp, client.console.cp);
    expect(
      await controlPlane.updateUser(consoleUser.user.id, { email: consoleUser.user.email }),
    ).toBe(true);
    const account = (await consoleUser.serviceAccount.core.service_accounts.list()).find(
      (account) => account.consoleUserId === consoleUser.user.id,
    )!;
    expect(await controlPlane.updateAccount(account.id, { scopes: ["console:config.read"] })).toBe(
      true,
    );
    expect(controlPlane.accounts.value).toEqual([]);
    expect(consoleUser.nuxtApp.$auth.state.value?.scopes).toEqual(["console:config.read"]);
  });
});
