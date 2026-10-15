import type { Configuration, HttpFile } from '../../generated/runtime';
import {
  ConfigApi,
  InfoApi,
  PresetsApi,
  StatsApi,
  TransformApi,
  URLsApi,
  type ImageInfoGetRequest,
  type ImageTransformGetRequest,
} from '../../generated/apis';
import type {
  Config,
  CreatePresetInput,
  GetInfoResponse,
  GetStatsResponse,
  ListPresetsResponse,
  Preset,
  SignURLInput,
  SignURLResponse,
  UpdatePresetInput,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import { handleApiCall } from '../core/transport';
import { ImageURLBuilder } from '../extensions/image-url-builder';

export interface ImageControlPlane {
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  presets: {
    create: (input: CreatePresetInput) => Promise<Preset>;
    delete: (presetId: string) => Promise<void>;
    get: (presetId: string) => Promise<Preset>;
    list: () => Promise<ListPresetsResponse>;
    update: (presetId: string, input: UpdatePresetInput) => Promise<Preset>;
  };
  stats: {
    get: () => Promise<GetStatsResponse>;
  };
  url: {
    sign: (input: SignURLInput) => Promise<SignURLResponse>;
  };
}

export interface ImageService extends ElevatableService<ImageControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => ImageControlPlane;

  url: (source: string) => ImageURLBuilder;

  info: {
    get: (params: ImageInfoGetRequest) => Promise<GetInfoResponse>;
    post: (body: Blob | ArrayBuffer | string) => Promise<GetInfoResponse>;
  };

  transform: {
    get: (params: ImageTransformGetRequest) => Promise<HttpFile>;
  };
}

export function useImage(client: LayrClient): ImageService {
  const config = client.config;

  const service: ImageService = {
    client,

    url: (source: string) => new ImageURLBuilder(client, source),

    info: {
      get: (params: ImageInfoGetRequest) =>
        handleApiCall(() => new InfoApi(config).imageInfoGet(params)),
      post: (body: Blob | ArrayBuffer | string) =>
        handleApiCall(() => new InfoApi(config).imageInfoPost({ body: body as unknown as object })),
    },

    transform: {
      get: (params: ImageTransformGetRequest) =>
        handleApiCall(() => new TransformApi(config).imageTransformGet(params)),
    },

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration): ImageControlPlane => ({
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).imageConfigGet()),
        update: (input: Config) =>
          handleApiCall(() => new ConfigApi(elevatedConfig).imageConfigUpdate({ config: input })),
      },
      presets: {
        create: (input: CreatePresetInput) =>
          handleApiCall(() =>
            new PresetsApi(elevatedConfig).imagePresetsCreate({ createPresetInput: input })
          ),
        delete: (presetId: string) =>
          handleApiCall(() => new PresetsApi(elevatedConfig).imagePresetsDelete({ presetId })),
        get: (presetId: string) =>
          handleApiCall(() => new PresetsApi(elevatedConfig).imagePresetsGet({ presetId })),
        list: () => handleApiCall(() => new PresetsApi(elevatedConfig).imagePresetsList()),
        update: (presetId: string, input: UpdatePresetInput) =>
          handleApiCall(() =>
            new PresetsApi(elevatedConfig).imagePresetsUpdate({
              presetId,
              updatePresetInput: input,
            })
          ),
      },
      stats: {
        get: () => handleApiCall(() => new StatsApi(elevatedConfig).imageStatsGet()),
      },
      url: {
        sign: (input: SignURLInput) =>
          handleApiCall(() => new URLsApi(elevatedConfig).imageUrlSign({ signURLInput: input })),
      },
    }),
  };

  return service;
}
