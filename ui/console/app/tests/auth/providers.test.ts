import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useNuxtApp } from "#app";
import type { AuthControlPlaneModels as Models } from "layr-client";
import Providers from "../../pages/auth/providers.vue";
import { authConfigInput } from "../../utils/auth-forms";
import { rootAuth } from "./helpers";
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
beforeEach(async () => {
  await signInConsoleUser();
  original = await rootAuth.cp.config.get();
});
afterEach(async () => {
  wrapper?.unmount();
  await rootAuth.cp.config.update(authConfigInput(original));
  await useNuxtApp().$router.push("/");
  vi.restoreAllMocks();
});
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.authControlPlane.busy.value).toBe(false));
}
function providerRow(label: string) {
  return pageDOM()
    .findAll("tbody tr")
    .find((row) => row.text().includes(label))!;
}
describe("External OAuth provider setup", () => {
  it.each(["Google", "GitHub", "Apple", "Discord"])(
    "only permits credential and enable changes for the %s preset",
    async (label) => {
      ({ wrapper } = await mountPage(Providers));
      await providerRow(label).trigger("click");
      const dialog = activeDialog()!;
      expect(dialog.textContent).toContain(`${label} Settings`);
      expect(field("Provider ID").attributes("disabled")).toBeDefined();
      expect(field("Callback URL").attributes("readonly")).toBeDefined();
      expect(field("OAuth Client ID").attributes("disabled")).toBeUndefined();
      expect(field("OAuth Client Secret").attributes("disabled")).toBeUndefined();
      expect(field("Enable Provider").attributes("disabled")).toBeUndefined();
      expect(dialog.querySelector('[role="combobox"]')).toBeNull();
      for (const text of [
        "Advanced Settings",
        "Authorization URL",
        "Token URL",
        "Profile ID Attribute",
        "OAuth Scopes",
        "Provider Preset",
      ])
        expect(dialog.textContent).not.toContain(text);
      expect(
        dialog.querySelectorAll(
          'input:not([type="checkbox"]):not([type="hidden"]):not([readonly]):not([disabled]), textarea',
        ),
      ).toHaveLength(2);
      if (label === "Apple") expect(dialog.textContent).toContain("client-secret JWT");
      if (label === "Google") {
        const writeClipboard = vi.spyOn(navigator.clipboard, "write").mockResolvedValue(undefined);
        await action("Copy Callback URL", dialog).trigger("click");
        await vi.waitFor(() => expect(writeClipboard).toHaveBeenCalledOnce());
        expect(dialog.textContent).toContain("Copied");
      }
    },
  );
  it("updates preset credentials, preserves saved secrets, and disables without deleting the preset", async () => {
    ({ wrapper } = await mountPage(Providers));
    await providerRow("Google").trigger("click");
    await field("OAuth Client ID").setValue("console-provider-client");
    await field("OAuth Client Secret").setValue("console-provider-secret");
    await field("Enable Provider").trigger("click");
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    expect(wrapper.setupState.authControlPlane.error.value).toBe("");
    const saved = await rootAuth.cp.config.get();
    expect(saved.oauthProviders!.google).toMatchObject({
      enabled: true,
      clientId: "console-provider-client",
      clientSecretConfigured: true,
    });
    expect(Object.keys(saved.oauthProviders!).filter((key) => key === "google")).toHaveLength(1);
    await providerRow("Google").trigger("click");
    expect(field("OAuth Client Secret").element).toHaveProperty("value", "");
    await field("OAuth Client ID").setValue("console-provider-client-updated");
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    expect((await rootAuth.cp.config.get()).oauthProviders!.google!.clientSecretConfigured).toBe(
      true,
    );
    await menuAction("Disable Provider", providerRow("Google").element);
    await action("Disable Provider", activeDialog()!).trigger("click");
    await settle();
    expect((await rootAuth.cp.config.get()).oauthProviders!.google!.enabled).toBe(false);
  });
  it("opens Add Custom Provider directly without a preset or provider selector", async () => {
    ({ wrapper } = await mountPage(Providers));
    await action("Add Custom Provider").trigger("click");
    const dialog = activeDialog()!;
    expect(dialog.textContent).toContain("Add Custom Provider");
    expect(dialog.querySelector('[role="combobox"]')).toBeNull();
    expect(dialog.textContent).not.toContain("Provider Preset");
    expect(dialog.textContent).toContain("Authorization URL");
    expect(dialog.textContent).toContain("Token URL");
    expect(field("Provider ID").attributes("disabled")).toBeUndefined();
    expect(field("Callback URL").element).toHaveProperty("value", "");
    await field("Provider ID").setValue("company-sso");
    expect(field("Callback URL").element).toHaveProperty(
      "value",
      `${useNuxtApp().$client.value.endpoint}/v1/oauth/company-sso/callback`,
    );
    await action("Advanced Settings", dialog).trigger("click");
    expect(dialog.textContent).toContain("OAuth Scopes");
  });
});
