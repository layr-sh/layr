import { expect, vi } from "vitest";
import { DOMWrapper, flushPromises } from "@vue/test-utils";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import { useNuxtApp } from "#app";
import type { Component } from "vue";
import { createClient, useConsole, useCore, withServiceAccount } from "layr-client";
import type useControlPlane from "../../composables/use-control-plane";

export function testServiceAccount() {
  const endpoint = process.env.LAYR__SERVER__BASE_URL;
  const serviceAccountKey = process.env.LAYR__SERVICE_ACCOUNT_KEY;
  if (!endpoint || !serviceAccountKey)
    throw new Error("Run the Console tests through scripts/check-console-ui.");
  const client = createClient({ endpoint });
  return {
    console: withServiceAccount(useConsole(client), { serviceAccountKey }).cp,
    core: withServiceAccount(useCore(client), { serviceAccountKey }).cp,
  };
}

export async function signInConsoleUser(scopes = ["*"]) {
  const serviceAccount = testServiceAccount();
  const email = `console-user-${crypto.randomUUID()}@example.com`;
  const password = "Console-user-test-password-123!";
  const user = await serviceAccount.console.users.create({ email, password });
  const account = (await serviceAccount.core.service_accounts.list()).find(
    (account) => account.consoleUserId === user.id,
  )!;
  await serviceAccount.core.service_accounts.update(account.id, { scopes });
  const nuxtApp = useNuxtApp();
  const result = await nuxtApp.$auth.signIn(email, password);
  expect(result.error).toBeNull();
  return { serviceAccount, user, password, nuxtApp };
}

export async function mountPage(component: Component) {
  const wrapper = await mountSuspended(component, {
    route: false,
    attachTo: document.body,
  });
  const controlPlane = wrapper.setupState.controlPlane as ReturnType<typeof useControlPlane>;
  await flushPromises();
  if (controlPlane)
    await vi.waitFor(() => expect(controlPlane.busy.value).toBe(false), { timeout: 10000 });
  if (wrapper.setupState.authControlPlane)
    await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
  if (wrapper.setupState.dataControlPlane)
    await vi.waitFor(() => expect(wrapper.setupState.dataControlPlane.busy.value).toBe(false));
  if (wrapper.setupState.imageControlPlane)
    await vi.waitFor(() => expect(wrapper.setupState.imageControlPlane.busy.value).toBe(false));
  if (wrapper.setupState.tasksControlPlane)
    await vi.waitFor(() => expect(wrapper.setupState.tasksControlPlane.busy.value).toBe(false));
  if (wrapper.setupState.functionControlPlane)
    await vi.waitFor(() => expect(wrapper.setupState.functionControlPlane.busy.value).toBe(false));
  if (wrapper.setupState.fileStorageControlPlane)
    await vi.waitFor(() =>
      expect(wrapper.setupState.fileStorageControlPlane.busy.value).toBe(false),
    );
  return { wrapper, controlPlane };
}

export function action(label: string, scope: ParentNode = document) {
  const buttons = [...scope.querySelectorAll("button")].filter(
    (button) => button.textContent?.trim() === label || button.getAttribute("aria-label") === label,
  );
  const element = label === "Close" ? buttons.at(-1) : buttons[0];
  expect(element, `Missing action: ${label}`).toBeDefined();
  const domWrapper = new DOMWrapper(element!);
  const trigger = domWrapper.trigger.bind(domWrapper);
  domWrapper.trigger = async (...args: Parameters<typeof domWrapper.trigger>) => {
    await trigger(...args);
    await flushPromises();
  };
  return domWrapper;
}

export function pageDOM() {
  return new DOMWrapper(document.body);
}

export function activeDialog() {
  return (
    [...document.querySelectorAll<HTMLElement>('[role="dialog"]')]
      .filter(
        (dialog) =>
          dialog.getAttribute("data-state") !== "closed" && !dialog.closest('[aria-hidden="true"]'),
      )
      .at(-1) || null
  );
}

export async function menuAction(label: string, scope: ParentNode = document) {
  const direct = [...scope.querySelectorAll("button")].find(
    (button) => button.textContent?.trim() === label,
  );
  if (direct) {
    await new DOMWrapper(direct).trigger("click");
    await flushPromises();
    return;
  }
  const trigger = scope.querySelector<HTMLButtonElement>('button[aria-haspopup="menu"]');
  expect(trigger, `Missing row menu for ${label}`).not.toBeNull();
  await new DOMWrapper(trigger!).trigger("keydown", { key: "Enter" });
  await flushPromises();
  const item = [...document.querySelectorAll('[role="menuitem"]')].find(
    (item) => item.textContent?.trim() === label,
  );
  expect(item, `Missing menu item: ${label}`).toBeDefined();
  await new DOMWrapper(item!).trigger("click");
  await flushPromises();
}

export function field(label: string) {
  const element = [...document.querySelectorAll("label")].find(
    (element) => element.textContent?.trim() === label,
  );
  expect(element, `Missing field: ${label}`).toBeDefined();
  const inputElementById = document.getElementById(element!.htmlFor);
  expect(inputElementById, `Missing input for ${label}`).not.toBeNull();
  return new DOMWrapper(inputElementById!);
}

export async function settle(controlPlane: ReturnType<typeof useControlPlane>) {
  await flushPromises();
  await vi.waitFor(() => expect(controlPlane.busy.value).toBe(false), { timeout: 10000 });
}

export async function selectOption(label: string, optionLabel: string) {
  await field(label).trigger("click");
  await flushPromises();
  const option = [...document.querySelectorAll('[role="option"]')].find(
    (element) => element.textContent?.trim() === optionLabel,
  );
  expect(option, `Missing option: ${optionLabel}`).toBeDefined();
  await new DOMWrapper(option!).trigger("keydown", { key: "Enter" });
  await flushPromises();
}

export async function createOption(label: string, optionLabel: string) {
  await field(label).trigger("click");
  await flushPromises();
  const search = document.querySelector<HTMLInputElement>(
    '[data-reka-popper-content-wrapper] input[role="combobox"]',
  );
  expect(search).not.toBeNull();
  await new DOMWrapper(search!).setValue(optionLabel);
  await flushPromises();
  const option = [...document.querySelectorAll('[role="option"]')].find((element) =>
    element.textContent?.includes(optionLabel),
  );
  expect(option).toBeDefined();
  await new DOMWrapper(option!).trigger("keydown", { key: "Enter" });
  await flushPromises();
  await new DOMWrapper(search!).trigger("keydown", { key: "Escape" });
  await flushPromises();
}

export { flushPromises };
