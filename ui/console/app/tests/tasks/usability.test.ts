import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import Jobs from "../../pages/tasks/jobs.vue";
import Executions from "../../pages/tasks/executions.vue";
import Configuration from "../../pages/tasks/configurations.vue";
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
  await vi.waitFor(() => expect(wrapper.setupState.tasksControlPlane.busy.value).toBe(false));
}

describe("Tasks guided forms", () => {
  it("lets advanced users paste a target without filling the structured fields first", async () => {
    ({ wrapper } = await mountPage(Jobs));
    await action("New Job").trigger("click");
    await field("Advanced JSON").trigger("click");
    await field("Target Payload (JSON)").setValue(
      '{"url":"https://example.com/pasted","body":{"sync":true}}',
    );
    await field("Advanced JSON").trigger("click");
    expect(field("Webhook URL").element).toHaveProperty("value", "https://example.com/pasted");
    expect(JSON.parse((field("Request Body (JSON)").element as HTMLTextAreaElement).value)).toEqual(
      { sync: true },
    );
  });
  it("creates HTTP targets without raw payloads and preserves settings across editor modes", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    ({ wrapper } = await mountPage(Jobs));
    await wrapper.setupState.saveJob();
    await action("New Job").trigger("click");
    const name = `guided-http-${crypto.randomUUID()}`;
    await field("Name").setValue(name);
    await field("Webhook URL").setValue("https://example.com/task");
    await field("Request Headers").setValue("X-Source: console\nAuthorization: Bearer test");
    await field("Request Body (JSON)").setValue('{"sync":true}');
    await action("Create Job").trigger("click");
    await settle();
    expect(wrapper.text()).toContain("Saved Paused");
    const job = ((await api.jobs.list()).data || []).find((job) => job.name === name)!;
    try {
      expect(job.targetPayload).toEqual({
        url: "https://example.com/task",
        headers: { "X-Source": "console", Authorization: "Bearer test" },
        body: { sync: true },
      });
      await action(`Select ${name}`).trigger("click");
      expect(wrapper.setupState.selectedJobId.value).toBe(job.id);
      await action(`Select ${name}`).trigger("click");
      expect(wrapper.setupState.selectedJobId.value).toBe("");
      await wrapper
        .findAll("tbody tr")
        .find((row) => row.text().includes(name))!
        .trigger("click");
      await action("Job Details").trigger("click");
      await settle();
      await field("Advanced JSON").trigger("click");
      expect(
        JSON.parse((field("Target Payload (JSON)").element as HTMLTextAreaElement).value),
      ).toEqual(job.targetPayload);
      await field("Target Payload (JSON)").setValue(
        JSON.stringify({ ...job.targetPayload, custom: "preserve" }),
      );
      await field("Advanced JSON").trigger("click");
      expect(field("Webhook URL").element).toHaveProperty("value", "https://example.com/task");
      await action("Save Job").trigger("click");
      await settle();
      expect((await api.jobs.get(job.id)).targetPayload).toEqual({
        ...job.targetPayload,
        custom: "preserve",
      });
      await action("New Job").trigger("click");
      await field("Name").setValue(name);
      await field("Webhook URL").setValue("https://example.com/duplicate");
      await action("Create Job").trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Request Failed");
      expect(wrapper.find("#tasks-job").exists()).toBe(true);
    } finally {
      await api.jobs.delete(job.id);
    }
  });

  it("shows supported schedule summaries and inline cron, name, and timezone errors", async () => {
    ({ wrapper } = await mountPage(Jobs));
    await action("New Job").trigger("click");
    await field("Name").setValue("schedule-validation");
    await field("Webhook URL").setValue("https://example.com/task");
    await selectOption("Schedule", "Every 15 minutes");
    expect(wrapper.text()).toContain("minutes 0, 15, 30, and 45");
    expect(wrapper.setupState.jobForm.cronExpression).toBe("*/15 * * * *");
    await selectOption("Schedule", "Custom cron expression");
    await field("Cron Expression").setValue("invalid");
    await action("Create Job").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("Use five cron fields");
    await field("Cron Expression").setValue("0 9 * * 1-5");
    expect(wrapper.text()).toContain("Custom cron: 0 9 * * 1-5");
    await field("Name").setValue(" ");
    wrapper.setupState.jobForm.timezone = "Bogus/Timezone";
    await wrapper.setupState.saveJob();
    expect(wrapper.text()).toContain("Enter a job name");
    expect(wrapper.text()).toContain("Choose a valid IANA timezone");
    wrapper.setupState.jobForm.timezone = "";
    expect(
      wrapper.setupState.validateJob().some((error: { name: string }) => error.name === "timezone"),
    ).toBe(true);
    wrapper.setupState.jobForm.timezone = "UTC";
    await field("Enable Schedule").trigger("click");
    expect(wrapper.text()).toContain("The scheduler will queue future runs");
    await action("Cancel").trigger("click");
  });

  it("keeps invalid target input while switching between structured and advanced modes", async () => {
    ({ wrapper } = await mountPage(Executions));
    await wrapper.setupState.queueExecution();
    expect(wrapper.text()).toContain("Choose a job or enter a valid job UUID");
    await selectOption("Execution Mode", "Run a Standalone Task");
    await action("Queue Execution").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("Enter a complete HTTP or HTTPS URL");
    await field("Request Headers").setValue("invalid header line");
    await field("Advanced JSON").trigger("click");
    expect(wrapper.text()).toContain("Your input has been kept");
    expect(
      wrapper.findAll("textarea").some((textarea) => textarea.attributes("name") === "target.raw"),
    ).toBe(false);
    await field("Webhook URL").setValue("https://example.com/task");
    await field("Request Headers").setValue("X-Valid: true");
    await field("Request Body (JSON)").setValue("{");
    await field("Advanced JSON").trigger("click");
    expect(wrapper.text()).toContain("Target Needs Attention");
    await field("Request Body (JSON)").setValue('{"valid":true}');
    await field("Advanced JSON").trigger("click");
    await field("Target Payload (JSON)").setValue("{");
    await field("Advanced JSON").trigger("click");
    expect(wrapper.setupState.triggerForm.target.advanced).toBe(true);
    await field("Target Payload (JSON)").setValue(
      '{"url":"https://example.com/updated","body":null}',
    );
    await field("Advanced JSON").trigger("click");
    expect(field("Webhook URL").element).toHaveProperty("value", "https://example.com/updated");
    expect(field("Request Body (JSON)").element).toHaveProperty("value", "null");
    await selectOption("Target Type", "SQL Query");
    await field("SQL Query").setValue("SELECT $1");
    await field("SQL Parameters (JSON)").setValue("{}");
    await wrapper.setupState.queueExecution();
    expect(wrapper.text()).toContain("Parameters must be a JSON array");
    await field("SQL Parameters (JSON)").setValue("[42]");
    expect(wrapper.setupState.validateExecution()).toEqual([]);
  });

  it("queues saved-job payload overrides without modifying the job", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const job = await api.jobs.create({
      name: `override-${crypto.randomUUID()}`,
      cronExpression: "0 0 1 1 *",
      targetType: "sql",
      targetPayload: { query: "SELECT 1" },
      isEnabled: true,
    });
    try {
      ({ wrapper } = await mountPage(Executions));
      wrapper.setupState.chooseJob("missing");
      await selectOption("Job", job.name);
      expect(wrapper.text()).toContain("Active schedule");
      await field("Override Saved Payload").trigger("click");
      expect(wrapper.text()).toContain("REPLACES");
      expect(field("Target Type").attributes("disabled")).toBeDefined();
      await field("SQL Query").setValue("SELECT $1::integer");
      await field("SQL Parameters (JSON)").setValue("[42]");
      await field("Advanced JSON").trigger("click");
      await action("Queue Execution").trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Task Execution Queued");
      expect((await api.jobs.get(job.id)).targetPayload).toEqual({ query: "SELECT 1" });
      const id = wrapper.setupState.tasksControlPlane.triggered.value.executionId;
      await vi.waitFor(async () =>
        expect(
          ((await api.executions.list({ jobId: job.id })).data || []).some(
            (log) => log.executionId === id && log.status === "success",
          ),
        ).toBe(true),
      );
    } finally {
      await api.jobs.delete(job.id);
    }
  });

  it("runs paused jobs once with confirmation and keeps failed confirmations open", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const job = await api.jobs.create({
      name: `run-now-${crypto.randomUUID()}`,
      cronExpression: "0 0 1 1 *",
      targetType: "sql",
      targetPayload: { query: "SELECT 1" },
      isEnabled: false,
    });
    try {
      ({ wrapper } = await mountPage(Jobs));
      await wrapper.setupState.runJob();
      await action(`Select ${job.name}`).trigger("click");
      await action("Run Now").trigger("click");
      await flushPromises();
      await action("Cancel", document.querySelector('[role="dialog"]')!).trigger("click");
      await action("Run Now").trigger("click");
      await flushPromises();
      await action("Queue Execution", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Execution Queued, Not Yet Completed");
      expect((await api.jobs.get(job.id)).isEnabled).toBe(false);
      expect(wrapper.find('a[href$="/tasks/executions"]').exists()).toBe(true);
      wrapper.setupState.selectedJobId.value = crypto.randomUUID();
      await action("Run Now").trigger("click");
      await flushPromises();
      await action("Queue Execution", document.querySelector('[role="dialog"]')!).trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Request Failed");
      expect(document.querySelector('[role="dialog"]')).not.toBeNull();
      await action("Close", document.querySelector('[role="dialog"]')!).trigger("click");
    } finally {
      await api.jobs.delete(job.id);
    }
  });

  it("allows execution-only Service Accounts to use a known job ID without listing jobs", async () => {
    const api = useNuxtApp().$client.value.tasks.cp;
    const job = await api.jobs.create({
      name: `limited-${crypto.randomUUID()}`,
      cronExpression: "0 0 1 1 *",
      targetType: "sql",
      targetPayload: { query: "SELECT 1" },
      isEnabled: false,
    });
    try {
      await signInConsoleUser(["tasks:execution.write"]);
      ({ wrapper } = await mountPage(Executions));
      expect(wrapper.text()).toContain("Your Service Account cannot list jobs");
      await field("Job ID").setValue(job.id);
      await field("Override Saved Payload").trigger("click");
      await selectOption("Target Type", "SQL Query");
      await field("SQL Query").setValue("SELECT 2");
      await action("Queue Execution").trigger("click");
      await settle();
      expect(wrapper.text()).toContain("Task Execution Queued");
      expect(wrapper.setupState.tasksControlPlane.jobs.value).toBeNull();
    } finally {
      await signInConsoleUser();
      await api.jobs.delete(job.id);
    }
  });

  it("validates numeric config limits and retry-delay relationships inline", async () => {
    ({ wrapper } = await mountPage(Configuration));
    const config = wrapper.setupState.tasksControlPlane.config.value;
    config.concurrencyLimit = 0;
    config.timeoutSeconds = 1.5;
    config.pollIntervalMs = 60001;
    config.retryInitialDelaySeconds = 10;
    config.retryMaxDelaySeconds = 5;
    await wrapper.setupState.save();
    expect(wrapper.text()).toContain("Enter a whole number");
    expect(wrapper.text()).toContain(
      "Maximum retry delay must be at least the initial retry delay",
    );
    expect(wrapper.setupState.tasksControlPlane.error.value).toBe("");
    wrapper.setupState.tasksControlPlane.config.value = null;
    await nextTick();
    expect(wrapper.setupState.validateConfig()).toEqual([]);
  });
});
