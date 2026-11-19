import { LayrAPIError } from "layr-client";
import type { FunctionControlPlane, FunctionControlPlaneModels } from "layr-client";

export default function (service: FunctionControlPlane) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const config = ref<FunctionControlPlaneModels.Config | null>(null);
  const stats = ref<FunctionControlPlaneModels.GetStatsResponse | null>(null);
  const endpoints = ref<FunctionControlPlaneModels.ListEndpointsResponse | null>(null);
  const selectedEndpoint = ref<FunctionControlPlaneModels.Endpoint | null>(null);
  const deployments = ref<FunctionControlPlaneModels.ListDeploymentsResponse | null>(null);
  const domains = ref<FunctionControlPlaneModels.ListCustomDomainsResponse | null>(null);
  const selectedDomain = ref<FunctionControlPlaneModels.CustomDomain | null>(null);
  const routes = ref<FunctionControlPlaneModels.ListCustomDomainRoutesResponse | null>(null);
  const executions = ref<FunctionControlPlaneModels.ListExecutionsResponse | null>(null);
  const executionEndpointId = ref("");
  const executionOffset = ref(0);
  const limit = 10;
  async function run(operation: () => Promise<void>, message = "") {
    if (busy.value) return false;
    busy.value = true;
    error.value = "";
    notice.value = "";
    try {
      await operation();
      notice.value = message;
      return true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : "The request failed. Please try again.";
      if (err instanceof LayrAPIError && (err.status === 401 || err.status === 403)) {
        await auth.refresh();
        if (!auth.state.value) await navigateTo("/sign-in");
      }
      return false;
    } finally {
      busy.value = false;
    }
  }
  function load(section: "configuration" | "statistics" | "endpoints" | "domains") {
    return run(async () => {
      if (section === "configuration") config.value = await service.config.get();
      if (section === "statistics") stats.value = await service.stats.get();
      if (section === "endpoints") endpoints.value = await service.endpoints.list();
      if (section === "domains") domains.value = await service.domains.list();
    });
  }
  function saveConfig(input: FunctionControlPlaneModels.Config) {
    return run(async () => {
      config.value = await service.config.update(input);
    }, "Function Configuration Saved");
  }
  function selectEndpoint(id: string, section: "details" | "deployments" | "routes" = "details") {
    return run(async () => {
      selectedEndpoint.value = null;
      deployments.value = null;
      routes.value = null;
      selectedEndpoint.value = await service.endpoints.get(id);
      if (section === "deployments") deployments.value = await service.deployments.list(id);
      if (section === "routes") routes.value = await service.endpoint_domains.list(id);
    });
  }
  function saveEndpoint(input: FunctionControlPlaneModels.CreateEndpointInput, id = "") {
    return run(async () => {
      if (id)
        selectedEndpoint.value = await service.endpoints.update(id, {
          description: input.description,
          runtime: input.runtime,
          entrypoint: input.entrypoint,
          memoryLimitMb: input.memoryLimitMb,
          timeoutSeconds: input.timeoutSeconds,
          isPublic: input.isPublic,
        });
      else selectedEndpoint.value = await service.endpoints.create(input);
      endpoints.value = await service.endpoints.list();
    }, "Function Endpoint Saved");
  }
  function deleteEndpoint(id: string) {
    return run(async () => {
      await service.endpoints.delete(id);
      selectedEndpoint.value = null;
      endpoints.value = await service.endpoints.list();
    }, "Function Endpoint Deleted");
  }
  function deploy(id: string, input: FunctionControlPlaneModels.CreateDeploymentInput) {
    return run(async () => {
      await service.deployments.create(id, input);
      selectedEndpoint.value = await service.endpoints.get(id);
      deployments.value = await service.deployments.list(id);
    }, "Function Deployment Activated");
  }
  function rollback(id: string, version: number) {
    return run(async () => {
      await service.deployments.rollback(id, { targetVersion: version });
      selectedEndpoint.value = await service.endpoints.get(id);
      deployments.value = await service.deployments.list(id);
    }, "Function Deployment Rolled Back");
  }
  function listExecutions(offset = 0) {
    return run(async () => {
      if (executionEndpointId.value)
        executions.value = await service.endpoints.executions(executionEndpointId.value, {
          limit,
          offset,
        });
      else executions.value = await service.executions.list({ limit, offset });
      executionOffset.value = offset;
    });
  }
  function selectDomain(id: string) {
    return run(async () => {
      selectedDomain.value = await service.domains.get(id);
    });
  }
  function saveDomain(input: FunctionControlPlaneModels.CreateCustomDomainInput, id = "") {
    return run(async () => {
      if (id)
        selectedDomain.value = await service.domains.update(id, {
          tlsEnabled: input.tlsEnabled,
          tlsAcmeEmail: input.tlsAcmeEmail,
        });
      else selectedDomain.value = await service.domains.create(input);
      domains.value = await service.domains.list();
    }, "Function Domain Saved");
  }
  function verifyDomain(id: string) {
    return run(async () => {
      selectedDomain.value = await service.domains.verify(id);
      domains.value = await service.domains.list();
    });
  }
  function deleteDomain(id: string) {
    return run(async () => {
      await service.domains.delete(id);
      selectedDomain.value = null;
      domains.value = await service.domains.list();
    }, "Function Domain Deleted");
  }
  function attachRoute(endpointId: string, domainId: string, pathPrefix: string) {
    return run(async () => {
      await service.endpoint_domains.create(endpointId, { customDomainId: domainId, pathPrefix });
      routes.value = await service.endpoint_domains.list(endpointId);
    }, "Function Domain Route Attached");
  }
  function deleteRoute(endpointId: string, id: string) {
    return run(async () => {
      await service.endpoint_domains.delete(endpointId, id);
      routes.value = await service.endpoint_domains.list(endpointId);
    }, "Function Domain Route Removed");
  }
  return {
    busy,
    error,
    notice,
    config,
    stats,
    endpoints,
    selectedEndpoint,
    deployments,
    domains,
    selectedDomain,
    routes,
    executions,
    executionEndpointId,
    executionOffset,
    limit,
    run,
    load,
    saveConfig,
    selectEndpoint,
    saveEndpoint,
    deleteEndpoint,
    deploy,
    rollback,
    listExecutions,
    selectDomain,
    saveDomain,
    verifyDomain,
    deleteDomain,
    attachRoute,
    deleteRoute,
  };
}
