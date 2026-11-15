import { Configuration } from "../../generated/core/control_plane/runtime";
import { Configuration as BaseConfiguration } from "../../generated/core/base/runtime";
import {
  EventHooksApi,
  EventHooksDeliveriesApi,
  EventsApi,
  ServiceAccountsApi,
  type CoreEventHooksListRequest,
  type CoreEventsListRequest,
} from "../../generated/core/control_plane/apis";
import { ManifestApi } from "../../generated/core/base/apis";
import type {
  CreateEventHookInput,
  CreateServiceAccountInput,
  CreateServiceAccountResponse,
  Event,
  EventHook,
  EventHookDelivery,
  ListEventsResponse,
  ServiceAccount,
  UpdateEventHookInput,
  UpdateServiceAccountInput,
} from "../../generated/core/control_plane/models";
import type { GetManifestResponse } from "../../generated/core/base/models";
import {
  ATTACH_CONTROL_PLANE,
  type ConfigurationParameters,
  type ElevatableService,
  type LayrClient,
} from "../core/client";
import { handleApiCall } from "../core/transport";

export interface CoreControlPlane {
  event_hooks: {
    create: (input: CreateEventHookInput) => Promise<EventHook>;
    delete: (eventHookId: string) => Promise<void>;
    get: (eventHookId: string) => Promise<EventHook>;
    list: (params?: CoreEventHooksListRequest) => Promise<Array<EventHook>>;
    update: (eventHookId: string, input: UpdateEventHookInput) => Promise<EventHook>;
    deliveries: {
      list: (eventHookId: string) => Promise<Array<EventHookDelivery>>;
      retry: (eventHookId: string, deliveryId: string) => Promise<void>;
    };
  };
  events: {
    get: (eventId: string) => Promise<Event>;
    list: (params?: CoreEventsListRequest) => Promise<ListEventsResponse>;
  };
  service_accounts: {
    create: (input: CreateServiceAccountInput) => Promise<CreateServiceAccountResponse>;
    delete: (serviceAccountId: string) => Promise<void>;
    get: (serviceAccountId: string) => Promise<ServiceAccount>;
    list: () => Promise<Array<ServiceAccount>>;
    update: (serviceAccountId: string, input: UpdateServiceAccountInput) => Promise<ServiceAccount>;
  };
}

export interface CoreService extends ElevatableService<CoreControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: ConfigurationParameters) => CoreControlPlane;

  manifest: () => Promise<GetManifestResponse>;
}

export function useCore(client: LayrClient): CoreService {
  const config = new BaseConfiguration(client.config);

  const service: CoreService = {
    client,

    manifest: () => handleApiCall(() => new ManifestApi(config).coreManifest()),

    [ATTACH_CONTROL_PLANE]: (configuration: ConfigurationParameters): CoreControlPlane => {
      const elevatedConfig = new Configuration(configuration);
      return {
        event_hooks: {
          create: (input: CreateEventHookInput) =>
            handleApiCall(() =>
              new EventHooksApi(elevatedConfig).coreEventHooksCreate({
                createEventHookInput: input,
              }),
            ),
          delete: (eventHookId: string) =>
            handleApiCall(async () => {
              await new EventHooksApi(elevatedConfig).coreEventHooksDelete({ eventHookId });
            }),
          get: (eventHookId: string) =>
            handleApiCall(() =>
              new EventHooksApi(elevatedConfig).coreEventHooksGet({ eventHookId }),
            ),
          list: (params = {}) =>
            handleApiCall(() => new EventHooksApi(elevatedConfig).coreEventHooksList(params)),
          update: (eventHookId: string, input: UpdateEventHookInput) =>
            handleApiCall(() =>
              new EventHooksApi(elevatedConfig).coreEventHooksUpdate({
                eventHookId,
                updateEventHookInput: input,
              }),
            ),
          deliveries: {
            list: (eventHookId: string) =>
              handleApiCall(() =>
                new EventHooksDeliveriesApi(elevatedConfig).coreEventHooksDeliveriesList({
                  eventHookId,
                }),
              ),
            retry: (eventHookId: string, deliveryId: string) =>
              handleApiCall(async () => {
                await new EventHooksDeliveriesApi(elevatedConfig).coreEventHooksDeliveriesRetry({
                  eventHookId,
                  deliveryId,
                  body: {},
                });
              }),
          },
        },
        events: {
          get: (eventId: string) =>
            handleApiCall(() => new EventsApi(elevatedConfig).coreEventsGet({ eventId })),
          list: (params = {}) =>
            handleApiCall(() => new EventsApi(elevatedConfig).coreEventsList(params)),
        },
        service_accounts: {
          create: (input: CreateServiceAccountInput) =>
            handleApiCall(() =>
              new ServiceAccountsApi(elevatedConfig).coreServiceAccountsCreate({
                createServiceAccountInput: input,
              }),
            ),
          delete: (serviceAccountId: string) =>
            handleApiCall(async () => {
              await new ServiceAccountsApi(elevatedConfig).coreServiceAccountsDelete({
                serviceAccountId,
              });
            }),
          get: (serviceAccountId: string) =>
            handleApiCall(() =>
              new ServiceAccountsApi(elevatedConfig).coreServiceAccountsGet({ serviceAccountId }),
            ),
          list: () =>
            handleApiCall(() => new ServiceAccountsApi(elevatedConfig).coreServiceAccountsList()),
          update: (serviceAccountId: string, input: UpdateServiceAccountInput) =>
            handleApiCall(() =>
              new ServiceAccountsApi(elevatedConfig).coreServiceAccountsUpdate({
                serviceAccountId,
                updateServiceAccountInput: input,
              }),
            ),
        },
      };
    },
  };

  return service;
}
