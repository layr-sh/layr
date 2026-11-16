import { createServer, type Server } from "node:http";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { UModal } from "#components";
import Settings from "../../pages/control-plane/index.vue";

import Users from "../../pages/control-plane/users.vue";
import Accounts from "../../pages/control-plane/service-accounts.vue";
import Hooks from "../../pages/control-plane/event-hooks.vue";
import Events from "../../pages/control-plane/events.vue";
import {
  action,
  createOption,
  field,
  flushPromises,
  mountPage,
  selectOption,
  settle,
  signInConsoleUser,
} from "./mount-page";

type Wrapper = Awaited<ReturnType<typeof mountPage>>["wrapper"];
let wrapper: Wrapper;
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
let webhookServer: Server;
let webhookUrl: string;
beforeAll(async () => {
  consoleUser = await signInConsoleUser();
  webhookServer = createServer((request, response) => {
    request.resume();
    response.writeHead(request.url === "/fail" ? 503 : 200);
    response.end();
  });
  await new Promise<void>((resolve, reject) => {
    webhookServer.once("error", reject);
    webhookServer.listen(0, "127.0.0.1", resolve);
  });
  const address = webhookServer.address();
  if (!address || typeof address === "string") throw new Error("Expected a TCP webhook listener");
  webhookUrl = `http://127.0.0.1:${address.port}`;
});
afterEach(() => wrapper?.unmount());
afterAll(async () => {
  await new Promise<void>((resolve, reject) =>
    webhookServer.close((error) => (error ? reject(error) : resolve())),
  );
});

describe("Console failures and model rendering", () => {
  it("keeps settings recoverable after unavailable data or invalid input", async () => {
    const page = await mountPage(Settings);
    wrapper = page.wrapper;
    const original = await consoleUser.serviceAccount.console.config.get();
    await field("Require Multi-Factor Authentication").trigger("click");
    await field("Require Multi-Factor Authentication").trigger("click");
    expect(await page.controlPlane.saveConfig({ ...original, sessionIdleTimeoutSeconds: 0 })).toBe(
      false,
    );
    page.controlPlane.config.value = null;
    await nextTick();
    await wrapper.setupState.saveSettings();
    expect(wrapper.text()).toContain("Settings Unavailable");

    await consoleUser.serviceAccount.console.config.update(original);
  });

  it("manages platform policy through the Core API and handles unavailable settings", async () => {
    const page = await mountPage(Settings);
    wrapper = page.wrapper;
    const original = await consoleUser.serviceAccount.core.config.get();
    try {
      await field("Allowed IP Addresses and CIDRs").setValue("127.0.0.1, ::1\n");
      await field("Audit Retention (Days)").setValue("30");
      await wrapper.get("#platform-settings").trigger("submit");
      await settle(page.controlPlane);
      expect(wrapper.text()).toContain("Platform Settings Saved");
      expect(await consoleUser.serviceAccount.core.config.get()).toEqual({
        allowedCidrs: ["127.0.0.1", "::1"],
        auditRetentionDays: 30,
      });
      expect(await page.controlPlane.saveCoreConfig({ ...original, auditRetentionDays: 0 })).toBe(
        false,
      );
      await consoleUser.serviceAccount.core.config.update({ ...original, allowedCidrs: null });
      await wrapper.setupState.reload();
      await settle(page.controlPlane);
      expect(field("Allowed IP Addresses and CIDRs").element).toHaveProperty("value", "");
      page.controlPlane.coreConfig.value = null;
      await nextTick();
      await wrapper.setupState.savePlatformSettings();
      expect(wrapper.text()).toContain("Settings Unavailable");
      page.controlPlane.busy.value = true;
      await nextTick();
      expect(wrapper.text()).toContain("Loading Platform Settings");
      page.controlPlane.busy.value = false;
    } finally {
      await consoleUser.serviceAccount.core.config.update(original);
    }
  });

  it("preserves failed user forms and handles stale records", async () => {
    const page = await mountPage(Users);
    wrapper = page.wrapper;
    const setup = wrapper.setupState;
    await setup.saveUser();
    await setup.resetPassword();
    await setup.confirmAction();
    await field("Email").setValue(consoleUser.user.email);
    await field("Initial Password").setValue(consoleUser.password);
    await wrapper.get("#console-user-form").trigger("submit");
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Console user already exists");
    await setup.editUser(crypto.randomUUID());
    const email = `session-${crypto.randomUUID()}@example.com`;
    const user = await consoleUser.serviceAccount.console.users.create({
      email,
      password: consoleUser.password,
    });
    await fetch(`${process.env.LAYR__SERVER__BASE_URL}/v1/_/console/sign-in`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password: consoleUser.password }),
    });
    await setup.editUser(user.id);
    await field("New Password").setValue("short");
    await setup.resetPassword();
    expect(field("New Password").element).toHaveProperty("value", "short");
    await action("Revoke All Sessions").trigger("click");
    await flushPromises();
    await action("Confirm", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(page.controlPlane.sessions.value).toEqual([]);
    page.controlPlane.users.value = [{ ...user, mfaEnabled: true, lastSignedInAt: new Date() }];
    await nextTick();
    expect(wrapper.text()).toContain("Enrolled");
    await action("Delete User").trigger("click");
    await flushPromises();
    wrapper.findComponent(UModal).vm.$emit("update:open", true);
    await nextTick();
    await action("Close", document.querySelector('[role="dialog"]')!).trigger("click");
    await flushPromises();
    await action("Delete User").trigger("click");
    await flushPromises();
    await consoleUser.serviceAccount.console.users.delete(user.id);
    await action("Confirm", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await action("Close").trigger("click");
  });

  it("identifies Console User owned accounts and does not offer their direct deletion", async () => {
    const user = await consoleUser.serviceAccount.console.users.create({
      email: `linked-${crypto.randomUUID()}@example.com`,
      password: consoleUser.password,
    });
    const account = (await consoleUser.serviceAccount.core.service_accounts.list()).find(
      (account) => account.consoleUserId === user.id,
    )!;
    const page = await mountPage(Accounts);
    wrapper = page.wrapper;
    const setup = wrapper.setupState;
    const accountRow = wrapper.findAll("tr").find((row) => row.text().includes(account.keyPrefix))!;
    await action("Manage", accountRow.element).trigger("click");
    await settle(page.controlPlane);
    await field("Account Enabled").trigger("click");
    await field("Account Enabled").trigger("click");
    expect(wrapper.text()).toContain("Console User Owned Service Account");
    expect(wrapper.findAll("button").some((button) => button.text() === "Delete Account")).toBe(
      false,
    );
    expect(wrapper.find('a[href*="/control-plane/users"]').exists()).toBe(true);
    page.controlPlane.selectedAccount.value = {
      ...account,
      description: undefined,
      scopes: null,
      allowedIps: null,
    };
    await nextTick();
    expect(setup.accountForm.scopes).toEqual([]);
    expect(field("Allowed IP Addresses").element).toHaveProperty("value", "");
    await selectOption("Scopes", "console:user.read");
    const loading = page.controlPlane.load("service-accounts");
    await setup.saveAccount();
    await loading;
    expect(page.controlPlane.selectedAccount.value).not.toBeNull();
    await action("Cancel").trigger("click");
    await setup.editAccount(crypto.randomUUID());
    expect(wrapper.text()).toContain("Request Failed");
    await consoleUser.serviceAccount.console.users.delete(user.id);
  });

  it("keeps a failed account deletion recoverable", async () => {
    const account = await consoleUser.serviceAccount.core.service_accounts.create({
      name: "Stale Account",
      scopes: ["core:event.read"],
    });
    const page = await mountPage(Accounts);
    wrapper = page.wrapper;
    await wrapper.setupState.editAccount(account.id);
    await field("Description").setValue("Updated Before Deletion");
    await wrapper.setupState.saveAccount();
    expect(page.controlPlane.selectedAccount.value).toBeNull();
    await wrapper.setupState.editAccount(account.id);
    await consoleUser.serviceAccount.core.service_accounts.delete(account.id);
    await wrapper.setupState.saveAccount();
    expect(wrapper.text()).toContain("Request Failed");
    await action("Delete Account").trigger("click");
    await flushPromises();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await action("Delete Account").trigger("click");
    await flushPromises();
    await action("Close", document.querySelector('[role="dialog"]')!).trigger("click");
    await flushPromises();
    await action("Delete Account").trigger("click");
    await flushPromises();
    await action("Delete Account", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Request Failed");
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
  });

  it("validates SQL hooks and preserves failed edits", async () => {
    const page = await mountPage(Hooks);
    wrapper = page.wrapper;
    const setup = wrapper.setupState;
    await selectOption("Delivery Driver", "SQL Function");
    await field("Name").setValue("SQL Integration Hook");
    await createOption("Event Types", "console.user.created");
    await field("SQL Function Name").setValue("public.console_integration_hook");
    await wrapper.get("#event-hook-form").trigger("submit");
    await settle(page.controlPlane);
    const hook = (await consoleUser.serviceAccount.core.event_hooks.list()).find(
      (hook) => hook.name === "SQL Integration Hook",
    )!;
    await selectOption("Driver Filter", "SQL");
    await selectOption("Status Filter", "Enabled");
    await wrapper.get("#event-hook-filters").trigger("submit");
    await settle(page.controlPlane);
    const hookRow = wrapper.findAll("tr").find((row) => row.text().includes(hook.name))!;
    await action("Manage", hookRow.element).trigger("click");
    await settle(page.controlPlane);
    const loading = page.controlPlane.load("event-hooks");
    await setup.saveHook();
    await loading;
    expect(page.controlPlane.selectedHook.value).not.toBeNull();
    await field("Name").setValue("");
    await setup.saveHook();
    expect(page.controlPlane.error.value).not.toBe("");
    await setup.editHook(String(hook.id));
    page.controlPlane.selectedHook.value = { ...hook, eventTypes: null };
    await nextTick();
    expect(wrapper.setupState.hookForm.eventTypes).toEqual([]);
    await createOption("Event Types", "*");
    await field("SQL Function Name").setValue("invalid-function-name");
    await wrapper.get("#event-hook-form").trigger("submit");
    await settle(page.controlPlane);
    await action("Cancel").trigger("click");
    await setup.editHook(String(hook.id));
    await action("Delete Hook").trigger("click");
    await flushPromises();
    await action("Close", document.querySelector('[role="dialog"]')!).trigger("click");
    await flushPromises();
    await action("Delete Hook").trigger("click");
    await flushPromises();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await consoleUser.serviceAccount.core.event_hooks.delete(String(hook.id));
    await setup.deleteHook();
    expect(wrapper.text()).toContain("Request Failed");
    await setup.editHook(crypto.randomUUID());
  });

  it("delivers and retries events against a real HTTP listener", async () => {
    const hook = await consoleUser.serviceAccount.core.event_hooks.create({
      name: "Retry Integration Hook",
      driver: "http",
      eventTypes: ["console.user.created"],
      httpTargetUrl: `${webhookUrl}/fail`,
      maxRetries: 0,
    });
    await consoleUser.serviceAccount.console.users.create({
      email: `delivery-${crypto.randomUUID()}@example.com`,
      password: consoleUser.password,
    });
    await vi.waitFor(
      async () =>
        expect(
          (await consoleUser.serviceAccount.core.event_hooks.deliveries.list(String(hook.id))).some(
            (delivery) => delivery.attemptCount > 0 && !delivery.isDelivered,
          ),
        ).toBe(true),
      { timeout: 30000 },
    );
    const page = await mountPage(Hooks);
    wrapper = page.wrapper;
    await wrapper.setupState.editHook(String(hook.id));
    expect(wrapper.text()).toContain("Pending / Failed");
    await consoleUser.serviceAccount.core.event_hooks.update(String(hook.id), {
      httpTargetUrl: `${webhookUrl}/success`,
    });
    await action("Retry Delivery").trigger("click");
    await settle(page.controlPlane);
    await vi.waitFor(
      async () =>
        expect(
          (await consoleUser.serviceAccount.core.event_hooks.deliveries.list(String(hook.id))).some(
            (delivery) => delivery.isDelivered,
          ),
        ).toBe(true),
      { timeout: 30000 },
    );
    await wrapper.setupState.editHook(String(hook.id));
    expect(wrapper.text()).toContain("Delivered");
    await consoleUser.serviceAccount.core.event_hooks.delete(String(hook.id));
  });

  it("paginates audit events and renders nullable payloads", async () => {
    for (let index = 0; index < 26; index++)
      await consoleUser.serviceAccount.console.users.create({
        email: `audit-${crypto.randomUUID()}@example.com`,
        password: consoleUser.password,
      });
    const page = await mountPage(Events);
    wrapper = page.wrapper;
    const setup = wrapper.setupState;
    await vi.waitFor(() => expect(page.controlPlane.events.value?.count).toBe(25));
    await action("Next").trigger("click");
    await settle(page.controlPlane);
    expect(page.controlPlane.eventOffset.value).toBe(25);
    await action("Previous").trigger("click");
    await settle(page.controlPlane);
    expect(setup.details.value).toEqual([]);
    expect(setup.eventData.value).toEqual([]);
    expect(setup.eventMetadata.value).toEqual([]);
    await action("Details").trigger("click");
    await settle(page.controlPlane);
    const event = page.controlPlane.selectedEvent.value!;
    await action("Event Data").trigger("click");
    await flushPromises();
    page.controlPlane.selectedEvent.value = {
      ...event,
      resourceId: undefined,
      actor: { type: "system" },
      context: {},
      metadata: { source: "test" },
    };
    await nextTick();
    await action("Metadata").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("Not Recorded");
    page.controlPlane.selectedEvent.value = { ...event, data: null, metadata: null };
    await nextTick();
    expect(setup.eventData.value).toEqual([]);
    expect(setup.eventMetadata.value).toEqual([]);
    page.controlPlane.events.value = { ...page.controlPlane.events.value!, total: 25 };
    await nextTick();
    expect(action("Next").attributes("disabled")).toBeDefined();
    await selectOption("Actor Type", "User");
    await field("Actor ID").setValue(consoleUser.user.id);
    await field("Resource ID").setValue(event.resourceId || "missing");
    await wrapper.get("form").trigger("submit");
    await settle(page.controlPlane);
    await page.controlPlane.selectEvent("invalid-id");
    expect(wrapper.text()).toContain("Request Failed");
  });
});
