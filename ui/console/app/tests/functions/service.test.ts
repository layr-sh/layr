import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { gunzipSync } from "node:zlib";
import { useNuxtApp } from "#app";
import { UCard, UInputTags } from "#components";
import Layout from "../../pages/functions.vue";
import Statistics from "../../pages/functions/index.vue";
import Configuration from "../../pages/functions/configurations.vue";
import Endpoints from "../../pages/functions/endpoints.vue";
import Deployments from "../../pages/functions/deployments.vue";
import Executions from "../../pages/functions/executions.vue";
import Domains from "../../pages/functions/domains.vue";
import {
  action,
  field,
  flushPromises,
  mountPage,
  selectOption,
  signInConsoleUser,
} from "../control-plane/mount-page";
let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.functionControlPlane.busy.value).toBe(false), {
    timeout: 90000,
  });
}
function selectRow(text: string) {
  const row = wrapper.findAll("tbody tr").find((row) => row.text().includes(text));
  expect(row, `Missing row: ${text}`).toBeDefined();
  return row!.trigger("click");
}

describe("Function service integration", () => {
  it("uses the Image service layout, ordered navigation, and card/footer patterns", async () => {
    ({ wrapper } = await mountPage(Layout));
    expect(
      wrapper.findAll('nav[aria-label="Function Service Settings"] a').map((link) => link.text()),
    ).toEqual([
      "Statistics",
      "Endpoints",
      "Deployments",
      "Executions",
      "Domains",
      "Configurations",
    ]);
    const router = useNuxtApp().$router;
    for (const path of [
      "/functions",
      "/functions/endpoints",
      "/functions/deployments",
      "/functions/executions",
      "/functions/domains",
      "/functions/configurations",
    ]) {
      wrapper.unmount();
      await router.push(path);
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(true);
    }
    await router.push("/");
    for (const page of [Statistics, Configuration, Endpoints, Deployments, Executions, Domains]) {
      wrapper.unmount();
      ({ wrapper } = await mountPage(page));
      for (const card of wrapper.findAllComponents(UCard)) {
        expect(card.props("title")).toBeTruthy();
        expect(card.props("description")).toBeTruthy();
      }
      for (const button of wrapper.findAll('button[type="submit"]')) {
        expect(button.element.closest('[data-slot="footer"]')).not.toBeNull();
        expect(document.getElementById(button.attributes("form")!)).not.toBeNull();
      }
    }
  });

  it("saves configuration including workerd and DNS defaults", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const original = await api.config.get();
    ({ wrapper } = await mountPage(Configuration));
    try {
      await selectOption("Default Runtime", "workerd");
      expect(wrapper.findAll("label").map((label) => label.text())).not.toContain(
        "Default Memory Setting (MB)",
      );
      for (const title of [
        "Endpoint Defaults",
        "Deployment Policy",
        "Workerd Compatibility",
        "Custom Domain Ingress",
        "Memory Limits Are Not Enforced",
        "Applied on Runtime Reload",
      ])
        expect(wrapper.text()).toContain(title);
      for (const [label, value] of [
        ["Default Timeout (Seconds)", "25"],
        ["Maximum Bundle Size (MiB)", "32"],
      ]) {
        await field(label!).setValue(value!);
        await field(label!).trigger("blur");
      }
      for (const key of ["API_URL", "API_TOKEN"]) {
        await field("Allowed Environment Keys").setValue(key);
        await field("Allowed Environment Keys").trigger("keydown", { key: "Enter" });
      }
      await field("Compatibility Date").setValue("2026-08-04");
      wrapper.setupState.compatibilityFlags.value = [];
      await nextTick();
      await field("Compatibility Flags").setValue("nodejs_compat");
      await field("Compatibility Flags").trigger("keydown", { key: "Enter" });
      await field("Default ACME Email").setValue("ops@example.com");
      await field("Custom Domain CNAME Target").setValue("ingress.example.com");
      await field("Custom Domain Apex IP").setValue("127.0.0.1");
      await action("Save Function Config").trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Configuration Saved");
      expect(await api.config.get()).toEqual({
        defaultRuntime: "workerd",
        defaultMemoryLimitMb: original.defaultMemoryLimitMb,
        defaultTimeoutSeconds: 25,
        maxBundleSizeBytes: 32 * 1024 * 1024,
        allowedEnvironmentKeys: ["API_URL", "API_TOKEN"],
        defaultTlsAcmeEmail: "ops@example.com",
        customDomainCnameTarget: "ingress.example.com",
        customDomainApexIp: "127.0.0.1",
        workerdRuntime: { compatibilityDate: "2026-08-04", compatibilityFlags: ["nodejs_compat"] },
      });
    } finally {
      await api.config.update(original);
    }
  });

  it("preserves exact byte limits and memory metadata while normalizing and clearing list policy", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const original = await api.config.get();
    try {
      await api.config.update({
        ...original,
        maxBundleSizeBytes: 30000001,
        defaultMemoryLimitMb: 192,
        defaultRuntime: "unsupported",
      });
      ({ wrapper } = await mountPage(Configuration));
      expect(wrapper.setupState.bundleSizeMib.value).toBe(30000001 / (1024 * 1024));
      await selectOption("Default Runtime", "workerd");
      wrapper.setupState.allowedKeys.value = [" API_URL ", "API_URL"];
      wrapper.setupState.compatibilityFlags.value = [" nodejs_compat ", "nodejs_compat"];
      await nextTick();
      await action("Save Function Config").trigger("click");
      await settle();
      const saved = await api.config.get();
      expect(saved.defaultRuntime).toBe("workerd");
      expect(saved.maxBundleSizeBytes).toBe(30000001);
      expect(saved.defaultMemoryLimitMb).toBe(192);
      expect(saved.allowedEnvironmentKeys).toEqual(["API_URL"]);
      expect(saved.workerdRuntime.compatibilityFlags).toEqual(["nodejs_compat"]);
      wrapper.setupState.allowedKeys.value = ["API_URL"];
      wrapper.setupState.compatibilityFlags.value = ["nodejs_compat"];
      await nextTick();
      await wrapper
        .findAllComponents(UInputTags)[0]!
        .find('[data-slot="itemDelete"]')
        .trigger("click");
      await wrapper
        .findAllComponents(UInputTags)[1]!
        .find('[data-slot="itemDelete"]')
        .trigger("click");
      await action("Save Function Config").trigger("click");
      await settle();
      const cleared = await api.config.get();
      expect(cleared.allowedEnvironmentKeys).toEqual([]);
      expect(cleared.workerdRuntime.compatibilityFlags).toBeUndefined();
      await wrapper.setupState.load();
      expect(wrapper.setupState.compatibilityFlags.value).toEqual([]);
      expect(cleared.maxBundleSizeBytes).toBe(30000001);
      expect(cleared.defaultMemoryLimitMb).toBe(192);
    } finally {
      await api.config.update(original);
    }
  });

  it("creates private endpoints, edits settings, and confirms deletion", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const name = `console-function-${crypto.randomUUID()}`;
    ({ wrapper } = await mountPage(Endpoints));
    await action("New Endpoint").trigger("click");
    await field("Name").setValue(name);
    await field("Description").setValue("Console endpoint");
    await field("Entrypoint").setValue("index.js");
    await action("Create Endpoint").trigger("click");
    await settle();
    const endpoint = (await api.endpoints.list()).data!.find((item) => item.name === name)!;
    try {
      expect(endpoint.isPublic).toBe(false);
      expect(wrapper.text()).toContain(`/v1/function/${name}`);
      await action("Endpoint Details").trigger("click");
      await settle();
      await field("Description").setValue("Updated endpoint");
      await selectOption("Runtime", "workerd");
      await field("Public Endpoint").trigger("click");
      await field("Memory Setting (MB)").setValue("192");
      await field("Memory Setting (MB)").trigger("blur");
      await field("Timeout (Seconds)").setValue("20");
      await field("Timeout (Seconds)").trigger("blur");
      await action("Save Endpoint").trigger("click");
      await settle();
      expect(await api.endpoints.get(endpoint.id)).toEqual(
        expect.objectContaining({
          description: "Updated endpoint",
          isPublic: true,
          memoryLimitMb: 192,
          timeoutSeconds: 20,
        }),
      );
      await action("Delete Endpoint").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Delete Endpoint").trigger("click");
      await flushPromises();
      await action("Delete Endpoint", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Endpoint Deleted");
    } finally {
      if ((await api.endpoints.list()).data!.some((item) => item.id === endpoint.id))
        await api.endpoints.delete(endpoint.id);
    }
  });

  it.each(["zip", "tar"] as const)(
    "uploads a real %s bundle and preserves variables unless explicitly removed",
    async (format) => {
      const api = useNuxtApp().$client.value.function.cp;
      const endpoint = await api.endpoints.create({
        name: `archive-${format}-${crypto.randomUUID()}`,
        isPublic: true,
      });
      const encoded =
        format === "zip"
          ? "UEsDBBQAAAAIAG8WdF2FwXh4RgAAAEYAAAAIAAAAaW5kZXguanNLrSjILypRSElNSyzNKVGoVkhLLUnO0NAEsopSS0qL8hTyUssVglKLC/LzilM1lBKLkjMyy1J1Swty8hNTlDStFWoVaq0BUEsBAhQDFAAAAAgAbxZ0XYXBeHhGAAAARgAAAAgAAAAAAAAAAAAAAIABAAAAAGluZGV4LmpzUEsFBgAAAAABAAEANgAAAGwAAAAAAA=="
          : "H4sIAAAAAAAC/+3NPQqDQBiE4T3KYqVFwgYkFt4iNxD9RIOssj+JELx7lgRSpI9N3qeZYZoZbSfr8erVD5nkXJavTL7TnMz50997VZpKaaN2EH1oXLpU/0nWZXZBd9I3cQr6oXsJ7ZAXqTkJ0Vlt5a4v4pfZesmzxrXDeJNDXKa56bKi1pveagUAAAAAAAAAAAAAAAAA2NkTinAUvQAoAAA=";
      const compressedUint8Array = Uint8Array.from(atob(encoded), (character) =>
        character.charCodeAt(0),
      );
      const archiveUint8Array =
        format === "zip" ? compressedUint8Array : new Uint8Array(gunzipSync(compressedUint8Array));
      const file = new File([archiveUint8Array.buffer], `bundle.${format}`);
      try {
        ({ wrapper } = await mountPage(Deployments));
        await selectOption("Endpoint", endpoint.name);
        await settle();
        expect(wrapper.find('input[type="file"]').attributes("accept")).toBe(".zip,.tar");
        Object.defineProperty(wrapper.find('input[type="file"]').element, "files", {
          value: [file],
          configurable: true,
        });
        await wrapper.find('input[type="file"]').trigger("change");
        await vi.waitFor(() =>
          expect(wrapper.setupState.deploymentForm.archive?.name).toBe(file.name),
        );
        await action("Add Variable").trigger("click");
        await flushPromises();
        await field("Variable Name").setValue("SECRET");
        await field("Variable Value").setValue("do-not-show-this-value");
        await action("Save Variable", document.querySelector('[role="dialog"]')!).trigger("click");
        await action("Review Deployment").trigger("click");
        await flushPromises();
        expect(document.querySelector('[role="dialog"]')!.textContent).toContain(file.name);
        expect(document.querySelector('[role="dialog"]')!.textContent).not.toContain(
          "do-not-show-this-value",
        );
        await action("Activate Deployment", document.querySelector('[role="dialog"]')!).trigger(
          "click",
        );
        await settle();
        expect((await api.deployments.list(endpoint.id)).data![0]!.bundleFormat).toBe(format);
        const url = `${useNuxtApp().$client.value.endpoint}/v1/function/${endpoint.name}`;
        await vi.waitFor(async () =>
          expect(await (await fetch(url)).text()).toBe("archive-upload"),
        );
        await wrapper.setupState.chooseEndpoint(endpoint.id);
        expect(wrapper.setupState.deploymentForm.variables).toEqual([
          { name: "SECRET", value: "do-not-show-this-value" },
        ]);
        expect(wrapper.text()).not.toContain("do-not-show-this-value");
        await selectRow("SECRET");
        await action("Remove Selected Variable").trigger("click");
        await flushPromises();
        await action("Remove from Draft", document.querySelector('[role="dialog"]')!).trigger(
          "click",
        );
        Object.defineProperty(wrapper.find('input[type="file"]').element, "files", {
          value: [file],
          configurable: true,
        });
        await wrapper.find('input[type="file"]').trigger("change");
        await vi.waitFor(() =>
          expect(wrapper.setupState.deploymentForm.archive?.name).toBe(file.name),
        );
        await action("Review Deployment").trigger("click");
        await flushPromises();
        expect(document.querySelector('[role="dialog"]')!.textContent).toContain(
          "Variables Will Be Removed",
        );
        await action("Activate Deployment", document.querySelector('[role="dialog"]')!).trigger(
          "click",
        );
        await settle();
        expect((await api.deployments.list(endpoint.id)).data![0]!.environmentVariables).toEqual(
          {},
        );
      } finally {
        await api.endpoints.delete(endpoint.id);
      }
    },
  );

  it("deploys file trees and variables, invokes real code, inspects history, and rolls back", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({
      name: `deploy-${crypto.randomUUID()}`,
      runtime: "workerd",
      entrypoint: "index.js",
      isPublic: true,
    });
    try {
      ({ wrapper } = await mountPage(Deployments));
      await selectOption("Endpoint", endpoint.name);
      await settle();
      await action("Source Files").trigger("keydown", { key: "Enter" });
      await field("Source Code").setValue(
        'export default { async fetch(request, env) { console.log("console invocation"); return new Response(env.MESSAGE); } };',
      );
      await action("Add File").trigger("click");
      await field("File Path").setValue("helper.js");
      await field("Source Code").setValue("export const value = 1;");
      await action("Add Variable").trigger("click");
      await flushPromises();
      await field("Variable Name").setValue("MESSAGE");
      await field("Variable Value").setValue("version-one");
      expect(field("Variable Value").attributes("type")).toBe("password");
      await field("Show Variable Value").trigger("click");
      expect(field("Variable Value").attributes("type")).toBe("text");
      await action("Save Variable", document.querySelector('[role="dialog"]')!).trigger("click");
      await flushPromises();
      await field("Override Workerd Compatibility").trigger("click");
      await field("Deployment Compatibility Date").setValue("2026-08-04");
      await field("Override Compatibility Flags").trigger("click");
      wrapper.setupState.deploymentForm.compatibilityFlags = [];
      await nextTick();
      await field("Deployment Compatibility Flags").setValue("nodejs_compat");
      await field("Deployment Compatibility Flags").trigger("keydown", { key: "Enter" });
      await action("Review Deployment").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Review Deployment").trigger("click");
      await flushPromises();
      await action("Activate Deployment", document.querySelector('[role="dialog"]')!).trigger(
        "click",
      );
      await settle();
      expect(wrapper.text()).toContain("Function Deployment Activated");
      const first = (await api.deployments.list(endpoint.id)).data![0]!;
      const url = `${useNuxtApp().$client.value.endpoint}/v1/function/${endpoint.name}`;
      await vi.waitFor(async () => expect(await (await fetch(url)).text()).toBe("version-one"));
      await wrapper.setupState.chooseEndpoint(endpoint.id);
      expect(wrapper.setupState.deploymentForm.variables).toEqual([
        { name: "MESSAGE", value: "version-one" },
      ]);
      expect(wrapper.setupState.deploymentForm.overrideRuntime).toBe(true);
      expect(wrapper.setupState.deploymentForm.overrideFlags).toBe(true);
      await action("Source Files").trigger("keydown", { key: "Enter" });
      await field("Source Code").setValue(
        "export default { async fetch(request, env) { return new Response(env.MESSAGE); } };",
      );
      await action("Add File").trigger("click");
      await field("File Path").setValue("helper.js");
      await field("Source Code").setValue("export const value = 1;");
      await selectRow(first.bundleHash);
      expect(wrapper.text()).toContain("helper.js");
      expect(wrapper.text()).toContain("Deployment Version 1");
      expect(wrapper.text()).toContain("Hidden");
      await field("Show Stored Variable Values").trigger("click");
      expect(wrapper.text()).toContain("version-one");
      await field("Show Stored Variable Values").trigger("click");
      expect(wrapper.text()).toContain("Hidden");
      await action("Close Version Details").trigger("click");
      await selectRow("MESSAGE");
      await action("Edit Selected Variable").trigger("click");
      await flushPromises();
      await field("Variable Value").setValue("version-two");
      await action("Save Variable", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Remove Selected File").trigger("click");
      await flushPromises();
      await action("Remove from Draft", document.querySelector('[role="dialog"]')!).trigger(
        "click",
      );
      await action("Add Variable").trigger("click");
      await flushPromises();
      await field("Variable Name").setValue("TEMP");
      await action("Save Variable", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Remove Selected Variable").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Remove Selected Variable").trigger("click");
      await flushPromises();
      await action("Remove from Draft", document.querySelector('[role="dialog"]')!).trigger(
        "click",
      );
      await field("Override Workerd Compatibility").trigger("click");
      await action("Review Deployment").trigger("click");
      await flushPromises();
      await action("Activate Deployment", document.querySelector('[role="dialog"]')!).trigger(
        "click",
      );
      await settle();
      await vi.waitFor(async () => expect(await (await fetch(url)).text()).toBe("version-two"));
      await selectRow(first.bundleHash);
      await action("Roll Back to Selected Version").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Roll Back to Selected Version").trigger("click");
      await flushPromises();
      await action("Confirm Rollback", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Deployment Rolled Back");
      await vi.waitFor(async () => expect(await (await fetch(url)).text()).toBe("version-one"));
      wrapper.unmount();
      ({ wrapper } = await mountPage(Endpoints));
      await selectRow(endpoint.name);
      await action("Endpoint Details").trigger("click");
      await settle();
      expect(wrapper.find('a[target="_blank"]').exists()).toBe(true);
      wrapper.unmount();
      const router = useNuxtApp().$router;
      await router.push({ path: "/functions/executions", query: { endpoint: endpoint.id } });
      ({ wrapper } = await mountPage(Executions));
      expect(wrapper.text()).toContain("GET");
      await wrapper
        .findAll("tbody tr")
        .find((row) => row.text().includes("GET"))!
        .trigger("click");
      expect(wrapper.text()).toContain("Invocation Details");
      await action("Close Details").trigger("click");
      await selectOption("Endpoint", "All Endpoints");
      await action("Apply Filter").trigger("click");
      await settle();
      await router.push("/");
    } finally {
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("registers domains, shows DNS instructions, and updates TLS settings", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const domain = `console-${crypto.randomUUID()}.example.test`;
    ({ wrapper } = await mountPage(Domains));
    await action("Add Domain").trigger("click");
    await field("Domain").setValue(domain);
    await field("Enable TLS").trigger("click");
    await field("ACME Contact Email").setValue("ops@example.com");
    await action("Save Domain").trigger("click");
    await settle();
    const stored = (await api.domains.list()).data!.find((item) => item.domain === domain)!;
    try {
      expect(wrapper.text()).toContain("DNS and TLS Instructions");
      expect(wrapper.text()).toContain("DNS Not Verified");
      await action("Verify DNS").trigger("click");
      await settle();
      expect(wrapper.setupState.functionControlPlane.selectedDomain.value.status).not.toBe(
        "active",
      );
      await action("Domain Details").trigger("click");
      await settle();
      await field("Enable TLS").trigger("click");
      await field("ACME Contact Email").setValue("tls@example.com");
      await action("Save Domain").trigger("click");
      await settle();
      expect((await api.domains.get(stored.id)).tlsAcmeEmail).toBe("tls@example.com");
      await action("Delete Domain").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Delete Domain").trigger("click");
      await flushPromises();
      await action("Delete Domain", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Domain Deleted");
    } finally {
      if ((await api.domains.list()).data!.some((item) => item.id === stored.id))
        await api.domains.delete(stored.id);
    }
  });

  it("verifies a local ingress domain and attaches and removes endpoint routes", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const config = await api.config.get();
    const endpoint = await api.endpoints.create({ name: `route-${crypto.randomUUID()}` });
    const domain = await api.domains.create({ domain: "127.0.0.2", tlsEnabled: false });
    try {
      await api.config.update({
        ...config,
        customDomainCnameTarget: "127.0.0.2",
        customDomainApexIp: "127.0.0.2",
      });
      expect((await api.domains.verify(domain.id)).status).toBe("active");
      ({ wrapper } = await mountPage(Domains));
      await selectOption("Route Endpoint", endpoint.name);
      await settle();
      await selectOption("Verified Domain", domain.domain);
      await field("Path Prefix").setValue("/api");
      await action("Attach Route").trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Domain Route Attached");
      expect((await api.endpoint_domains.list(endpoint.id)).data).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ pathPrefix: "/api", customDomainId: domain.id }),
        ]),
      );
      await selectRow("/api");
      await action("Remove Selected Route").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Remove Selected Route").trigger("click");
      await flushPromises();
      await action("Remove Route", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Function Domain Route Removed");
    } finally {
      await api.domains.delete(domain.id);
      await api.endpoints.delete(endpoint.id);
      await api.config.update(config);
    }
  });

  it("renders actual endpoint and runner metrics without readonly inputs", async () => {
    ({ wrapper } = await mountPage(Statistics));
    expect(wrapper.text()).toContain("Function Statistics");
    expect(wrapper.findAll("input")).toHaveLength(0);
    const stats = wrapper.setupState.functionControlPlane.stats.value;
    expect(wrapper.text()).toContain(stats.totalEndpoints.toLocaleString());
    expect(wrapper.text()).toContain("workerd Runtime");
    wrapper.setupState.functionControlPlane.stats.value = {
      totalEndpoints: 0,
      runtimes: {
        missing: null,
        unhealthy: {
          available: false,
          activeIsolates: 0,
          memoryUsageMb: 0,
          processPid: 0,
          errorMessage: "Runner failed",
        },
        ready: { available: true, activeIsolates: 2, memoryUsageMb: 4, processPid: 123 },
      },
    };
    await nextTick();
    expect(wrapper.text()).toContain("Runner failed");
    expect(wrapper.text()).toContain("123");
    wrapper.setupState.functionControlPlane.stats.value.runtimes = {};
    await nextTick();
    expect(wrapper.text()).toContain("No Runtime Health Reported");
  });
});
