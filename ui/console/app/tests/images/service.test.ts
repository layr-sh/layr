import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { UCard, UFormField } from "#components";
import { useNuxtApp } from "#app";
import { createClient, useImage, withServiceAccount } from "layr-client";
import Layout from "../../pages/image.vue";
import Configuration from "../../pages/image/configurations.vue";
import Presets from "../../pages/image/presets.vue";
import Signing from "../../pages/image/signing.vue";
import Statistics from "../../pages/image/index.vue";
import { action, field, mountPage, signInConsoleUser } from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];

const endpoint = process.env.LAYR__SERVER__BASE_URL;
const serviceAccountKey = process.env.LAYR__SERVICE_ACCOUNT_KEY;
if (!endpoint || !serviceAccountKey)
  throw new Error("Run Console tests through scripts/check-console-ui.");
const imageControlPlane = withServiceAccount(useImage(createClient({ endpoint })), {
  serviceAccountKey,
}).cp;

async function waitForRequest() {
  await vi.waitFor(() => expect(document.querySelector('[aria-busy="true"]')).toBeNull());
}

beforeAll(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());

describe("Image service integration", () => {
  it("shows nested, scope-aware navigation and standard card structure", async () => {
    ({ wrapper } = await mountPage(Layout));
    expect(wrapper.find('nav[aria-label="Image Service Settings"]').exists()).toBe(true);
    expect(
      wrapper
        .findAll('nav[aria-label="Image Service Settings"] a')
        .map((link) => ({ label: link.text(), to: link.attributes("href") })),
    ).toEqual([
      { label: "Statistics", to: useNuxtApp().$router.resolve("/image").href },
      { label: "Presets", to: useNuxtApp().$router.resolve("/image/presets").href },
      { label: "URL Signing", to: useNuxtApp().$router.resolve("/image/signing").href },
      { label: "Configurations", to: useNuxtApp().$router.resolve("/image/configurations").href },
    ]);

    for (const component of [Configuration, Presets, Signing, Statistics]) {
      wrapper.unmount();
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

  it("updates Image Configuration through the API", async () => {
    const original = await imageControlPlane.config.get();
    const page = await mountPage(Configuration);
    wrapper = page.wrapper;
    for (const formField of wrapper.findAllComponents(UFormField)) {
      expect(formField.props("description")).toBeTruthy();
    }
    try {
      for (const [label, value] of [
        ["Default Quality", "73"],
        ["Maximum Source Resolution (Megapixels)", "45"],
        ["Maximum Animation Frames", "64"],
        ["Cache Lifetime (Seconds)", "120"],
        ["Watermark Opacity", "0.65"],
      ]) {
        await field(label!).setValue(value!);
        await field(label!).trigger("blur");
      }
      for (const label of [
        "Allow Unsigned Requests",
        "Progressive JPEG",
        "Interlaced PNG",
        "Strip Metadata",
        "Keep Copyright",
        "Strip Color Profile",
      ])
        await field(label).trigger("click");
      await field("Watermark URL").setValue("https://images.example.com/watermark.png");
      await field("Allowed Source Domains").setValue("images.example.com, cdn.example.com\n");
      expect(action("Save Image Config").attributes("disabled")).toBeUndefined();
      await action("Save Image Config").trigger("click");
      await vi.waitFor(() => expect(wrapper.text()).toContain("Image Configuration Saved"));
      expect(await imageControlPlane.config.get()).toEqual({
        defaultQuality: 73,
        allowedDomains: ["images.example.com", "cdn.example.com"],
        maxSrcResolution: 45,
        maxAnimationFrames: 64,
        cacheTtlSeconds: 120,
        watermarkOpacity: 0.65,
        watermarkUrl: "https://images.example.com/watermark.png",
        allowInsecure: !original.allowInsecure,
        jpegProgressive: !original.jpegProgressive,
        pngInterlaced: !original.pngInterlaced,
        stripMetadata: !original.stripMetadata,
        keepCopyright: !original.keepCopyright,
        stripColorProfile: !original.stripColorProfile,
      });
      await wrapper.setupState.load();
      expect(field("Allowed Source Domains").element).toHaveProperty(
        "value",
        "images.example.com\ncdn.example.com",
      );
    } finally {
      await imageControlPlane.config.update(original);
    }
  });

  it("creates, edits, and deletes Image presets through the API", async () => {
    const page = await mountPage(Presets);
    wrapper = page.wrapper;
    const name = `console-image-${crypto.randomUUID()}`;
    await vi.waitFor(() => expect(wrapper.text()).toContain("Transformation Presets"));
    await action("New Preset").trigger("click");
    await field("Name").setValue(name);
    await field("Description").setValue("Console integration test");
    await field("Processing Options").setValue("w_48,h_48,fit_cover");
    await action("Create Preset").trigger("click");
    await vi.waitFor(() => expect(wrapper.text()).toContain("Image Preset Created"));
    expect((await imageControlPlane.presets.list()).data).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name, processingOptions: "w_48,h_48,fit_cover" }),
      ]),
    );
    expect(wrapper.find('[role="combobox"]').exists()).toBe(false);
    expect(wrapper.text()).not.toContain("No Presets Yet");
    const row = wrapper.findAll("tbody tr").find((row) => row.text().includes(name));
    expect(row).toBeDefined();
    expect(row!.text()).toContain("Console integration test");
    expect(row!.text()).toContain("w_48,h_48,fit_cover");
    await row!.trigger("click");
    expect(row!.attributes("data-selected")).toBe("true");
    await action("Edit Preset").trigger("click");
    await waitForRequest();
    await field("Description").setValue("Updated description");
    await action("Save Preset").trigger("click");
    await vi.waitFor(() => expect(wrapper.text()).toContain("Image Preset Updated"));
    expect((await imageControlPlane.presets.list()).data).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name, description: "Updated description" }),
      ]),
    );
    await action("Delete Preset").trigger("click");
    await vi.waitFor(() => expect(document.querySelector('[role="dialog"]')).not.toBeNull());
    await action("Delete Preset", document.querySelector('[role="dialog"]')!).trigger("click");
    await vi.waitFor(() => expect(wrapper.text()).toContain("Image Preset Deleted"));
    expect(
      (await imageControlPlane.presets.list()).data!.some((preset) => preset.name === name),
    ).toBe(false);
  });

  it("signs an image path and renders service statistics", async () => {
    ({ wrapper } = await mountPage(Signing));
    await field("Image Path").setValue("/resize,w_48/https://example.com/image.jpg");
    await action("Sign URL").trigger("click");
    await vi.waitFor(() => expect(wrapper.text()).toContain("Image URL Signed"));
    expect(field("Signed URL").element).toHaveProperty(
      "value",
      expect.stringContaining("/v1/image/"),
    );

    wrapper.unmount();
    ({ wrapper } = await mountPage(Statistics));
    const stats = await imageControlPlane.stats.get();
    await vi.waitFor(() => expect(wrapper.text()).toContain("Total Requests"));
    expect(wrapper.get('[data-testid="total-requests"]').text()).toBe(
      stats.totalRequests.toLocaleString(),
    );
    expect(wrapper.findAll("input")).toHaveLength(0);
    expect(wrapper.get('[data-testid="cache-hits"]').text()).toBe(stats.cacheHits.toLocaleString());
    expect(wrapper.get('[data-testid="cache-misses"]').text()).toBe(
      stats.cacheMisses.toLocaleString(),
    );
    expect(wrapper.get('[data-testid="cache-hit-ratio"]').text()).toBe(
      `${(stats.cacheHitRatio * 100).toFixed(2)}%`,
    );
    expect(wrapper.get('[data-testid="bytes-served"]').attributes("title")).toBe(
      `${stats.bytesServed.toLocaleString()} bytes`,
    );
    expect(wrapper.get('[data-testid="process-memory"]').attributes("title")).toMatch(/bytes$/);
    expect(wrapper.get('[role="progressbar"]').attributes("aria-label")).toBe(
      `Cache Hit Ratio: ${stats.cacheHitRatio * 100}%`,
    );
    expect(wrapper.text()).toContain("Cache Hit Ratio");
    expect(wrapper.findAll("button").some((button) => button.text().includes("Refresh"))).toBe(
      false,
    );
    await waitForRequest();
  });
});
