import type { Configuration } from '../../generated/runtime';
import {
  ConfigApi,
  DeploymentsApi,
  DomainsApi,
  EndpointDomainsApi,
  EndpointsApi,
  ExecutionsApi,
  StatsApi,
  type FunctionEndpointsExecutionsRequest,
  type FunctionExecutionsListRequest,
} from '../../generated/apis';
import type {
  Config,
  CreateCustomDomainInput,
  CreateCustomDomainRouteInput,
  CreateDeploymentInput,
  CreateEndpointInput,
  CustomDomain,
  CustomDomainRoute,
  Deployment,
  Endpoint,
  GetStatsResponse,
  ListCustomDomainRoutesResponse,
  ListCustomDomainsResponse,
  ListDeploymentsResponse,
  ListEndpointsResponse,
  ListExecutionsResponse,
  RollbackDeploymentInput,
  UpdateCustomDomainInput,
  UpdateEndpointInput,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import { handleApiCall } from '../core/transport';

export interface FunctionControlPlane {
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  deployments: {
    create: (endpointId: string, input: CreateDeploymentInput) => Promise<Deployment>;
    list: (endpointId: string) => Promise<ListDeploymentsResponse>;
    rollback: (endpointId: string, input: RollbackDeploymentInput) => Promise<Deployment>;
  };
  domains: {
    create: (input: CreateCustomDomainInput) => Promise<CustomDomain>;
    delete: (domainId: string) => Promise<void>;
    get: (domainId: string) => Promise<CustomDomain>;
    list: () => Promise<ListCustomDomainsResponse>;
    update: (domainId: string, input: UpdateCustomDomainInput) => Promise<CustomDomain>;
    verify: (domainId: string) => Promise<CustomDomain>;
  };
  endpoint_domains: {
    create: (endpointId: string, input: CreateCustomDomainRouteInput) => Promise<CustomDomainRoute>;
    delete: (endpointId: string, routeId: string) => Promise<void>;
    list: (endpointId: string) => Promise<ListCustomDomainRoutesResponse>;
  };
  endpoints: {
    create: (input: CreateEndpointInput) => Promise<Endpoint>;
    delete: (endpointId: string) => Promise<void>;
    executions: (endpointId: string, params?: Omit<FunctionEndpointsExecutionsRequest, 'endpointId'>) => Promise<ListExecutionsResponse>;
    get: (endpointId: string) => Promise<Endpoint>;
    list: () => Promise<ListEndpointsResponse>;
    update: (endpointId: string, input: UpdateEndpointInput) => Promise<Endpoint>;
  };
  executions: {
    list: (params?: FunctionExecutionsListRequest) => Promise<ListExecutionsResponse>;
  };
  stats: {
    get: () => Promise<GetStatsResponse>;
  };
}

export interface FunctionService extends ElevatableService<FunctionControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => FunctionControlPlane;
}

export function useFunction(client: LayrClient): FunctionService {
  const service: FunctionService = {
    client,

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration): FunctionControlPlane => ({
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).functionConfigGet()),
        update: (input: Config) =>
          handleApiCall(() => new ConfigApi(elevatedConfig).functionConfigUpdate({ config: input })),
      },
      deployments: {
        create: (endpointId: string, input: CreateDeploymentInput) =>
          handleApiCall(() =>
            new DeploymentsApi(elevatedConfig).functionDeploymentsCreate({
              endpointId,
              createDeploymentInput: input,
            })
          ),
        list: (endpointId: string) =>
          handleApiCall(() =>
            new DeploymentsApi(elevatedConfig).functionDeploymentsList({ endpointId })
          ),
        rollback: (endpointId: string, input: RollbackDeploymentInput) =>
          handleApiCall(() =>
            new DeploymentsApi(elevatedConfig).functionDeploymentsRollback({
              endpointId,
              rollbackDeploymentInput: input,
            })
          ),
      },
      domains: {
        create: (input: CreateCustomDomainInput) =>
          handleApiCall(() =>
            new DomainsApi(elevatedConfig).functionDomainsCreate({
              createCustomDomainInput: input,
            })
          ),
        delete: (domainId: string) =>
          handleApiCall(async () => {
            await new DomainsApi(elevatedConfig).functionDomainsDelete({ domainId });
          }),
        get: (domainId: string) =>
          handleApiCall(() => new DomainsApi(elevatedConfig).functionDomainsGet({ domainId })),
        list: () => handleApiCall(() => new DomainsApi(elevatedConfig).functionDomainsList()),
        update: (domainId: string, input: UpdateCustomDomainInput) =>
          handleApiCall(() =>
            new DomainsApi(elevatedConfig).functionDomainsUpdate({
              domainId,
              updateCustomDomainInput: input,
            })
          ),
        verify: (domainId: string) =>
          handleApiCall(() =>
            new DomainsApi(elevatedConfig).functionDomainsVerify({
              domainId,
              body: null,
            })
          ),
      },
      endpoint_domains: {
        create: (endpointId: string, input: CreateCustomDomainRouteInput) =>
          handleApiCall(() =>
            new EndpointDomainsApi(elevatedConfig).functionEndpointDomainsCreate({
              endpointId,
              createCustomDomainRouteInput: input,
            })
          ),
        delete: (endpointId: string, routeId: string) =>
          handleApiCall(() =>
            new EndpointDomainsApi(elevatedConfig).functionEndpointDomainsDelete({
              endpointId,
              routeId,
            })
          ),
        list: (endpointId: string) =>
          handleApiCall(() =>
            new EndpointDomainsApi(elevatedConfig).functionEndpointDomainsList({ endpointId })
          ),
      },
      endpoints: {
        create: (input: CreateEndpointInput) =>
          handleApiCall(() =>
            new EndpointsApi(elevatedConfig).functionEndpointsCreate({
              createEndpointInput: input,
            })
          ),
        delete: (endpointId: string) =>
          handleApiCall(() =>
            new EndpointsApi(elevatedConfig).functionEndpointsDelete({ endpointId })
          ),
        executions: (endpointId: string, params = {}) =>
          handleApiCall(() =>
            new EndpointsApi(elevatedConfig).functionEndpointsExecutions({
              endpointId,
              ...params,
            })
          ),
        get: (endpointId: string) =>
          handleApiCall(() =>
            new EndpointsApi(elevatedConfig).functionEndpointsGet({ endpointId })
          ),
        list: () => handleApiCall(() => new EndpointsApi(elevatedConfig).functionEndpointsList()),
        update: (endpointId: string, input: UpdateEndpointInput) =>
          handleApiCall(() =>
            new EndpointsApi(elevatedConfig).functionEndpointsUpdate({
              endpointId,
              updateEndpointInput: input,
            })
          ),
      },
      executions: {
        list: (params = {}) =>
          handleApiCall(() =>
            new ExecutionsApi(elevatedConfig).functionExecutionsList(params)
          ),
      },
      stats: {
        get: () => handleApiCall(() => new StatsApi(elevatedConfig).functionStatsGet()),
      },
    }),
  };

  return service;
}
