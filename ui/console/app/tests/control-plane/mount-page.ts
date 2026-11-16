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
  const wrapper = await mountSuspended(component, { route: false, attachTo: document.body });
  const controlPlane = wrapper.setupState.controlPlane as ReturnType<typeof useControlPlane>;
  await flushPromises();
  if (controlPlane) await vi.waitFor(() => expect(controlPlane.busy.value).toBe(false));
  return { wrapper, controlPlane };
}

export function action(label: string, scope: ParentNode = document) {
  const element = [...scope.querySelectorAll("button")].find(
    (button) => button.textContent?.trim() === label || button.getAttribute("aria-label") === label,
  );
  expect(element, `Missing action: ${label}`).toBeDefined();
  return new DOMWrapper(element!);
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
  await vi.waitFor(() => expect(controlPlane.busy.value).toBe(false));
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
    '[role="dialog"] input, [role="combobox"] input, input[role="combobox"]',
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
