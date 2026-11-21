import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { AuthRecords, UInputTags, UModal } from "#components";
import type { AuthControlPlaneModels as Models } from "layr-client";
import Applications from "../../pages/auth/applications.vue";
import APIs from "../../pages/auth/resource-servers.vue";
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
function state() {
  return wrapper.findComponent(AuthRecords).vm as unknown as typeof wrapper.setupState;
}
function row(identity: string) {
  return pageDOM()
    .findAll("tbody tr")
    .find((item) => item.text().includes(identity))!;
}
function modal(title: string) {
  return wrapper.findAllComponents(UModal).find((item) => item.props("title") === title)!;
}

describe("Auth application, API, and custom-provider configuration", () => {
  it("creates, filters, edits, and deletes an API without treating its scopes as service-account grants", async () => {
    ({ wrapper } = await mountPage(APIs));
    await state().remove();
    await action("New API").trigger("click");
    await state().save();
    expect(state().validate().length).toBeGreaterThan(0);
    await pageDOM().get("#auth-record").trigger("submit");
    await flushPromises();
    const identifier = `urn:console-api:${crypto.randomUUID()}`;
    await field("API Name").setValue("Console API");
    await field("Audience Identifier").setValue(identifier);
    await field("Description").setValue("Integration-test API");
    state().form.scopes = null;
    await nextTick();
    wrapper.findComponent(UInputTags).vm.$emit("update:modelValue", ["read", "write"]);
    await nextTick();
    expect(state().form.scopes).toEqual(["read", "write"]);
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    expect(wrapper.setupState.authControlPlane.error.value).toBe("");
    expect(row(identifier).text()).toContain("Console API");
    await row(identifier).find("td:last-child > div").trigger("click");
    expect(state().editing).toBe(false);
    await field("Search").setValue("not-a-registered-api");
    expect(pageDOM().text()).toContain("Try a different name");
    await field("Search").setValue("");
    await menuAction("Details", row(identifier).element);
    expect(field("Audience Identifier").attributes("disabled")).toBeDefined();
    await field("API Name").setValue("Updated API");
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    await menuAction("Delete API", row(identifier).element);
    modal("Delete API?").vm.$emit("update:open", true);
    await nextTick();
    expect(state().confirming).not.toBeNull();
    await action("Close", activeDialog()!).trigger("click");
    expect(state().confirming).toBeNull();
    await menuAction("Delete API", row(identifier).element);
    await action("Cancel", activeDialog()!).trigger("click");
    await menuAction("Delete API", row(identifier).element);
    await action("Delete API", activeDialog()!).trigger("click");
    await settle();
    expect(
      ((await rootAuth.cp.config.get()).oidc.resourceServers || []).some(
        (item) => item.identifier === identifier,
      ),
    ).toBe(false);
  });

  it("registers confidential applications and keeps failed mutation dialogs recoverable", async () => {
    ({ wrapper } = await mountPage(Applications));
    await action("New Application").trigger("click");
    const clientId = crypto.randomUUID();
    await field("Application Name").setValue("Console App");
    await field("OIDC Client ID").setValue(clientId);
    await field("Public Client").trigger("click");
    await field("OIDC Client Secret").setValue("server-client-secret");
    state().form.redirectUris = ["https://app.example.com/callback"];
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    expect(row(clientId).text()).toContain("Confidential");
    await row(clientId).trigger("click");
    expect(field("OIDC Client ID").attributes("disabled")).toBeDefined();
    await signInConsoleUser(["auth:config.read"]);
    await state().save();
    await settle();
    expect(activeDialog()?.textContent).toContain("Save Failed");
    await action("Close", activeDialog()!).trigger("click");
    expect(pageDOM().text()).not.toContain("New Application");
    await menuAction("Details", row(clientId).element);
    expect(field("Application Name").attributes("disabled")).toBeDefined();
    modal("Application Details").vm.$emit("update:open", false);
    await nextTick();
    await signInConsoleUser();
    await menuAction("Delete Application", row(clientId).element);
    await signInConsoleUser(["auth:config.read"]);
    await state().remove();
    await settle();
    expect(activeDialog()?.textContent).toContain("Action Failed");
    await action("Cancel", activeDialog()!).trigger("click");
    await signInConsoleUser();
    await menuAction("Delete Application", row(clientId).element);
    await action("Delete Application", activeDialog()!).trigger("click");
    await settle();
    expect(
      ((await rootAuth.cp.config.get()).oidc.clients || []).some(
        (item) => item.clientId === clientId,
      ),
    ).toBe(false);
  });

  it("validates custom providers and preserves configuration until a successful save", async () => {
    ({ wrapper } = await mountPage(Providers));
    await action("Add Custom Provider").trigger("click");
    await state().save();
    expect(state().validate().length).toBeGreaterThan(0);
    state().providerAdvanced = false;
    await pageDOM().get("#auth-record").trigger("submit");
    await flushPromises();
    expect(state().providerAdvanced).toBe(true);
    await field("Provider ID").setValue("console-ui-custom");
    await field("Authorization URL").setValue("https://identity.example.com/authorize");
    await field("Token URL").setValue("https://identity.example.com/token");
    await field("User Info URL").setValue("https://identity.example.com/userinfo");
    await field("Profile ID Attribute").setValue("sub");
    await field("OAuth Scopes").setValue("openid email profile");
    await pageDOM().get("#auth-record").trigger("submit");
    await settle();
    const config = await rootAuth.cp.config.get();
    expect(config.oauthProviders!["console-ui-custom"]).toMatchObject({
      enabled: false,
      preset: "custom",
      scope: "openid email profile",
      authUrl: "https://identity.example.com/authorize",
    });
    original.oauthProviders = {
      ...original.oauthProviders,
      "console-ui-custom": config.oauthProviders!["console-ui-custom"]!,
    };
    await row("console-ui-custom").trigger("click");
    expect(state().providerAdvanced).toBe(true);
    await action("Close", activeDialog()!).trigger("click");
    const control = wrapper.setupState.authControlPlane;
    const google = control.config.value.oauthProviders.google;
    google.preset = "";
    await row("google").trigger("click");
    expect(activeDialog()?.textContent).toContain("Google Settings");
    await action("Close", activeDialog()!).trigger("click");
    await action("Refresh").trigger("click");
    await settle();
  });
});
