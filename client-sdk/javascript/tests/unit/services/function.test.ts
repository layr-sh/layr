import { describe, it, expect } from "vitest";
import { useFunction } from "../../../src/services/function";
import { createMockServiceContext } from "./helpers";

describe("Function Service Unit Tests", () => {
  it("exercises function control plane (.cp) endpoints", async () => {
    const { client, elevate, getLastUrl, getLastInit, serviceAccountKey } =
      createMockServiceContext();
    const func = elevate(useFunction(client));

    // config
    await func.cp.config.get();
    expect(getLastUrl()).toContain("/v1/_/function/config");
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": serviceAccountKey });

    await func.cp.config.update({});
    expect(getLastUrl()).toContain("/v1/_/function/config");

    // deployments
    await func.cp.deployments.create("ep-1", { sourceCode: 'export default () => "ok";' });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/deploy");

    await func.cp.deployments.list("ep-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/deployments");

    await func.cp.deployments.rollback("ep-1", { deploymentId: "dep-old" });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/rollback");

    // domains
    await func.cp.domains.create({ domain: "api.example.com" });
    expect(getLastUrl()).toContain("/v1/_/function/domains");

    await func.cp.domains.get("dom-1");
    expect(getLastUrl()).toContain("/v1/_/function/domains/dom-1");

    await func.cp.domains.list();
    expect(getLastUrl()).toContain("/v1/_/function/domains");

    await func.cp.domains.update("dom-1", {});
    expect(getLastUrl()).toContain("/v1/_/function/domains/dom-1");

    await func.cp.domains.verify("dom-1");
    expect(getLastUrl()).toContain("/v1/_/function/domains/dom-1/verify");

    await func.cp.domains.delete("dom-1");
    expect(getLastUrl()).toContain("/v1/_/function/domains/dom-1");

    // endpoint_domains
    await func.cp.endpoint_domains.create("ep-1", { domainId: "dom-1", path: "/hello" });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/domains");

    await func.cp.endpoint_domains.list("ep-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/domains");

    await func.cp.endpoint_domains.delete("ep-1", "rt-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/domains/rt-1");

    // endpoints
    await func.cp.endpoints.create({ name: "my-fn", handler: "index.handler" });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints");

    await func.cp.endpoints.get("ep-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1");

    await func.cp.endpoints.list();
    expect(getLastUrl()).toContain("/v1/_/function/endpoints");

    await func.cp.endpoints.update("ep-1", { name: "my-fn-renamed" });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1");

    await func.cp.endpoints.executions("ep-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/executions");

    await func.cp.endpoints.executions("ep-1", { limit: 15 });
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1/executions");

    await func.cp.endpoints.delete("ep-1");
    expect(getLastUrl()).toContain("/v1/_/function/endpoints/ep-1");

    // executions
    await func.cp.executions.list();
    expect(getLastUrl()).toContain("/v1/_/function/executions");

    await func.cp.executions.list({ limit: 10 });
    expect(getLastUrl()).toContain("/v1/_/function/executions");

    // stats
    await func.cp.stats.get();
    expect(getLastUrl()).toContain("/v1/_/function/stats");
  });
});
