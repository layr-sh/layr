import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { UCard } from "#components";

import Layout from "../../pages/control-plane.vue";
import Settings from "../../pages/control-plane/index.vue";
import Users from "../../pages/control-plane/users.vue";
import Accounts from "../../pages/control-plane/service-accounts.vue";
import Hooks from "../../pages/control-plane/event-hooks.vue";
import Events from "../../pages/control-plane/events.vue";
import Security from "../../pages/control-plane/account-security.vue";
import {
  action,
  field,
  flushPromises,
  mountPage,
  createOption,
  settle,
  signInConsoleUser,
} from "./mount-page";

type Wrapper = Awaited<ReturnType<typeof mountPage>>["wrapper"];
let wrapper: Wrapper;
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
beforeAll(async () => {
  consoleUser = await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());

describe("Console control-plane pages", () => {
  it("preserves the header and uses independent routes with vertical navigation", async () => {
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.get("h1").text()).toBe("Control Plane");
    expect(wrapper.text()).toContain("Manage your console, access, and platform configuration.");
    expect(wrapper.find('nav[aria-label="Control Plane Settings"]').exists()).toBe(true);
    for (const label of [
      "Configuration",
      "Console Users",
      "Service Accounts",
      "Event Hooks",
      "Events",
      "Account Security",
    ])
      expect(wrapper.text()).toContain(label);
  });

  it("uses card title and description props with footer-associated submit actions", async () => {
    for (const component of [Settings, Users, Accounts, Hooks, Events, Security]) {
      wrapper?.unmount();
      ({ wrapper } = await mountPage(component));
      for (const card of wrapper.findAllComponents(UCard)) {
        expect(card.props("title")).toBeTruthy();
        expect(card.props("description")).toBeTruthy();
      }
      for (const button of wrapper.findAll('button[type="submit"]')) {
        expect(button.element.closest('[data-slot="footer"]')).not.toBeNull();
        expect(button.attributes("form")).toBeTruthy();
        expect(document.getElementById(button.attributes("form")!)).not.toBeNull();
      }
    }
  });

  it("offers every standard event type and service wildcard without requiring existing hooks", async () => {
    ({ wrapper } = await mountPage(Hooks));
    const standardEvents = [
      "core",
      "console",
      "auth",
      "data",
      "filestorage",
      "function",
      "image",
      "tasks",
    ].flatMap((service) =>
      [...readFileSync(`../../${service}/event.go`, "utf8").matchAll(/NewEvent\("([^"]+)"/g)].map(
        (match) => match[1],
      ),
    );
    expect(wrapper.setupState.eventTypeOptions.value).toEqual(
      [
        ...new Set([
          "*",
          ...["core", "console", "auth", "data", "filestorage", "function", "image", "tasks"].map(
            (service) => `${service}.*`,
          ),
          ...standardEvents,
        ]),
      ].sort(),
    );
  });

  it("saves Console settings through the real configuration API", async () => {
    const page = await mountPage(Settings);
    wrapper = page.wrapper;
    const original = await consoleUser.serviceAccount.console.config.get();
    await field("Access Token Lifetime (Seconds)").setValue("60");
    await field("Maximum Session Lifetime (Seconds)").setValue("3600");

    await action("Save Console Config").trigger("click");
    await settle(page.controlPlane);
    expect(wrapper.text()).toContain("Console Settings Saved");
    expect(await consoleUser.serviceAccount.console.config.get()).toMatchObject({
      sessionIdleTimeoutSeconds: 60,
      sessionMaxLifetimeSeconds: 3600,
    });
    expect(wrapper.findAll("button").some((button) => button.text() === "Refresh")).toBe(false);
    expect(field("Access Token Lifetime (Seconds)").element).toHaveProperty("value", "60");
    await consoleUser.serviceAccount.console.config.update(original);
  });

  it("manages Console Users without leaving revoked sessions on screen", async () => {
    const page = await mountPage(Users);
    wrapper = page.wrapper;
    const email = `ui-${crypto.randomUUID()}@example.com`;
    await field("Email").setValue(email);
    await field("Initial Password").setValue("Initial-password-123!");
    await wrapper.get("#console-user-form").trigger("submit");
    await settle(page.controlPlane);
    const user = (await consoleUser.serviceAccount.console.users.list()).find(
      (user) => user.email === email,
    )!;
    const setup = wrapper.setupState;
    await setup.editUser(user.id);
    expect(wrapper.text()).toContain("Linked Service Account");
    await field("Account Enabled").trigger("click");
    await wrapper.get("#console-user-form").trigger("submit");
    await settle(page.controlPlane);
    expect(page.controlPlane.sessions.value).toEqual([]);
    await field("Account Enabled").trigger("click");
    await wrapper.get("#console-user-form").trigger("submit");
    await settle(page.controlPlane);
    await field("New Password").setValue("Changed-password-123!");
    await wrapper.get("#console-password-form").trigger("submit");
    await settle(page.controlPlane);
    expect(field("New Password").element).toHaveProperty("value", "");
    const signInResponse = await fetch(
      `${process.env.LAYR__SERVER__BASE_URL}/v1/_/console/sign-in`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password: "Changed-password-123!" }),
      },
    );
    expect(signInResponse.ok).toBe(true);
    await setup.editUser(user.id);
    expect(page.controlPlane.sessions.value).toHaveLength(1);
    await action("Revoke").trigger("click");
    await flushPromises();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await action("Revoke").trigger("click");
    await flushPromises();
    await action("Confirm", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(page.controlPlane.sessions.value).toEqual([]);
    await action("Delete User").trigger("click");
    await flushPromises();
    await action("Confirm", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.console.users.list()).some((entry) => entry.id === user.id),
    ).toBe(false);
    await setup.editUser(consoleUser.user.id);
    expect(wrapper.text()).toContain("You cannot disable or delete your own Console User");
    expect(wrapper.findAll("button").some((button) => button.text() === "Delete User")).toBe(false);
    await action("Close").trigger("click");
  });

  it("shows standalone ownership, preserves expiration precision, and removes standalone accounts", async () => {
    const page = await mountPage(Accounts);
    wrapper = page.wrapper;
    await wrapper.setupState.deleteAccount();
    await wrapper.get("#service-account-form").trigger("submit");
    await flushPromises();
    expect(wrapper.text()).toContain("At least one explicit scope");
    const name = `UI Account ${crypto.randomUUID()}`;
    await field("Name").setValue(name);
    await field("Description").setValue("Console integration test");
    await createOption("Scopes", "core:event.read");
    await createOption("Scopes", "console:config.read");
    await createOption("Scopes", "core:*.read");
    await field("Allowed IP Addresses").setValue("127.0.0.1");
    await field("Expiration (UTC)").setValue("2027-01-01T12:00:12.345");
    await wrapper.get("#service-account-form").trigger("submit");
    await settle(page.controlPlane);
    const account = (await consoleUser.serviceAccount.core.service_accounts.list()).find(
      (account) => account.name === name,
    )!;
    expect(account.expiresAt).toEqual(new Date("2027-01-01T12:00:12.345Z"));
    expect(account.scopes).toEqual(["core:event.read", "console:config.read", "core:*.read"]);
    expect(wrapper.text()).toContain("Standalone");
    const secret = page.controlPlane.secretKey.value;
    expect(secret).not.toBe("");
    await action("I Have Saved the Key").trigger("click");
    await wrapper.setupState.editAccount(account.id);
    await field("Name").setValue(`${name} Updated`);
    await wrapper.get("#service-account-form").trigger("submit");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.core.service_accounts.get(account.id)).expiresAt,
    ).toEqual(account.expiresAt);
    await wrapper.setupState.editAccount(account.id);
    await action("Cancel").trigger("click");
    await wrapper.setupState.editAccount(account.id);
    await action("Delete Account").trigger("click");
    await flushPromises();
    await action("Delete Account", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.core.service_accounts.list()).some(
        (entry) => entry.id === account.id,
      ),
    ).toBe(false);
  });

  it("manages HTTP hook configuration and keeps signing secrets private", async () => {
    const page = await mountPage(Hooks);
    wrapper = page.wrapper;
    await wrapper.setupState.deleteHook();
    await wrapper.get("#event-hook-form").trigger("submit");
    await flushPromises();
    expect(wrapper.text()).toContain("At least one event pattern");
    const name = `UI Hook ${crypto.randomUUID()}`;
    await field("Name").setValue(name);
    await createOption("Event Types", "console.user.created");
    await createOption("Event Types", "custom.integration.*");
    await field("Target URL").setValue(`${process.env.LAYR__SERVER__BASE_URL}/readyz`);
    await field("Signing Secret").setValue("integration-signing-secret");
    await field("Maximum Retries").setValue("2");
    await field("Timeout (Seconds)").setValue("5");
    await wrapper.get("#event-hook-form").trigger("submit");
    await settle(page.controlPlane);
    const hook = (await consoleUser.serviceAccount.core.event_hooks.list()).find(
      (hook) => hook.name === name,
    )!;
    await wrapper.setupState.editHook(String(hook.id));
    expect(field("Signing Secret").element).toHaveProperty("value", "");
    await field("Hook Enabled").trigger("click");
    await wrapper.get("#event-hook-form").trigger("submit");
    await settle(page.controlPlane);
    expect(
      (await consoleUser.serviceAccount.core.event_hooks.get(String(hook.id)))
        .signingSecretConfigured,
    ).toBe(true);
    await wrapper.setupState.editHook(String(hook.id));
    await action("Delete Hook").trigger("click");
    await flushPromises();
    await action("Delete Hook", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle(page.controlPlane);
  });

  it("filters and inspects real audit events", async () => {
    const page = await mountPage(Events);
    wrapper = page.wrapper;
    await vi.waitFor(() => expect(page.controlPlane.events.value?.count).toBeGreaterThan(0));
    await action("Details").trigger("click");
    await settle(page.controlPlane);
    const event = page.controlPlane.selectedEvent.value!;
    expect(wrapper.text()).toContain(String(event.id));
    await action("Close").trigger("click");
    await field("Event Type").setValue(event.type);
    await field("Resource Type").setValue(event.resourceType);
    await wrapper.get("form").trigger("submit");
    await settle(page.controlPlane);
    await action("Clear Filters").trigger("click");
    await settle(page.controlPlane);
  });
});
