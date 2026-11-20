import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import { UCard, UFormField } from "#components";
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
  selectOption,
  signInConsoleUser,
  flushPromises,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());
async function settle() {
  await flushPromises();
  await vi.waitFor(() => expect(wrapper.setupState.tasksControlPlane.busy.value).toBe(false));
}
function rowAction(text: string, label: string) {
  const row = pageDOM()
    .findAll("tbody tr")
    .find((row) => row.text().includes(text));
  expect(row, `Missing table row: ${text}`).toBeDefined();
  return menuAction(label, row!.element);
}

describe("Tasks service integration", () => {
  it("matches the Image layout with ordered navigation and standard cards", async () => {
    ({ wrapper } = await mountPage(Layout));
    expect(
      pageDOM()
        .findAll('nav[aria-label="Tasks Service Settings"] a')
        .map((link) => link.text()),
    ).toEqual(["Statistics", "Jobs", "Executions", "Dead-Letter Queue", "Configurations"]);
    const router = useNuxtApp().$router;
    for (const path of [
      "/tasks",
      "/tasks/jobs",
      "/tasks/executions",
      "/tasks/dead-letter-queue",
      "/tasks/configurations",
    ]) {
      await router.push(path);
      expect(wrapper.setupState.canView.value).toBe(true);
    }
    for (const page of [Statistics, Configuration, Jobs, Executions, DLQ]) {
      wrapper.unmount();
      ({ wrapper } = await mountPage(page));
      for (const card of wrapper.findAllComponents(UCard)) {
        expect(card.props("title")).toBeTruthy();
        expect(card.props("description")).toBeTruthy();
      }
      for (const button of pageDOM().findAll('button[type="submit"]')) {
        expect(button.element.closest('[data-slot="footer"]'), button.text()).not.toBeNull();
        expect(document.getElementById(button.attributes("form")!)).not.toBeNull();
      }
    }
    await router.push("/");
  });

  it("saves all runtime configuration fields through the API", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const original = await api.config.get();
    ({ wrapper } = await mountPage(Configuration));
    try {
      for (const formField of wrapper.findAllComponents(UFormField))
        expect(formField.props("description")).toBeTruthy();
      for (const [label, value] of [
        ["Worker Concurrency", "12"],
        ["Execution Timeout (Seconds)", "20"],
        ["Poll Interval (Milliseconds)", "500"],
        ["Initial Retry Delay (Seconds)", "2"],
        ["Maximum Retry Delay (Seconds)", "60"],
        ["Maximum Attempts", "3"],
      ]) {
        await field(label!).setValue(value!);
        await field(label!).trigger("blur");
      }
      await action("Save Tasks Config").trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Tasks Configuration Saved");
      expect(await api.config.get()).toEqual({
        concurrencyLimit: 12,
        timeoutSeconds: 20,
        pollIntervalMs: 500,
        retryInitialDelaySeconds: 2,
        retryMaxDelaySeconds: 60,
        retryMaxAttempts: 3,
      });
    } finally {
      await api.config.update(original);
    }
  });

  it("creates, updates, pauses, inspects, and deletes scheduled jobs", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const name = `console-tasks-${crypto.randomUUID()}`;
    ({ wrapper } = await mountPage(Jobs));
    await action("New Job").trigger("click");
    await field("Name").setValue(name);
    await selectOption("Schedule", "Custom cron expression");
    await field("Cron Expression").setValue("0 0 1 1 *");
    await selectOption("Timezone", "Asia/Jakarta");
    await selectOption("Target Type", "SQL Query");
    expect(pageDOM().text()).toContain("SQL Can Change Project Data");
    await field("SQL Query").setValue("SELECT 1");
    await action("Create Job").trigger("click");
    await settle();
    expect(pageDOM().text()).toContain("Task Job Saved");
    const job = ((await api.jobs.list()).data || []).find((job) => job.name === name)!;
    expect(job.isEnabled).toBe(false);
    try {
      await rowAction(name, "Details");
      await settle();
      expect(pageDOM().text()).toContain("Never");
      expect(field("SQL Query").element).toHaveProperty("value", "SELECT 1");
      expect(pageDOM().text()).toContain("Paused — no automatic run scheduled");
      await field("Enable Schedule").trigger("click");
      await action("Save Job").trigger("click");
      await settle();
      expect((await api.jobs.get(job.id)).isEnabled).toBe(true);
      expect(pageDOM().text()).toContain("Active");
      await menuAction("Details");
      await settle();
      await field("Enable Schedule").trigger("click");
      await action("Save Job").trigger("click");
      await settle();
      expect((await api.jobs.get(job.id)).isEnabled).toBe(false);
      await menuAction("Delete Job");
      await flushPromises();
      await action("Cancel", activeDialog()!).trigger("click");
      await menuAction("Delete Job");
      await flushPromises();
      await action("Delete Job", activeDialog()!).trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Task Job Deleted");
      expect(((await api.jobs.list()).data || []).some((item) => item.id === job.id)).toBe(false);
    } finally {
      if (((await api.jobs.list()).data || []).some((item) => item.id === job.id))
        await api.jobs.delete(job.id);
    }
  });

  it("queues standalone and scheduled executions and inspects attempt history", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const job = await api.jobs.create({
      name: `tasks-run-${crypto.randomUUID()}`,
      cronExpression: "0 0 1 1 *",
      timezone: "UTC",
      isEnabled: false,
      targetType: "sql",
      targetPayload: { query: "SELECT 1" },
    });
    try {
      ({ wrapper } = await mountPage(Executions));
      if (wrapper.setupState.auth.hasScope("tasks:execution.write"))
        await action("New Execution").trigger("click");
      await selectOption("Job", job.name);
      await action("Queue Execution").trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Task Execution Queued");
      const id = wrapper.setupState.tasksControlPlane.triggered.value.executionId;
      await vi.waitFor(async () =>
        expect(
          ((await api.executions.list({ jobId: job.id })).data || []).some(
            (log) => log.executionId === id,
          ),
        ).toBe(true),
      );
      await field("Filter by Job ID").setValue(job.id);
      await action("Apply Filter").trigger("click");
      await settle();
      await rowAction(id, "Details");
      expect(pageDOM().text()).toContain("Execution Details");
      expect(pageDOM().text()).toContain("Not Recorded");
      await action("Close").trigger("click");
      expect(pageDOM().text()).not.toContain("Execution Details");
      await selectOption("Execution Mode", "Run a Standalone Task");
      await selectOption("Target Type", "SQL Query");
      await field("SQL Query").setValue("SELECT 1");
      await action("Queue Execution").trigger("click");
      await settle();
      const standaloneId = wrapper.setupState.tasksControlPlane.triggered.value.executionId;
      await vi.waitFor(async () =>
        expect(
          ((await api.executions.list()).data || []).some(
            (log) => log.executionId === standaloneId,
          ),
        ).toBe(true),
      );
      await field("Filter by Job ID").setValue("");
      await action("Apply Filter").trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Standalone");
    } finally {
      await api.jobs.delete(job.id);
    }
  });

  it("retries and purges actual dead-lettered executions", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const original = await api.config.get();
    await api.config.update({ ...original, retryMaxAttempts: 1, pollIntervalMs: 100 });
    const queued = await api.executions.create({
      payload: { url: "http://metadata.google.internal/blocked" },
    });
    try {
      await vi.waitFor(async () =>
        expect(
          ((await api.dlq.list()).data || []).some((item) => item.id === queued.executionId),
        ).toBe(true),
      );
      ({ wrapper } = await mountPage(DLQ));
      await rowAction(queued.executionId, "Details");
      expect(pageDOM().text()).toContain("Failed Execution Details");
      expect(pageDOM().text()).toContain("metadata.google.internal");
      await action("Close").trigger("click");
      await rowAction(queued.executionId, "Retry Execution");
      await settle();
      expect(pageDOM().text()).toContain("Task Execution Requeued");
      expect(wrapper.setupState.selectedExecution.value).toBeNull();
      await vi.waitFor(async () =>
        expect(
          ((await api.dlq.list()).data || []).some((item) => item.id === queued.executionId),
        ).toBe(true),
      );
      await wrapper.setupState.tasksControlPlane.listDLQ();
      await nextTick();
      await rowAction(queued.executionId, "Purge Execution");
      await flushPromises();
      await action("Cancel", activeDialog()!).trigger("click");
      await menuAction("Purge Execution");
      await flushPromises();
      await action("Purge Execution", activeDialog()!).trigger("click");
      await settle();
      expect(pageDOM().text()).toContain("Dead-Lettered Execution Purged");
      expect(
        ((await api.dlq.list()).data || []).some((item) => item.id === queued.executionId),
      ).toBe(false);
    } finally {
      await api.config.update(original);
      if (((await api.dlq.list()).data || []).some((item) => item.id === queued.executionId))
        await api.dlq.purge(queued.executionId);
    }
  });

  it("renders real statistics as metrics instead of readonly inputs", async () => {
    ({ wrapper } = await mountPage(Statistics));
    const stats = wrapper.setupState.tasksControlPlane.stats.value;
    expect(pageDOM().text()).toContain("Tasks Statistics");
    expect(pageDOM().text()).toContain(stats.totalJobs.toLocaleString());
    expect(pageDOM().text()).toContain(`${stats.successRate.toFixed(2)}%`);
    expect(pageDOM().findAll("input")).toHaveLength(0);
    expect(pageDOM().get('[role="progressbar"]').attributes("aria-label")).toBe(
      `Execution Success Rate: ${stats.successRate}%`,
    );
  });
});
