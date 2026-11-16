import { beforeAll, describe, expect, it } from "vitest";
import useControlPlane from "../../composables/use-control-plane";
import { signInConsoleUser, testServiceAccount } from "./mount-page";

describe("control-plane API integration", () => {
  let serviceAccount: ReturnType<typeof testServiceAccount>;
  let controlPlane: ReturnType<typeof useControlPlane>;
  beforeAll(async () => {
    await signInConsoleUser();
    serviceAccount = testServiceAccount();
    controlPlane = useControlPlane(serviceAccount.core, serviceAccount.console);
  });

  it("loads every section and persists Console configuration", async () => {
    for (const section of [
      "settings",
      "users",
      "service-accounts",
      "event-hooks",
      "events",
    ] as const)
      expect(await controlPlane.load(section)).toBe(true);
    const original = await serviceAccount.console.config.get();
    expect(await controlPlane.saveConfig({ ...original, sessionIdleTimeoutSeconds: 60 })).toBe(
      true,
    );
    expect((await serviceAccount.console.config.get()).sessionIdleTimeoutSeconds).toBe(60);
    await serviceAccount.console.config.update(original);
  });

  it("manages Console Users and their automatically linked Service Accounts", async () => {
    const email = `api-${crypto.randomUUID()}@example.com`;
    expect(await controlPlane.createUser({ email, password: "Initial-password-123!" })).toBe(true);
    const user = controlPlane.users.value.find((user) => user.email === email)!;
    const accounts = await serviceAccount.core.service_accounts.list();
    expect(accounts.filter((account) => account.consoleUserId === user.id)).toHaveLength(1);
    expect(await controlPlane.selectUser(user.id)).toBe(true);
    expect(await controlPlane.updateUser(user.id, { email, isEnabled: true })).toBe(true);
    expect(await controlPlane.updateUser(user.id, { isEnabled: false })).toBe(true);
    expect(controlPlane.sessions.value).toEqual([]);
    expect(await controlPlane.resetPassword(user.id, "Changed-password-123!")).toBe(true);
    expect(await controlPlane.revokeSession(user.id, crypto.randomUUID())).toBe(false);
    expect(await controlPlane.revokeSession(user.id)).toBe(true);
    expect(await controlPlane.deleteUser(user.id)).toBe(true);
    expect(controlPlane.selectedUser.value).toBeNull();
  });

  it("manages standalone Service Accounts and one-time secrets", async () => {
    expect(
      await controlPlane.createAccount({
        name: "API Integration Account",
        scopes: ["core:event.read"],
      }),
    ).toBe(true);
    expect(controlPlane.secretKey.value).not.toBe("");
    const account = controlPlane.accounts.value.find(
      (account) => account.name === "API Integration Account",
    )!;
    expect(await controlPlane.selectAccount(account.id)).toBe(true);
    expect(await controlPlane.updateAccount(account.id, { isEnabled: false })).toBe(true);
    expect(await controlPlane.deleteAccount(account.id)).toBe(true);
    expect(controlPlane.selectedAccount.value).toBeNull();
  });

  it("manages both hook filters and hook deliveries", async () => {
    expect(
      await controlPlane.createHook({
        name: "API Integration Hook",
        driver: "http",
        eventTypes: ["console.user.created"],
        httpTargetUrl: `${process.env.LAYR__SERVER__BASE_URL}/missing-webhook`,
        maxRetries: 0,
      }),
    ).toBe(true);
    const hook = controlPlane.hooks.value.find((hook) => hook.name === "API Integration Hook")!;
    expect(await controlPlane.selectHook(String(hook.id))).toBe(true);
    expect(await controlPlane.updateHook(String(hook.id), { isEnabled: false })).toBe(true);
    controlPlane.hookFilters.value = { driver: "http", status: "disabled" };
    expect(await controlPlane.load("event-hooks")).toBe(true);
    expect(
      controlPlane.hooks.value.every((entry) => entry.driver === "http" && !entry.isEnabled),
    ).toBe(true);
    controlPlane.hookFilters.value.status = "enabled";
    expect(await controlPlane.load("event-hooks")).toBe(true);
    expect(await controlPlane.retryDelivery(String(hook.id), crypto.randomUUID())).toBe(false);
    expect(await controlPlane.deleteHook(String(hook.id))).toBe(true);
    controlPlane.hookFilters.value = { driver: "all", status: "all" };
  });

  it("filters, paginates, and inspects actual audit events", async () => {
    expect(await controlPlane.listEvents()).toBe(true);
    const event = controlPlane.events.value!.data![0]!;
    expect(await controlPlane.selectEvent(String(event.id))).toBe(true);
    controlPlane.eventFilters.value.type = ` ${event.type} `;
    expect(await controlPlane.listEvents(25)).toBe(true);
    expect(controlPlane.eventOffset.value).toBe(25);
    expect(await controlPlane.listEvents()).toBe(true);
  });

  it("preserves errors and prevents duplicate operations", async () => {
    expect(await controlPlane.selectUser("not-a-uuid")).toBe(false);
    expect(controlPlane.error.value).not.toBe("");
    const loading = controlPlane.load("settings");
    expect(await controlPlane.deleteUser(crypto.randomUUID())).toBe(false);
    expect(await loading).toBe(true);
    expect(controlPlane.error.value).toBe("");
  });

  it("handles non-Error transport failures", async () => {
    const failingConsole = {
      ...serviceAccount.console,
      config: {
        ...serviceAccount.console.config,
        get: () => Promise.reject("Unexpected rejection"),
      },
    };
    const rejectedControlPlane = useControlPlane(serviceAccount.core, failingConsole);
    expect(await rejectedControlPlane.load("settings")).toBe(false);
    expect(rejectedControlPlane.error.value).toBe("The request failed. Please try again.");
    expect(rejectedControlPlane.busy.value).toBe(false);
  });
});
