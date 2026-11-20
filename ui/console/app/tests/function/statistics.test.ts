import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { nextTick } from "vue";
import { useNuxtApp } from "#app";
import Statistics from "../../pages/function/index.vue";
import {
  action,
  flushPromises,
  mountPage,
  pageDOM,
  signInConsoleUser,
} from "../control-plane/mount-page";

let wrapper: Awaited<ReturnType<typeof mountPage>>["wrapper"];
beforeEach(async () => {
  await signInConsoleUser();
});
afterEach(() => wrapper?.unmount());

it("reports real deployment and execution usage with only the statistics scope", async () => {
  const client = useNuxtApp().$client.value;
  const api = client.function.cp;
  const active = await api.endpoints.create({
    name: `stats-live-${crypto.randomUUID()}`,
    isPublic: true,
  });
  const idle = await api.endpoints.create({
    name: `stats-idle-${crypto.randomUUID()}`,
    isPublic: false,
  });
  try {
    await api.deployments.create(active.id, {
      bundleFormat: "raw",
      bundleContent:
        'export default { fetch(request) { return new Response("statistics", { status: new URL(request.url).pathname.endsWith("/fail") ? 503 : 200 }); } };',
    });
    const url = `${client.endpoint}/v1/function/${active.name}`;
    expect((await fetch(url)).status).toBe(200);
    expect((await fetch(`${url}/fail`)).status).toBe(503);
    await vi.waitFor(async () => {
      const stats = await api.stats.get();
      const usage = stats.endpoints!.find((endpoint) => endpoint.name === active.name)!;
      expect(usage.executions).toBeGreaterThanOrEqual(2);
      expect(usage.failedExecutions).toBeGreaterThanOrEqual(1);
      expect(usage.averageDurationMs).toBeGreaterThanOrEqual(0);
      expect(usage.p95DurationMs).toBeGreaterThanOrEqual(0);
    });
    await signInConsoleUser(["function:stats.read"]);
    ({ wrapper } = await mountPage(Statistics));
    expect(pageDOM().text()).toContain("Last 24 Hours");
    expect(pageDOM().text()).toContain("Failure Rate");
    expect(pageDOM().text()).toContain("HTTP 5xx Responses");
    expect(pageDOM().text()).toContain("not lifetime traffic");
    const rows = pageDOM().findAll("tbody tr");
    const liveRow = rows.find((row) => row.text().includes(active.name))!;
    const idleRow = rows.find((row) => row.text().includes(idle.name))!;
    expect(liveRow.text()).toContain("Public");
    expect(liveRow.text()).toContain("Deployed");
    expect(liveRow.text()).toContain("ms");
    expect(idleRow.text()).toContain("Private");
    expect(idleRow.text()).toContain("No Deployment");
    expect(idleRow.text()).toContain("No Invocations");
    await action("Refresh").trigger("click");
    await flushPromises();
    await vi.waitFor(() => expect(wrapper.setupState.functionControlPlane.busy.value).toBe(false));
    const stats = wrapper.setupState.functionControlPlane.stats.value;
    expect(stats.totalDeployments).toBeGreaterThanOrEqual(1);
    expect(stats.bundleSizeBytes).toBeGreaterThan(0);
    expect(stats.executionsLast24Hours).toBeGreaterThanOrEqual(2);
    expect(stats.generatedAt.getTime() - stats.windowStartedAt.getTime()).toBe(86400000);
    expect(
      stats.endpoints.find((endpoint: { name: string }) => endpoint.name === active.name),
    ).toMatchObject({ isDeployed: true, isPublic: true });
    stats.executionsLast24Hours = 0;
    stats.failedExecutionsLast24Hours = 0;
    stats.averageDurationMs = undefined;
    stats.p95DurationMs = undefined;
    stats.lastExecutedAt = undefined;
    stats.endpoints = null;
    await nextTick();
    expect(wrapper.setupState.traffic.value).toContainEqual({
      label: "Failure Rate",
      value: "No Invocations",
    });
    expect(wrapper.setupState.traffic.value).toContainEqual({
      label: "Average Duration",
      value: "No Invocations",
    });
    expect(wrapper.setupState.traffic.value).toContainEqual({
      label: "P95 Duration",
      value: "No Invocations",
    });
    expect(pageDOM().text()).toContain("Last recorded invocation: No Invocations");
    expect(pageDOM().text()).toContain("No Function endpoints");
  } finally {
    await signInConsoleUser();
    await api.endpoints.delete(active.id);
    await api.endpoints.delete(idle.id);
  }
});
