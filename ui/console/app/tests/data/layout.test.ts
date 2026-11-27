import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useNuxtApp } from "#app";
import Layout from "../../pages/data.vue";
import { mountPage, pageDOM, signInConsoleUser } from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(async () => {
  wrapper?.unmount();
  await useNuxtApp().$router.push("/");
});
describe("Data service layout", () => {
  it("matches service sidebar spacing and scope-aware navigation", async () => {
    ({ wrapper } = await mountPage(Layout));
    const menu = pageDOM().get('nav[aria-label="Data Service Settings"]');
    expect(menu.element.parentElement?.className).toBe("-mx-2.5");
    expect(menu.findAll("a").map((link) => link.text())).toEqual([
      "Tables",
      "SQL Editor",
      "Database Functions",
      "API Explorer",
      "Key–Value",
      "Realtime",
      "Configurations",
      "Cache",
    ]);
    for (const path of [
      "/data",
      "/data/sql",
      "/data/functions",
      "/data/api",
      "/data/key-value",
      "/data/realtime",
      "/data/configurations",
      "/data/cache",
    ]) {
      await useNuxtApp().$router.push(path);
      expect(wrapper.setupState.canView.value).toBe(true);
    }
    await signInConsoleUser(["data:query.read"]);
    await useNuxtApp().$router.push("/data");
    expect(wrapper.setupState.canView.value).toBe(false);
    expect(wrapper.setupState.items.value.map((item: { label: string }) => item.label)).toEqual([
      "API Explorer",
      "Key–Value",
      "Realtime",
    ]);
    await useNuxtApp().$router.push("/data/api");
    expect(wrapper.setupState.canView.value).toBe(true);
    await signInConsoleUser([]);
    expect(wrapper.setupState.items.value).toEqual([]);
  });
});
