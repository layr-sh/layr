import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UModal } from "#components";
import Endpoints from "../../pages/function/endpoints.vue";
import Domains from "../../pages/function/domains.vue";
import Deployments from "../../pages/function/deployments.vue";
import FunctionExecutions from "../../pages/function/executions.vue";
import Jobs from "../../pages/tasks/jobs.vue";
import TaskExecutions from "../../pages/tasks/executions.vue";
import DLQ from "../../pages/tasks/dead-letter-queue.vue";
import Presets from "../../pages/image/presets.vue";
import Users from "../../pages/control-plane/users.vue";
import Accounts from "../../pages/control-plane/service-accounts.vue";
import Hooks from "../../pages/control-plane/event-hooks.vue";
import Events from "../../pages/control-plane/events.vue";
import {
  action,
  activeDialog,
  flushPromises,
  menuAction,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "./mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let session: Awaited<ReturnType<typeof signInConsoleUser>>;
let endpoint: { id: string; name: string };
let domain: { id: string; domain: string };
let job: { id: string; name: string };
let preset: { id: string; name: string };
let account: { id: string; name: string };
let hook: { id: string | number; name: string };
const executionId = crypto.randomUUID();
const versionId = crypto.randomUUID();
const log = {
  id: executionId,
  executionId,
  status: "failed",
  executedAt: new Date(),
  createdAt: new Date(),
  durationMs: 1,
  executionDurationMs: 1,
  statusCode: 500,
  method: "GET",
  path: "/row-action-test",
  attempts: 1,
  maxAttempts: 1,
  payload: { query: "SELECT 1" },
};
const list = (data: unknown[]) => ({
  data,
  total: data.length,
  count: data.length,
  limit: 10,
  offset: 0,
});

beforeAll(async () => {
  session = await signInConsoleUser();
  const client = useNuxtApp().$client.value;
  endpoint = await client.function.cp.endpoints.create({
    name: `row-endpoint-${crypto.randomUUID()}`,
  });
  domain = await client.function.cp.domains.create({
    domain: `row-${crypto.randomUUID()}.example.test`,
    tlsEnabled: false,
  });
  job = await client.tasks.cp.jobs.create({
    name: `row-job-${crypto.randomUUID()}`,
    cronExpression: "0 0 1 1 *",
    targetType: "sql",
    targetPayload: { query: "SELECT 1" },
    isEnabled: false,
  });
  preset = await client.image.cp.presets.create({
    name: `row-preset-${crypto.randomUUID()}`,
    processingOptions: "w_48",
  });
  account = await session.serviceAccount.core.service_accounts.create({
    name: `row-account-${crypto.randomUUID()}`,
    scopes: ["core:event.read"],
  });
  hook = await session.serviceAccount.core.event_hooks.create({
    name: `row-hook-${crypto.randomUUID()}`,
    driver: "sql",
    eventTypes: ["row.actions.*"],
    sqlFunctionName: "public.row_action_test",
  });
});
afterEach(() => wrapper?.unmount());
afterAll(async () => {
  const client = useNuxtApp().$client.value;
  await client.function.cp.endpoints.delete(endpoint.id);
  await client.function.cp.domains.delete(domain.id);
  await client.tasks.cp.jobs.delete(job.id);
  await client.image.cp.presets.delete(preset.id);
  await session.serviceAccount.core.service_accounts.delete(account.id);
  await session.serviceAccount.core.event_hooks.delete(String(hook.id));
});

describe("Record row shortcuts and modal lifecycle", () => {
  it.each([
    ["function endpoints", Endpoints, (): string => endpoint.name],
    ["function domains", Domains, (): string => domain.domain],
    ["task jobs", Jobs, (): string => job.name],
    ["image presets", Presets, (): string => preset.name],
    ["console users", Users, (): string => session.user.email],
    ["service accounts", Accounts, (): string => account.name],
    ["event hooks", Hooks, (): string => hook.name],
    ["audit events", Events, (): string => ""],
    ["function executions", FunctionExecutions, (): string => log.path],
    ["task executions", TaskExecutions, (): string => executionId],
    ["dead-letter executions", DLQ, (): string => executionId],
    ["deployment versions", Deployments, (): string => "row-version-hash"],
  ] as const)(
    "opens %s directly and allows closing and reopening without selection",
    async (_name, component, text) => {
      ({ wrapper } = await mountPage(component));
      const state = wrapper.setupState;
      if (component === FunctionExecutions)
        state.functionControlPlane.executions.value = list([{ ...log, endpointId: endpoint.id }]);
      if (component === TaskExecutions) state.tasksControlPlane.executions.value = list([log]);
      if (component === DLQ) state.tasksControlPlane.dlq.value = list([log]);
      if (component === Deployments) {
        await state.chooseEndpoint(endpoint.id);
        state.functionControlPlane.deployments.value = list([
          {
            id: versionId,
            version: 1,
            bundleHash: "row-version-hash",
            bundleFormat: "files",
            createdAt: new Date(),
            bundleFiles: {},
            environmentVariables: {},
          },
        ]);
      }
      await nextTick();
      const row = pageDOM()
        .findAll("tbody tr")
        .find((row) => row.text().includes(text()))!;
      expect(row).toBeDefined();
      expect(row.attributes("data-selected")).toBe("false");
      await row.trigger("click");
      await flushPromises();
      await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
      const modal = wrapper.findAllComponents(UModal).find((modal) => modal.props("open"))!;
      expect(modal.props("ui")).toHaveProperty(
        "content",
        expect.stringMatching(/max-w-(?:3|4|5)xl/),
      );
      expect([...activeDialog()!.querySelector('[data-slot="footer"]')!.classList]).toEqual(
        expect.arrayContaining(["flex", "flex-wrap", "justify-end", "gap-3"]),
      );
      expect(modal.props("ui")).toHaveProperty("body", "space-y-6");
      expect(modal.props("ui")).toHaveProperty("footer", "flex flex-wrap justify-end gap-3");
      modal.vm.$emit("update:open", true);
      await nextTick();
      expect(modal.props("open")).toBe(true);
      modal.vm.$emit("update:open", false);
      await flushPromises();
      await vi.waitFor(() => expect(activeDialog()).toBeNull());
      await row.trigger("keydown", { key: "Enter" });
      await flushPromises();
      await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
      if (component === Events || component === TaskExecutions) {
        const control = component === Events ? state.controlPlane : state.tasksControlPlane;
        control.error.value = "A request failed while details were open";
        await nextTick();
        expect(activeDialog()!.textContent).toContain("A request failed while details were open");
      }
      const closeScope =
        component === Events
          ? activeDialog()!.querySelector('[data-slot="footer"]')!
          : activeDialog()!;
      await action("Close", closeScope).trigger("click");
      await vi.waitFor(() => expect(activeDialog()).toBeNull());
    },
  );

  it.each([
    ["deployment", Deployments, "New Deployment", "composing"],
    ["domain route", Domains, "Attach Domain Route", "attachingRoute"],
    ["task execution", TaskExecutions, "New Execution", "triggerOpen"],
  ] as const)("closes and reopens the new %s form", async (_name, component, label, stateKey) => {
    ({ wrapper } = await mountPage(component));
    if (component === Deployments || component === Domains)
      await wrapper.setupState.chooseEndpoint(endpoint.id);
    await action(label).trigger("click");
    await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
    const modal = wrapper.findAllComponents(UModal).find((modal) => modal.props("open"))!;
    const footer = activeDialog()!.querySelector('[data-slot="footer"]')!;
    expect([...footer.classList]).toEqual(
      expect.arrayContaining(["flex", "flex-wrap", "justify-end", "gap-3"]),
    );
    expect(footer.querySelector("button")!.textContent?.trim()).toBe("Cancel");
    expect(footer.querySelectorAll("button")).toHaveLength(2);
    modal.vm.$emit("update:open", true);
    await nextTick();
    expect(wrapper.setupState[stateKey].value).toBe(true);
    await action("Cancel", activeDialog()!).trigger("click");
    await vi.waitFor(() => expect(activeDialog()).toBeNull());
    expect(wrapper.setupState[stateKey].value).toBe(false);
    await action(label).trigger("click");
    await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
    await action("Close", activeDialog()!).trigger("click");
  });

  it.each([
    [Users, "New Console User"],
    [Accounts, "New Service Account"],
    [Hooks, "New Event Hook"],
  ] as const)(
    "keeps %s creation focused and provides footer cancellation",
    async (component, label) => {
      ({ wrapper } = await mountPage(component));
      await action(label).trigger("click");
      await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
      if (component === Users) {
        expect(activeDialog()!.textContent).not.toContain("Active Sessions");
        expect(activeDialog()!.textContent).not.toContain("Reset Password");
      }
      const footer = activeDialog()!.querySelector('[data-slot="footer"]')!;
      await action("Cancel", footer).trigger("click");
      await vi.waitFor(() => expect(activeDialog()).toBeNull());
    },
  );

  it.each([
    [Endpoints, "functionControlPlane", "confirmingDelete"],
    [Presets, "imageControlPlane", "confirmingDelete"],
    [Jobs, "tasksControlPlane", "confirmingDelete"],
    [Jobs, "tasksControlPlane", "confirmingRun"],
    [Domains, "functionControlPlane", "confirmingDelete"],
    [Domains, "functionControlPlane", "confirmingRemove"],
    [Deployments, "functionControlPlane", "confirmingRollback"],
    [DLQ, "tasksControlPlane", "confirmingPurge"],
    [Accounts, "controlPlane", "confirmingDelete"],
    [Hooks, "controlPlane", "confirmingDelete"],
  ] as const)("omits empty confirmation bodies in %s (%s, %s)", async (component, plane, flag) => {
    ({ wrapper } = await mountPage(component));
    wrapper.setupState[flag].value = true;
    await flushPromises();
    await vi.waitFor(() => expect(activeDialog()).not.toBeNull());
    expect(activeDialog()!.querySelector('[data-slot="body"]')).toBeNull();
    expect([...activeDialog()!.querySelector('[data-slot="footer"]')!.classList]).toEqual(
      expect.arrayContaining(["flex", "flex-wrap", "justify-end", "gap-3"]),
    );
    wrapper.setupState[plane].error.value = "The operation failed. Please try again.";
    await nextTick();
    expect(activeDialog()!.querySelector('[data-slot="body"]')!.textContent).toContain(
      "The operation failed",
    );
    wrapper.setupState[plane].error.value = "";
    await nextTick();
    expect(activeDialog()!.querySelector('[data-slot="body"]')).toBeNull();
    await action("Cancel", activeDialog()!).trigger("click");
  });

  it.each([
    ["accounts", Accounts, "Delete Account"],
    ["hooks", Hooks, "Delete Hook"],
  ] as const)(
    "opens a row-specific destructive confirmation for %s without opening its editor",
    async (kind, component, label) => {
      ({ wrapper } = await mountPage(component));
      const name = kind === "accounts" ? account.name : hook.name;
      const row = pageDOM()
        .findAll("tbody tr")
        .find((row) => row.text().includes(name))!;
      await menuAction(label, row.element);
      expect(wrapper.setupState.editorOpen.value).toBe(false);
      expect(activeDialog()!.textContent).toContain(label);
      await action("Cancel", activeDialog()!).trigger("click");
      await vi.waitFor(() => expect(activeDialog()).toBeNull());
    },
  );
});
