import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import useImageControlPlane from "../../composables/use-image-control-plane";
import Layout from "../../pages/image.vue";
import Configuration from "../../pages/image/configurations.vue";
import Presets from "../../pages/image/presets.vue";
import Signing from "../../pages/image/signing.vue";
import Statistics from "../../pages/image/index.vue";
import {
  action,
  field,
  flushPromises,
  mountPage,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
beforeEach(async () => {
  consoleUser = await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.imageControlPlane.busy.value).toBe(false));
}

describe("Image permissions and failure handling", () => {
  it("checks every nested route against Image scopes", async () => {
    const router = consoleUser.nuxtApp.$router;
    for (const path of ["/image", "/image/presets", "/image/signing", "/image/configurations"]) {
      await router.push(path);
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(true);
      wrapper.unmount();
    }
    await signInConsoleUser(["image:preset.read"]);
    await router.push("/image");
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.canView.value).toBe(false);
    expect(wrapper.text()).toContain("Access Restricted");
    expect(wrapper.find('a[href="/image/signing"]').exists()).toBe(false);
    wrapper.unmount();
    await router.push("/image/presets");
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.setupState.canView.value).toBe(true);
    wrapper.unmount();
    await router.push("/");
  });

  it("keeps configuration failures visible and unavailable forms safe", async () => {
    ({ wrapper } = await mountPage(Configuration));
    const original = { ...wrapper.setupState.imageControlPlane.config.value };
    expect(
      await wrapper.setupState.imageControlPlane.saveConfig({ ...original, defaultQuality: 0 }),
    ).toBe(false);
    expect(wrapper.text()).toContain("Request Failed");
    wrapper.setupState.imageControlPlane.config.value = null;
    await nextTick();
    await wrapper.setupState.save();
    expect(wrapper.text()).toContain("Configuration Unavailable");
    wrapper.setupState.imageControlPlane.busy.value = true;
    await nextTick();
    expect(wrapper.text()).toContain("Loading Image Configuration");
    wrapper.setupState.imageControlPlane.busy.value = false;
  });

  it("renders read-only configuration and denies unauthorized configuration requests", async () => {
    await signInConsoleUser(["image:config.read"]);
    ({ wrapper } = await mountPage(Configuration));
    expect(wrapper.findAll("button").some((button) => button.text() === "Save Image Config")).toBe(
      false,
    );
    expect(field("Default Quality").attributes("disabled")).toBeDefined();
    expect(
      await wrapper.setupState.imageControlPlane.saveConfig(
        wrapper.setupState.imageControlPlane.config.value,
      ),
    ).toBe(false);
    expect(useNuxtApp().$auth.state.value).not.toBeNull();
    wrapper.unmount();
    await signInConsoleUser(["image:preset.read"]);
    ({ wrapper } = await mountPage(Configuration));
    expect(wrapper.text()).toContain("Configuration Unavailable");
    expect(await wrapper.setupState.imageControlPlane.load("configuration")).toBe(false);
    expect(wrapper.text()).toContain("Request Failed");
  });

  it("handles empty selection, canceled forms, and stale presets", async () => {
    ({ wrapper } = await mountPage(Presets));
    await wrapper.setupState.editPreset();
    await wrapper.setupState.deletePreset();
    await action("New Preset").trigger("click");
    await action("Cancel").trigger("click");
    expect(wrapper.find("#image-preset").exists()).toBe(false);
    wrapper.setupState.selectedPresetId.value = crypto.randomUUID();
    await wrapper.setupState.editPreset();
    expect(wrapper.text()).toContain("Request Failed");
    await action("Delete Preset").trigger("click");
    await flushPromises();
    await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
    await action("Delete Preset").trigger("click");
    await flushPromises();
    await action("Delete Preset", document.querySelector('[role="dialog"]')!).trigger("click");
    await settle();
    expect(document.querySelector('[role="dialog"]')).not.toBeNull();
    await action("Close", document.querySelector('[role="dialog"]')!).trigger("click");
  });

  it("preserves failed preset edits and displays presets without descriptions", async () => {
    const image = useNuxtApp().$client.value.image.cp;
    const preset = await image.presets.create({
      name: `image-empty-${crypto.randomUUID()}`,
      processingOptions: "rs:fill:48:48",
    });
    const conflictingPreset = await image.presets.create({
      name: `image-conflict-${crypto.randomUUID()}`,
      processingOptions: "rs:fill:48:48",
    });
    try {
      ({ wrapper } = await mountPage(Presets));
      wrapper.setupState.selectedPresetId.value = preset.id;
      await wrapper.setupState.editPreset();
      expect(field("Description").element).toHaveProperty("value", "");
      await field("Name").setValue(conflictingPreset.name);
      await wrapper.setupState.savePreset();
      expect(wrapper.find("#image-preset").exists()).toBe(true);
      expect(wrapper.text()).toContain("Request Failed");
      await action("New Preset").trigger("click");
      await field("Name").setValue(preset.name);
      await field("Processing Options").setValue("rs:fill:48:48");
      await wrapper.setupState.savePreset();
      expect(wrapper.text()).toContain("Request Failed");
    } finally {
      await image.presets.delete(preset.id);
      await image.presets.delete(conflictingPreset.id);
    }
  });

  it("shows preset details without exposing write controls", async () => {
    const image = useNuxtApp().$client.value.image.cp;
    const preset = await image.presets.create({
      name: `image-read-${crypto.randomUUID()}`,
      processingOptions: "rs:fill:48:48",
    });
    try {
      await signInConsoleUser(["image:preset.read"]);
      ({ wrapper } = await mountPage(Presets));
      wrapper.setupState.selectedPresetId.value = preset.id;
      await wrapper.setupState.editPreset();
      expect(
        wrapper
          .findAll("button")
          .some((button) => ["New Preset", "Save Preset", "Delete Preset"].includes(button.text())),
      ).toBe(false);
      expect(field("Processing Options").attributes("disabled")).toBeDefined();
    } finally {
      await signInConsoleUser();
      await image.presets.delete(preset.id);
    }
    wrapper.unmount();
    await signInConsoleUser(["image:stats.read"]);
    ({ wrapper } = await mountPage(Presets));
    expect(wrapper.text()).toContain("No Presets Yet");
  });

  it("renders signing errors without retaining a previous signed result", async () => {
    ({ wrapper } = await mountPage(Signing));
    await field("Image Path").setValue("/plain/https://example.com/a.png");
    await action("Sign URL").trigger("click");
    await settle();
    expect(wrapper.text()).toContain("Signed Image URL");
    await field("Image Path").setValue("");
    await wrapper.setupState.sign();
    expect(wrapper.text()).toContain("Request Failed");
    expect(wrapper.text()).not.toContain("Signed Image URL");
    wrapper.unmount();
    await signInConsoleUser(["image:stats.read"]);
    ({ wrapper } = await mountPage(Signing));
    expect(wrapper.findAll("button").some((button) => button.text() === "Sign URL")).toBe(false);
  });

  it("handles unavailable statistics and failed statistics requests", async () => {
    await signInConsoleUser(["image:config.read"]);
    ({ wrapper } = await mountPage(Statistics));
    expect(wrapper.text()).toContain("Statistics Unavailable");
    wrapper.setupState.imageControlPlane.busy.value = true;
    await nextTick();
    expect(wrapper.text()).toContain("Loading Image Statistics");
    wrapper.setupState.imageControlPlane.busy.value = false;
    expect(await wrapper.setupState.imageControlPlane.load("statistics")).toBe(false);
    expect(wrapper.text()).toContain("Request Failed");
  });

  it("serializes requests and provides meaningful errors for thrown values", async () => {
    const imageControlPlane = useImageControlPlane(useNuxtApp().$client.value.image.cp);
    expect(
      await imageControlPlane.run(async () => {
        expect(await imageControlPlane.load("statistics")).toBe(false);
      }),
    ).toBe(true);
    expect(
      await imageControlPlane.run(async () => {
        throw new Error("Image operation failed");
      }),
    ).toBe(false);
    expect(imageControlPlane.error.value).toBe("Image operation failed");
    expect(
      await imageControlPlane.run(async () => {
        throw "failure";
      }),
    ).toBe(false);
    expect(imageControlPlane.error.value).toBe("The request failed. Please try again.");
  });

  it("clears expired Console authentication after a rejected Image request", async () => {
    ({ wrapper } = await mountPage(Configuration));
    await consoleUser.serviceAccount.console.users.sessions.delete(
      consoleUser.user.id,
      consoleUser.nuxtApp.$auth.state.value!.id,
    );
    wrapper.setupState.imageControlPlane.config.value = null;
    await wrapper.setupState.load();
    expect(wrapper.setupState.imageControlPlane.error.value).not.toBe("");
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
    expect(consoleUser.nuxtApp.$router.currentRoute.value.path).toBe("/sign-in");
  });
});
