import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useNuxtApp } from "#app";
import DefaultLayout from "../../layouts/default.vue";
import Overview from "../../pages/index.vue";
import { mountPage, signInConsoleUser } from "./mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let enabledServices: string[] | null;
beforeEach(async () => {
  await signInConsoleUser();
  const manifest = useNuxtApp().$manifest;
  enabledServices = manifest.state.value.enabledServices;
  manifest.state.value = {
    ...manifest.state.value,
    enabledServices: ["function", "image", "file_storage"],
  };
});
afterEach(async () => {
  wrapper?.unmount();
  const nuxtApp = useNuxtApp();
  nuxtApp.$manifest.state.value = { ...nuxtApp.$manifest.state.value, enabledServices };
  await nuxtApp.$router.push("/");
});

describe("Canonical service names and routes", () => {
  it("uses canonical service names and routes in main navigation", async () => {
    ({ wrapper } = await mountPage(DefaultLayout));
    const router = useNuxtApp().$router;
    const links = wrapper
      .findAll("header nav a")
      .map((link) => ({ label: link.text(), href: link.attributes("href") }));
    expect(links).toEqual(
      expect.arrayContaining([
        { label: "Function", href: router.resolve("/function").href },
        { label: "Image", href: router.resolve("/image").href },
        { label: "File Storage", href: router.resolve("/file-storage").href },
      ]),
    );
    expect(links.some((link) => link.label === "Functions" || link.label === "Images")).toBe(false);
  });

  it("uses the same service names and destinations in overview shortcuts", async () => {
    ({ wrapper } = await mountPage(Overview));
    const services = wrapper.setupState.overviewServices.value;
    expect(
      services.find((service: { serviceKey: string }) => service.serviceKey === "function"),
    ).toMatchObject({ title: "Function", to: "/function", quickAction: "View Endpoints" });
    expect(
      services.find((service: { serviceKey: string }) => service.serviceKey === "image"),
    ).toMatchObject({ title: "Image", to: "/image/presets", quickAction: "View Presets" });
    expect(
      services.find((service: { serviceKey: string }) => service.serviceKey === "file_storage"),
    ).toMatchObject({ title: "File Storage", to: "/file-storage", quickAction: "Browse Buckets" });
  });

  it.each([
    "/function",
    "/function/endpoints",
    "/function/deployments",
    "/function/executions",
    "/function/domains",
    "/function/configurations",
    "/image",
    "/image/presets",
    "/image/signing",
    "/image/configurations",
    "/file-storage",
    "/file-storage/buckets",
    "/file-storage/objects",
    "/file-storage/signing",
    "/file-storage/configurations",
  ])("resolves %s through generated Nuxt routes", (path) => {
    expect(useNuxtApp().$router.resolve(path).matched.length).toBeGreaterThan(0);
  });

  it.each([
    ["function", "/function/endpoints"],
    ["image", "/image/presets"],
    ["file_storage", "/file-storage/buckets"],
  ])("guards disabled %s services using the singular route prefix", async (service, path) => {
    const nuxtApp = useNuxtApp();
    await nuxtApp.$router.push(path);
    expect(nuxtApp.$router.currentRoute.value.path).toBe(path);
    nuxtApp.$manifest.state.value = {
      ...nuxtApp.$manifest.state.value,
      enabledServices: ["function", "image", "file_storage"].filter((name) => name !== service),
    };
    await nuxtApp.$router.push("/");
    await nuxtApp.$router.push(path);
    expect(nuxtApp.$router.currentRoute.value.path).toBe("/");
  });
});
