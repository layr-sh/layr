import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import useTasksControlPlane from "../../composables/use-tasks-control-plane";
import Layout from "../../pages/tasks.vue";
import Statistics from "../../pages/tasks/index.vue";
import Configuration from "../../pages/tasks/configurations.vue";
import Jobs from "../../pages/tasks/jobs.vue";
import Executions from "../../pages/tasks/executions.vue";
import DLQ from "../../pages/tasks/dead-letter-queue.vue";
import {
  action,
  pageDOM,
  activeDialog,
  menuAction,
  field,
  mountPage,
  signInConsoleUser,
  flushPromises,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
let consoleUser: Awaited<ReturnType<typeof signInConsoleUser>>;
beforeEach(async () => {
  consoleUser = await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.tasksControlPlane.busy.value).toBe(false));
}

describe("Tasks permissions and errors", () => {
  it("gates nested navigation by individual scopes", async () => {
    const router = useNuxtApp().$router;
    for (const [scope, path] of [
      ["tasks:stats.read", "/tasks"],
      ["tasks:job.read", "/tasks/jobs"],
      ["tasks:execution.read", "/tasks/executions"],
      ["tasks:execution.read", "/tasks/dead-letter-queue"],
      ["tasks:config.read", "/tasks/configurations"],
    ]) {
      await signInConsoleUser([scope!]);
      await router.push(path!);
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(true);
      expect(wrapper.setupState.items.value.map((item: { scope: string }) => item.scope)).toEqual(
        scope === "tasks:execution.read" ? [scope, scope] : [scope],
      );
      wrapper.unmount();
      await router.push(scope === "tasks:stats.read" ? "/tasks/jobs" : "/tasks");
      ({ wrapper } = await mountPage(Layout));
      expect(wrapper.setupState.canView.value).toBe(false);
      expect(pageDOM().text()).toContain("Access Restricted");
      wrapper.unmount();
    }
    await router.push("/");
  });

  it("does not fetch pages without their read scopes", async () => {
    await signInConsoleUser([]);
    for (const page of [Configuration, Statistics, Jobs, Executions, DLQ]) {
      ({ wrapper } = await mountPage(page));
      expect(wrapper.setupState.tasksControlPlane.error.value).toBe("");
      expect(pageDOM().findAll('button[type="submit"]')).toHaveLength(page === Executions ? 1 : 0);
      wrapper.unmount();
    }
  });

  it("keeps read-only configuration safe and reports failed saves", async () => {
    await signInConsoleUser(["tasks:config.read"]);
    ({ wrapper } = await mountPage(Configuration));
    expect(field("Worker Concurrency").attributes("disabled")).toBeDefined();
    expect(pageDOM().text()).not.toContain("Save Tasks Config");
    expect(
      await wrapper.setupState.tasksControlPlane.saveConfig(
        wrapper.setupState.tasksControlPlane.config.value,
      ),
    ).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
    wrapper.setupState.tasksControlPlane.config.value = null;
    await wrapper.setupState.save();
    await nextTick();
    expect(pageDOM().text()).toContain("Configuration Unavailable");
    wrapper.setupState.tasksControlPlane.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Tasks Configuration");
    wrapper.setupState.tasksControlPlane.busy.value = false;
  });

  it("reports invalid configuration without losing the form", async () => {
    ({ wrapper } = await mountPage(Configuration));
    expect(
      await wrapper.setupState.tasksControlPlane.saveConfig({
        ...wrapper.setupState.tasksControlPlane.config.value,
        concurrencyLimit: 0,
      }),
    ).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
    expect(pageDOM().find("#tasks-configuration").exists()).toBe(true);
  });

  it("handles unavailable statistics and loading placeholders", async () => {
    await signInConsoleUser([]);
    ({ wrapper } = await mountPage(Statistics));
    expect(wrapper.setupState.metrics.value).toEqual([]);
    expect(pageDOM().text()).toContain("Statistics Unavailable");
    wrapper.setupState.tasksControlPlane.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Tasks Statistics");
    wrapper.setupState.tasksControlPlane.busy.value = false;
    expect(await wrapper.setupState.tasksControlPlane.load("statistics")).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("keeps failed job forms open and handles stale or empty selections", async () => {
    ({ wrapper } = await mountPage(Jobs));
    await wrapper.setupState.editJob();
    await wrapper.setupState.deleteJob();
    await action("New Job").trigger("click");
    expect(pageDOM().text()).toContain("HTTP sends a POST");
    await action("Cancel").trigger("click");
    expect(pageDOM().find("#tasks-job").exists()).toBe(false);
    await action("New Job").trigger("click");
    wrapper.setupState.schedule.value = "custom";
    Object.assign(wrapper.setupState.jobForm, {
      name: `invalid-${crypto.randomUUID()}`,
      cronExpression: "invalid",
    });
    wrapper.setupState.jobForm.target.url = "https://example.com/task";
    await nextTick();
    await wrapper.setupState.saveJob();
    await nextTick();
    expect(pageDOM().find("#tasks-job").exists()).toBe(true);
    expect(wrapper.setupState.validateJob()).toContainEqual(
      expect.objectContaining({
        name: "cronExpression",
        message: expect.stringContaining("Use five cron fields"),
      }),
    );
    wrapper.setupState.jobForm.cronExpression = "0 0 * * *";
    wrapper.setupState.jobForm.target.advanced = true;
    for (const payload of ["{", "null", "[]", '"text"']) {
      wrapper.setupState.jobForm.target.raw = payload;
      await wrapper.setupState.saveJob();
      expect(
        wrapper.setupState
          .validateJob()
          .some((error: { name: string }) => error.name === "target.raw"),
      ).toBe(true);
    }
    wrapper.setupState.selectedJobId.value = crypto.randomUUID();
    await wrapper.setupState.editJob();
    expect(pageDOM().text()).toContain("Request Failed");
    await wrapper.setupState.deleteJob();
    expect(pageDOM().text()).toContain("Request Failed");
    wrapper.setupState.confirmingDelete.value = true;
    await flushPromises();
    await action("Close", activeDialog()!).trigger("click");
  });

  it("shows job details without write controls and records last run metadata", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const job = await api.jobs.create({
      name: `readonly-${crypto.randomUUID()}`,
      cronExpression: "0 0 1 1 *",
      targetType: "http",
      targetPayload: { url: "https://example.com/task" },
      isEnabled: false,
    });
    try {
      await signInConsoleUser(["tasks:job.read"]);
      ({ wrapper } = await mountPage(Jobs));
      const row = pageDOM()
        .findAll("tbody tr")
        .find((row) => row.text().includes(job.name))!;
      await menuAction("Details", row.element);
      await settle();
      expect(field("Name").attributes("disabled")).toBeDefined();
      for (const label of ["New Job", "Delete Job", "Save Job"])
        expect(pageDOM().text()).not.toContain(label);
      wrapper.setupState.tasksControlPlane.selectedJob.value.lastRunAt = new Date(
        "2026-01-01T00:00:00Z",
      );
      await nextTick();
      expect(pageDOM().text()).toContain(new Date("2026-01-01T00:00:00Z").toLocaleString());
    } finally {
      await signInConsoleUser();
      await api.jobs.delete(job.id);
    }
  });

  it("reports invalid trigger payloads and clears previous queue results", async () => {
    ({ wrapper } = await mountPage(Executions));
    if (wrapper.setupState.auth.hasScope("tasks:execution.write"))
      await action("New Execution").trigger("click");
    for (const payload of ["{", "null", "[]", '"text"']) {
      await wrapper.setupState.tasksControlPlane.trigger("", payload);
      expect(pageDOM().text()).toContain("Request Failed");
      expect(wrapper.setupState.tasksControlPlane.triggered.value).toBeNull();
    }
    expect(await wrapper.setupState.tasksControlPlane.trigger(crypto.randomUUID(), "")).toBe(false);
    await field("Filter by Job ID").setValue("invalid");
    await action("Apply Filter").trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("paginates execution logs and displays all recorded and missing details", async () => {
    ({ wrapper } = await mountPage(Executions));
    if (wrapper.setupState.auth.hasScope("tasks:execution.write"))
      await action("New Execution").trigger("click");
    const state = wrapper.setupState.tasksControlPlane;
    const log = {
      id: crypto.randomUUID(),
      executionId: crypto.randomUUID(),
      jobId: crypto.randomUUID(),
      status: "failed",
      executionDurationMs: 10,
      executedAt: new Date(),
      responseStatusCode: 400,
      responseBody: "bad request",
      errorMessage: "failed target",
    };
    state.executions.value = { data: [log], total: 11, count: 1, limit: 10, offset: 0 };
    await nextTick();
    await action("Details").trigger("click");
    expect(pageDOM().text()).toContain("failed target");
    expect(pageDOM().text()).toContain("bad request");
    expect(pageDOM().text()).toContain("400");
    wrapper.setupState.selectedExecution.value.responseBody = undefined;
    await nextTick();
    expect(pageDOM().text()).toContain("Not Recorded");
    await action("Next").trigger("click");
    await settle();
    expect(state.executionOffset.value).toBe(10);
    await action("Previous").trigger("click");
    await settle();
    expect(state.executionOffset.value).toBe(0);
    state.executions.value = { data: [], total: 0, count: 0, limit: 10, offset: 0 };
    state.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Executions");
    state.busy.value = false;
  });

  it("handles stale dead-lettered executions and paginates without losing errors", async () => {
    ({ wrapper } = await mountPage(DLQ));
    await wrapper.setupState.retry();
    await wrapper.setupState.purge();
    const state = wrapper.setupState.tasksControlPlane;
    const item = {
      id: crypto.randomUUID(),
      jobId: crypto.randomUUID(),
      status: "failed",
      attempts: 1,
      maxAttempts: 1,
      runAt: new Date(),
      createdAt: new Date(),
      payload: { query: "SELECT 1" },
    };
    state.dlq.value = { data: [item], total: 11, count: 1, limit: 10, offset: 0 };
    await nextTick();
    await menuAction("Details");
    expect(pageDOM().text()).toContain("Not Recorded");
    await wrapper.setupState.retry();
    expect(pageDOM().text()).toContain("Request Failed");
    expect(wrapper.setupState.selectedExecution.value).not.toBeNull();
    await menuAction("Purge Execution");
    await flushPromises();
    await action("Purge Execution", activeDialog()!).trigger("click");
    await settle();
    expect(activeDialog()).not.toBeNull();
    await action("Close", activeDialog()!).trigger("click");
    await action("Next").trigger("click");
    await settle();
    expect(state.dlqOffset.value).toBe(10);
    await action("Previous").trigger("click");
    await settle();
    expect(state.dlqOffset.value).toBe(0);
    state.dlq.value = { data: [] };
    state.busy.value = true;
    await nextTick();
    expect(pageDOM().text()).toContain("Loading Dead-Letter Queue");
    state.busy.value = false;
  });

  it("hides execution mutations for read-only Service Accounts", async () => {
    await signInConsoleUser(["tasks:execution.read"]);
    ({ wrapper } = await mountPage(Executions));
    if (wrapper.setupState.auth.hasScope("tasks:execution.write"))
      await action("New Execution").trigger("click");
    expect(pageDOM().text()).not.toContain("Trigger Execution");
    wrapper.unmount();
    ({ wrapper } = await mountPage(DLQ));
    expect(pageDOM().text()).not.toContain("Retry Execution");
    expect(pageDOM().text()).not.toContain("Purge Execution");
    expect(await wrapper.setupState.tasksControlPlane.retryDLQ(crypto.randomUUID())).toBe(false);
    expect(pageDOM().text()).toContain("Request Failed");
  });

  it("serializes requests, handles unknown errors, and preserves JSON object validation", async () => {
    const tasksControlPlane = useTasksControlPlane(useNuxtApp().$client.value.tasks.cp);
    expect(
      await tasksControlPlane.run(async () => {
        expect(await tasksControlPlane.load("jobs")).toBe(false);
      }),
    ).toBe(true);
    expect(
      await tasksControlPlane.run(async () => {
        throw "failure";
      }),
    ).toBe(false);
    expect(tasksControlPlane.error.value).toBe("The request failed. Please try again.");
    for (const payload of ["{", "null", "[]", '"text"']) {
      expect(
        await tasksControlPlane.saveJob({
          name: "invalid",
          cronExpression: "0 0 * * *",
          targetType: "http",
          targetPayload: payload,
        }),
      ).toBe(false);
      expect(tasksControlPlane.error.value).not.toBe("");
    }
    expect(
      await tasksControlPlane.saveJob({
        name: `http-${crypto.randomUUID()}`,
        cronExpression: "invalid",
        targetType: "http",
        targetPayload: '{"url":"https://example.com"}',
      }),
    ).toBe(false);
  });

  it("clears expired Console authentication after rejected Tasks requests", async () => {
    ({ wrapper } = await mountPage(Configuration));
    await consoleUser.serviceAccount.console.users.sessions.delete(
      consoleUser.user.id,
      consoleUser.nuxtApp.$auth.state.value!.id,
    );
    expect(await wrapper.setupState.tasksControlPlane.load("configuration")).toBe(false);
    expect(consoleUser.nuxtApp.$auth.state.value).toBeNull();
    expect(consoleUser.nuxtApp.$router.currentRoute.value.path).toBe("/sign-in");
  });
});
