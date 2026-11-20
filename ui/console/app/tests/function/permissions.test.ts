import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UFileUpload, UModal } from "#components";
import Layout from "../../pages/function.vue";
import Statistics from "../../pages/function/index.vue";
import Configuration from "../../pages/function/configurations.vue";
import Endpoints from "../../pages/function/endpoints.vue";
import Deployments from "../../pages/function/deployments.vue";
import Executions from "../../pages/function/executions.vue";
import Domains from "../../pages/function/domains.vue";
import useFunctionControlPlane from "../../composables/use-function-control-plane";
import {
  action,
  pageDOM,
  activeDialog,
  menuAction,
  field,
  flushPromises,
  mountPage,
  selectOption,
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
  await vi.waitFor(() => expect(wrapper.setupState.functionControlPlane.busy.value).toBe(false), {
    timeout: 90000,
  });
}

describe("Function permissions and failures", () => {
  it("filters navigation and prevents fetching sections without read scopes", async () => {
    const router = useNuxtApp().$router;
    for (const [scope, path] of [
      ["function:stats.read", "/function"],
      ["function:endpoint.read", "/function/endpoints"],
      ["function:config.read", "/function/configurations"],
    ]) {
      await signInConsoleUser([scope!]);
      await router.push(path!);
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(true);
      expect(
        wrapper.setupState.items.value.every((item: { scope: string }) => item.scope === scope),
      ).toBe(true);
      wrapper.unmount();
      await router.push(scope === "function:stats.read" ? "/function/endpoints" : "/function");
      ({ wrapper } = await mountPage(Layout));
      expect(pageDOM().text()).toContain("Access Restricted");
      wrapper.unmount();
    }
    await router.push("/");
    await signInConsoleUser([]);
    for (const page of [Statistics, Configuration, Endpoints, Deployments, Executions, Domains]) {
      ({ wrapper } = await mountPage(page));
      expect(wrapper.setupState.functionControlPlane.error.value).toBe("");
      wrapper.unmount();
    }
  });

  it("keeps configuration unavailable and read-only states safe", async () => {
    await signInConsoleUser(["function:config.read"]);
    ({ wrapper } = await mountPage(Configuration));
    expect(field("Default Runtime").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save Function Config");
    expect(
      await wrapper.setupState.functionControlPlane.saveConfig(
        wrapper.setupState.functionControlPlane.config.value,
      ),
    ).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
    await wrapper.setupState.save();
    expect(pageDOM().text()).toContain("Request Failed");
    const state = wrapper.setupState.functionControlPlane;
    state.config.value = null;
    await nextTick();
    await wrapper.setupState.save();
    expect(wrapper.setupState.validateConfig()).toEqual([]);
    expect(pageDOM().text()).toContain("Configuration Unavailable");
    state.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Function Configuration");
    await wrapper.setupState.load();
    expect(wrapper.setupState.allowedKeys.value).toEqual([]);
    expect(wrapper.setupState.compatibilityFlags.value).toEqual([]);
    expect(wrapper.setupState.bundleSizeMib.value).toBe(0);
    wrapper.setupState.bundleSizeMib.value = 1;
    expect(state.config.value).toBeNull();
    state.busy.value = false;
    wrapper.unmount();
    await signInConsoleUser([]);
    ({ wrapper } = await mountPage(Statistics));
    expect(wrapper.setupState.runtimes.value).toEqual([]);
    expect(wrapper.setupState.summary.value).toEqual([]);
    expect(wrapper.setupState.traffic.value).toEqual([]);
    wrapper.setupState.functionControlPlane.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Function Statistics");
    wrapper.setupState.functionControlPlane.busy.value = false;
    expect(await wrapper.setupState.functionControlPlane.load("statistics")).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("validates every configuration field before submission", async () => {
    ({ wrapper } = await mountPage(Configuration));
    const config = wrapper.setupState.functionControlPlane.config.value;
    Object.assign(config, {
      defaultRuntime: "",

      defaultTimeoutSeconds: 1.5,
      maxBundleSizeBytes: -1,
      defaultTlsAcmeEmail: "bad",
      customDomainCnameTarget: "https://bad",
      customDomainApexIp: "bad",
    });
    config.workerdRuntime.compatibilityDate = "";
    wrapper.setupState.allowedKeys.value = ["BAD KEY"];
    wrapper.setupState.compatibilityFlags.value = ["--bad flag"];
    await wrapper.setupState.save();
    expect(pageDOM().text()).toContain("Only the workerd runtime is available");
    expect(pageDOM().text()).toContain("Use environment variable names");
    expect(pageDOM().text()).toContain("Use compatibility flag names");
    expect(pageDOM().text()).toContain("Choose a valid compatibility date");
    config.customDomainApexIp = "300.1.1.1";
    expect(
      wrapper.setupState
        .validateConfig()
        .some((error: { name: string }) => error.name === "customDomainApexIp"),
    ).toBe(true);
    config.customDomainApexIp = "::1";
    expect(
      wrapper.setupState
        .validateConfig()
        .some((error: { name: string }) => error.name === "customDomainApexIp"),
    ).toBe(false);
    config.customDomainApexIp = "bad::ip";
    expect(
      wrapper.setupState
        .validateConfig()
        .some((error: { name: string }) => error.name === "customDomainApexIp"),
    ).toBe(true);
    Object.assign(config, {
      defaultRuntime: "workerd",
      defaultMemoryLimitMb: 128,
      defaultTimeoutSeconds: 30,
      maxBundleSizeBytes: 1000,
      defaultTlsAcmeEmail: "",
      customDomainCnameTarget: "localhost",
      customDomainApexIp: "",
    });
    config.workerdRuntime.compatibilityDate = "2026-08-04";
    wrapper.setupState.allowedKeys.value = [" API_URL "];
    wrapper.setupState.compatibilityFlags.value = [" nodejs_compat "];
    expect(wrapper.setupState.validateConfig()).toEqual([]);
  });

  it("validates endpoints, preserves failed forms, and handles stale selections", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({ name: `existing-${crypto.randomUUID()}` });
    try {
      ({ wrapper } = await mountPage(Endpoints));
      expect(wrapper.setupState.invocationUrl.value).toBe("");
      await wrapper.setupState.editEndpoint();
      await wrapper.setupState.deleteEndpoint();
      await wrapper.setupState.saveEndpoint();
      await action("New Endpoint").trigger("click");
      await action("Cancel").trigger("click");
      await action("New Endpoint").trigger("click");
      await field("Name").setValue("bad name");
      await field("Entrypoint").setValue("../bad.js");
      await field("Customize Resource Settings").trigger("click");
      wrapper.setupState.endpointForm.memoryLimitMb = 0;
      wrapper.setupState.endpointForm.timeoutSeconds = 0;
      await wrapper.setupState.saveEndpoint();
      expect(pageDOM().text()).toContain("Use a relative entrypoint");
      expect(pageDOM().text()).toContain("Enter a positive whole number");
      await field("Name").setValue(endpoint.name);
      await field("Entrypoint").setValue("index.js");
      wrapper.setupState.endpointForm.memoryLimitMb = 128;
      wrapper.setupState.endpointForm.timeoutSeconds = 30;
      await action("Create Endpoint").trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Request Failed");
      expect(pageDOM().find("#function-endpoint").exists()).toBe(true);
      wrapper.setupState.selectedEndpointId.value = crypto.randomUUID();
      await wrapper.setupState.editEndpoint();
      expect(pageDOM().text()).toContain("Request Failed");
      await menuAction("Delete Endpoint");
      wrapper.setupState.selectedEndpointId.value = crypto.randomUUID();
      await flushPromises();
      await action("Delete Endpoint", activeDialog()!).trigger("click");
      await settle();
      expect(activeDialog()).not.toBeNull();
      await action("Close", activeDialog()!).trigger("click");
      wrapper.setupState.functionControlPlane.endpoints.value = null;
      wrapper.setupState.functionControlPlane.busy.value = true;
      await nextTick();
      expect(pageDOM().text()).toContain("Loading Endpoints");
      wrapper.setupState.functionControlPlane.busy.value = false;
    } finally {
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("does not expose endpoint, deployment, or domain mutation controls to readers", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({
      name: `reader-${crypto.randomUUID()}`,
      isPublic: false,
    });
    const domain = await api.domains.create({
      domain: `reader-${crypto.randomUUID()}.example.test`,
      tlsEnabled: false,
    });
    try {
      await signInConsoleUser(["function:endpoint.read"]);
      ({ wrapper } = await mountPage(Endpoints));
      await menuAction(
        "Details",
        pageDOM()
          .findAll("tbody tr")
          .find((row) => row.text().includes(endpoint.name))!.element,
      );
      await settle();
      expect(field("Description").attributes("disabled")).toBeDefined();
      for (const label of ["New Endpoint", "Save Endpoint", "Delete Endpoint"])
        expect(pageDOM().text()).not.toContain(label);
      wrapper.unmount();
      ({ wrapper } = await mountPage(Deployments));
      await selectOption("Endpoint", endpoint.name);
      await settle();
      if (
        wrapper.setupState.composing &&
        wrapper.setupState.auth.hasScope("function:endpoint.deploy")
      )
        await action("New Deployment").trigger("click");
      expect(pageDOM().text()).not.toContain("New Deployment");
      wrapper.unmount();
      ({ wrapper } = await mountPage(Domains));
      await menuAction(
        "Details",
        pageDOM()
          .findAll("tbody tr")
          .find((row) => row.text().includes(domain.domain))!.element,
      );
      await settle();
      expect(field("ACME Contact Email").attributes("disabled")).toBeDefined();
      await selectOption("Route Endpoint", endpoint.name);
      await settle();
      for (const label of [
        "Save Domain",
        "New Domain",
        "Delete Domain",
        "Verify DNS",
        "Attach Route",
        "Remove Route",
      ])
        expect(pageDOM().text()).not.toContain(label);
    } finally {
      await signInConsoleUser();
      await api.domains.delete(domain.id);
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("keeps deployment and rollback failures visible in their confirmations", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({
      name: `failed-deploy-${crypto.randomUUID()}`,
      entrypoint: "main.js",
    });
    const router = useNuxtApp().$router;
    try {
      await router.push({ path: "/function/deployments", query: { endpoint: endpoint.id } });
      ({ wrapper } = await mountPage(Deployments));
      await action("New Deployment").trigger("click");
      await action("Source Files").trigger("keydown", { key: "Enter" });
      expect(field("File Path").element).toHaveProperty("value", "main.js");
      await action("Review Deployment").trigger("click");
      await flushPromises();
      expect(pageDOM().text()).toContain("Include a non-empty entrypoint");
      await wrapper.setupState.reviewDeployment();
      await field("Source Code").setValue("export default {");
      await action("Review Deployment").trigger("click");
      await flushPromises();

      await action("Activate Deployment", activeDialog()!).trigger("click");
      await settle();
      expect(activeDialog()!.textContent).toContain("Deployment Failed");
      await action("Close", activeDialog()!).trigger("click");

      wrapper.setupState.selectedDeployment.value = {
        id: crypto.randomUUID(),
        version: 999,
        bundleFiles: null,
        environmentVariables: null,
      };
      await nextTick();
      wrapper.setupState.confirmingRollback.value = true;
      await flushPromises();
      await action("Confirm Rollback", activeDialog()!).trigger("click");
      await settle();
      expect(activeDialog()!.textContent).toContain("Rollback Failed");
      await action("Close", activeDialog()!).trigger("click");
      await wrapper.setupState.chooseEndpoint("");
      expect(wrapper.setupState.validateDeployment().length).toBeGreaterThan(0);
      await wrapper.setupState.reviewDeployment();
      await wrapper.setupState.deploy();
      wrapper.setupState.selectedDeployment.value = null;
      await wrapper.setupState.rollback();
      await wrapper.setupState.chooseEndpoint(endpoint.id);
      if (wrapper.setupState.composing) await action("New Deployment").trigger("click");
      wrapper.setupState.deploymentForm.files[0].content = "";
      await wrapper.setupState.deploy();
      await wrapper.setupState.chooseEndpoint(crypto.randomUUID());
      expect(pageDOM().text()).toContain("Request Failed");
    } finally {
      await router.push("/");
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("handles archive selection, rejection, read failures, and stale reads without publishing", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({ name: `upload-errors-${crypto.randomUUID()}` });
    try {
      ({ wrapper } = await mountPage(Deployments));
      await selectOption("Endpoint", endpoint.name);
      await settle();
      if (
        wrapper.setupState.composing &&
        wrapper.setupState.auth.hasScope("function:endpoint.deploy")
      )
        await action("New Deployment").trigger("click");
      await action("Review Deployment").trigger("click");
      await flushPromises();
      expect(pageDOM().text()).toContain("Select a ZIP or TAR bundle");
      await wrapper.setupState.selectArchive(new File(["bytes"], "bundle.txt"));
      await nextTick();
      expect(wrapper.setupState.readingArchive.value).toBe(false);
      expect(pageDOM().text()).toContain("Choose a ZIP");
      wrapper.findComponent(UFileUpload).vm.$emit("reject", [new File(["bytes"], "bundle.tgz")]);
      await nextTick();
      expect(wrapper.setupState.archiveFile.value).toBeNull();
      wrapper.setupState.functionControlPlane.config.value.maxBundleSizeBytes = 3;
      await wrapper.setupState.selectArchive(new File(["four"], "bundle.zip"));
      expect(pageDOM().text()).toContain("service limit");
      wrapper.setupState.functionControlPlane.config.value.maxBundleSizeBytes = 100;
      const failedFile = new File(["four"], "broken.zip");
      vi.spyOn(failedFile, "arrayBuffer").mockRejectedValueOnce(new Error("Unable to read file"));
      await wrapper.setupState.selectArchive(failedFile);
      expect(pageDOM().text()).toContain("Unable to read file");
      vi.spyOn(failedFile, "arrayBuffer").mockRejectedValueOnce("Read rejected");
      await wrapper.setupState.selectArchive(failedFile);
      expect(pageDOM().text()).toContain("Read rejected");
      const delayedFile = new File(["four"], "delayed.zip");
      let complete!: (buffer: ArrayBuffer) => void;
      vi.spyOn(delayedFile, "arrayBuffer").mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            complete = resolve;
          }),
      );
      const pending = wrapper.setupState.selectArchive(delayedFile);
      await nextTick();
      expect(action("Review Deployment").attributes("disabled")).toBeDefined();
      await wrapper.setupState.reviewDeployment();
      await wrapper.setupState.deploy();
      await wrapper.setupState.selectArchive(undefined);
      complete(new TextEncoder().encode("four").buffer);
      await pending;
      expect(wrapper.setupState.deploymentForm.archive).toBeNull();
      let reject!: (reason: Error) => void;
      vi.spyOn(delayedFile, "arrayBuffer").mockImplementationOnce(
        () =>
          new Promise((_resolve, rejectRead) => {
            reject = rejectRead;
          }),
      );
      const stale = wrapper.setupState.selectArchive(delayedFile);
      await wrapper.setupState.selectArchive(new File(["new"], "current.tar"));
      reject(new Error("Stale read failure"));
      await stale;
      expect(wrapper.setupState.deploymentForm.archive.name).toBe("current.tar");
      expect(wrapper.setupState.archiveError.value).toBe("");
      wrapper.setupState.deploymentForm.overrideRuntime = true;
      wrapper.setupState.deploymentForm.compatibilityDate = "2025-01-01";
      wrapper.setupState.deploymentForm.overrideFlags = true;
      wrapper.setupState.deploymentForm.compatibilityFlags = [];
      await action("Review Deployment").trigger("click");
      await flushPromises();
      expect(activeDialog()!.textContent).toContain("No explicit flags");
      await action("Close", activeDialog()!).trigger("click");
      await action("Clear Archive").trigger("click");
      expect(wrapper.setupState.deploymentForm.archive).toBeNull();
    } finally {
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("edits selected draft files and validates variable dialogs without exposing values", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({ name: `draft-editor-${crypto.randomUUID()}` });
    try {
      await signInConsoleUser(["function:endpoint.read", "function:endpoint.deploy"]);
      ({ wrapper } = await mountPage(Deployments));
      await selectOption("Endpoint", endpoint.name);
      await settle();
      if (
        wrapper.setupState.composing &&
        wrapper.setupState.auth.hasScope("function:endpoint.deploy")
      )
        await action("New Deployment").trigger("click");
      expect(pageDOM().text()).toContain("service size limits apply");
      await wrapper.setupState.editVariable();
      wrapper.setupState.removingDraftItem.value = "file";
      await wrapper.setupState.removeDraftItem();
      wrapper.setupState.removingDraftItem.value = "variable";
      await wrapper.setupState.removeDraftItem();
      await action("Source Files").trigger("keydown", { key: "Enter" });
      wrapper.setupState.deploymentForm.files = [];
      await nextTick();
      expect(pageDOM().text()).toContain("No Source Files");
      await action("Add File").trigger("click");
      await field("File Path").setValue("index.js");
      await field("Source Code").setValue("export default {};");
      await action("Add File").trigger("click");
      expect(wrapper.setupState.fileOptions.value[1].label).toBe("Untitled file");
      await field("File Path").setValue("middle.js");
      await action("Add File").trigger("click");
      await field("File Path").setValue("last.js");
      await selectOption("File", "middle.js");
      expect(wrapper.setupState.selectedFileIndex.value).toBe(1);
      expect(field("File Path").element).toHaveProperty("value", "middle.js");
      await action("Remove File").trigger("click");
      await flushPromises();
      const modal = wrapper
        .findAllComponents(UModal)
        .find((component) => component.props("title") === "Remove from Deployment Draft?")!;
      modal.vm.$emit("update:open", true);
      expect(wrapper.setupState.removingDraftItem.value).toBe("file");
      await action("Close", activeDialog()!).trigger("click");
      await action("Remove File").trigger("click");
      await flushPromises();
      await action("Remove from Draft", activeDialog()!).trigger("click");
      expect(
        wrapper.setupState.deploymentForm.files.map((file: { path: string }) => file.path),
      ).toEqual(["index.js", "last.js"]);
      await action("Add Variable").trigger("click");
      await flushPromises();
      await field("Variable Name").setValue("bad name");
      await wrapper.setupState.saveVariable();
      expect(activeDialog()!.textContent).toContain("Enter a unique environment variable name");
      wrapper.setupState.functionControlPlane.config.value = {
        allowedEnvironmentKeys: ["ALLOWED"],
      };
      await field("Variable Name").setValue("DENIED");
      await action("Save Variable", activeDialog()!).trigger("click");
      await flushPromises();
      expect(activeDialog()!.textContent).toContain("not in the service environment-key allowlist");
      await field("Variable Name").setValue("ALLOWED");
      await field("Variable Value").setValue("private-value");
      await action("Save Variable", activeDialog()!).trigger("click");
      await vi.waitFor(() => expect(wrapper.setupState.editingVariable.value).toBe(false));
      expect(wrapper.setupState.deploymentForm.variables).toEqual([
        { name: "ALLOWED", value: "private-value" },
      ]);
      expect(pageDOM().text()).not.toContain("private-value");
      await action("Add Variable").trigger("click");
      await flushPromises();
      await field("Variable Name").setValue("ALLOWED");
      await action("Save Variable", activeDialog()!).trigger("click");
      await flushPromises();
      expect(activeDialog()!.textContent).toContain("Enter a unique environment variable name");
      await action("Cancel", activeDialog()!).trigger("click");
      await action("Add Variable").trigger("click");
      await flushPromises();
      await action("Close", activeDialog()!).trigger("click");
      wrapper.setupState.functionControlPlane.config.value = null;
      wrapper.setupState.variableForm.name = "FREE_KEY";
      expect(wrapper.setupState.validateVariable()).toEqual([]);
    } finally {
      await signInConsoleUser();
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("validates domain and route forms, missing selections, and failed destructive actions", async () => {
    const api = useNuxtApp().$client.value.function.cp;
    const endpoint = await api.endpoints.create({ name: `domain-error-${crypto.randomUUID()}` });
    const domain = await api.domains.create({
      domain: `duplicate-${crypto.randomUUID()}.example.test`,
      tlsEnabled: false,
    });
    try {
      ({ wrapper } = await mountPage(Domains));
      await wrapper.setupState.editDomain();
      await wrapper.setupState.verifyDomain();
      await wrapper.setupState.deleteDomain();
      await wrapper.setupState.attachRoute();
      await wrapper.setupState.deleteRoute();
      await wrapper.setupState.saveDomain();
      await action("New Domain").trigger("click");
      await action("Cancel").trigger("click");
      await action("New Domain").trigger("click");
      await field("Domain").setValue("https://invalid");
      await field("ACME Contact Email").setValue("invalid");
      await wrapper.setupState.saveDomain();
      expect(pageDOM().text()).toContain("Enter a hostname");
      await field("Domain").setValue(domain.domain);
      await field("ACME Contact Email").setValue("");
      await action("Save Domain").trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Request Failed");
      wrapper.setupState.domainId.value = domain.id;
      await wrapper.setupState.editDomain();
      wrapper.setupState.functionControlPlane.selectedDomain.value.tlsErrorMessage =
        "Certificate failed";
      wrapper.setupState.functionControlPlane.selectedDomain.value.requiredDnsRecords = undefined;
      wrapper.setupState.functionControlPlane.selectedDomain.value.status = "active";
      await nextTick();
      expect(pageDOM().text()).toContain("Certificate failed");
      wrapper.setupState.domainId.value = crypto.randomUUID();
      await wrapper.setupState.editDomain();
      await wrapper.setupState.verifyDomain();
      await menuAction("Delete Domain");
      wrapper.setupState.domainId.value = crypto.randomUUID();
      await flushPromises();
      await action("Delete Domain", activeDialog()!).trigger("click");
      await settle();
      expect(activeDialog()!.textContent).toContain("Deletion Failed");
      await action("Close", activeDialog()!).trigger("click");
      await selectOption("Route Endpoint", endpoint.name);
      await settle();
      await action("Attach Domain Route").trigger("click");
      await wrapper.setupState.attachRoute();
      expect(pageDOM().text()).toContain("Choose a DNS-verified domain");
      for (const prefix of ["bad", "/bad path", "/bad?query", "/" + "a".repeat(255)]) {
        wrapper.setupState.routeForm.pathPrefix = prefix;
        expect(
          wrapper.setupState
            .validateRoute()
            .some((error: { name: string }) => error.name === "pathPrefix"),
        ).toBe(true);
      }
      wrapper.setupState.selectedRouteId.value = crypto.randomUUID();
      await nextTick();
      wrapper.setupState.confirmingRemove.value = true;
      await flushPromises();
      await action("Remove Route", activeDialog()!).trigger("click");
      await settle();
      expect(activeDialog()!.textContent).toContain("Removal Failed");
      await action("Close", activeDialog()!).trigger("click");
      await wrapper.setupState.chooseEndpoint("");
      wrapper.setupState.functionControlPlane.domains.value = null;
      expect(wrapper.setupState.verifiedDomains.value).toEqual([]);
      wrapper.setupState.functionControlPlane.busy.value = true;
      await nextTick();
      expect(pageDOM().text()).toContain("Loading Domains");
      wrapper.setupState.functionControlPlane.busy.value = false;
    } finally {
      await api.domains.delete(domain.id);
      await api.endpoints.delete(endpoint.id);
    }
  });

  it("paginates logs and safely displays missing and recorded execution output", async () => {
    ({ wrapper } = await mountPage(Executions));
    const state = wrapper.setupState.functionControlPlane;
    state.executions.value = {
      data: [
        {
          id: crypto.randomUUID(),
          endpointId: crypto.randomUUID(),
          method: "POST",
          path: "/test",
          statusCode: 500,
          durationMs: 1,
          stdout: "<script>not HTML</script>",
          stderr: "error output",
          errorMessage: "Invocation failed",
          createdAt: new Date(),
        },
      ],
      total: 11,
      count: 1,
      offset: 0,
      limit: 10,
    };
    await nextTick();
    await action("Details").trigger("click");
    expect(pageDOM().text()).toContain("Invocation failed");
    expect(pageDOM().text()).toContain("error output");
    expect(pageDOM().text()).toContain("<script>not HTML</script>");
    expect(pageDOM().findAll("script")).toHaveLength(0);
    wrapper.setupState.selectedExecution.value.stdout = "";
    wrapper.setupState.selectedExecution.value.stderr = "";
    wrapper.setupState.selectedExecution.value.errorMessage = undefined;
    await nextTick();
    expect(pageDOM().text()).toContain("No standard output");
    expect(pageDOM().text()).toContain("No standard error");
    state.executions.value.data[0].statusCode = 100;
    await nextTick();
    await action("Next").trigger("click");
    await settle();
    expect(state.executionOffset.value).toBe(10);
    await action("Previous").trigger("click");
    await settle();
    expect(state.executionOffset.value).toBe(0);
    state.executions.value = { data: [], total: 0 };
    state.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Executions");
    state.busy.value = false;
    wrapper.setupState.filterForm.endpoint = "invalid";
    await wrapper.setupState.applyFilter();
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("serializes operations, handles unexpected errors, and clears expired authentication", async () => {
    const functionControlPlane = useFunctionControlPlane(useNuxtApp().$client.value.function.cp);
    expect(
      await functionControlPlane.run(async () => {
        expect(await functionControlPlane.load("endpoints")).toBe(false);
      }),
    ).toBe(true);
    expect(
      await functionControlPlane.run(async () => {
        throw "failure";
      }),
    ).toBe(false);
    expect(functionControlPlane.error.value).toBe("The request failed. Please try again.");
    ({ wrapper } = await mountPage(Configuration));
    await consoleUser.serviceAccount.console.users.sessions.delete(
      consoleUser.user.id,
      consoleUser.nuxtApp.$auth.state.value!.id,
    );
    expect(await wrapper.setupState.functionControlPlane.load("configuration")).toBe(false);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
    expect(consoleUser.nuxtApp.$router.currentRoute.value.path).toBe("/sign-in");
  });
});
