import { Configuration } from "../../generated/file_storage/control_plane/runtime";
import {
  Configuration as BaseConfiguration,
  type HttpFile,
} from "../../generated/file_storage/base/runtime";
import {
  BucketsApi,
  ConfigApi,
  type FileStorageBucketsListObjectsRequest,
} from "../../generated/file_storage/control_plane/apis";
import {
  ObjectsApi,
  URLsApi,
  type FileStorageObjectsDeleteRequest,
  type FileStorageObjectsDownloadRequest,
  type FileStorageObjectsHeadRequest,
} from "../../generated/file_storage/base/apis";
import type {
  Bucket,
  Config,
  CreateBucketInput,
  ListBucketObjectsResponse,
  ListBucketsResponse,
  UpdateBucketInput,
} from "../../generated/file_storage/control_plane/models";
import type {
  ModelObject,
  PresignURLInput,
  PresignURLResponse,
} from "../../generated/file_storage/base/models";
import {
  ATTACH_CONTROL_PLANE,
  type ConfigurationParameters,
  type ElevatableService,
  type LayrClient,
} from "../core/client";
import { handleApiCall } from "../core/transport";
import { BucketClient } from "../extensions/bucket-client";

export interface FileStorageControlPlane {
  buckets: {
    create: (input: CreateBucketInput) => Promise<Bucket>;
    delete: (bucket: string) => Promise<void>;
    get: (bucket: string) => Promise<Bucket>;
    list: () => Promise<ListBucketsResponse>;
    list_objects: (
      bucket: string,
      params?: Omit<FileStorageBucketsListObjectsRequest, "bucket">,
    ) => Promise<ListBucketObjectsResponse>;
    update: (bucket: string, input: UpdateBucketInput) => Promise<Bucket>;
  };
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
}

export interface FileStorageService extends ElevatableService<FileStorageControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: ConfigurationParameters) => FileStorageControlPlane;

  from: (bucket: string) => BucketClient;

  objects: {
    delete: (
      bucket: string,
      key: string,
      params?: Omit<FileStorageObjectsDeleteRequest, "bucket" | "key">,
    ) => Promise<void>;
    download: (
      bucket: string,
      key: string,
      params?: Omit<FileStorageObjectsDownloadRequest, "bucket" | "key">,
    ) => Promise<HttpFile>;
    head: (
      bucket: string,
      key: string,
      params?: Omit<FileStorageObjectsHeadRequest, "bucket" | "key">,
    ) => Promise<Headers>;
    upload_post: (
      bucket: string,
      key: string,
      body: Blob | ArrayBuffer | string,
    ) => Promise<ModelObject>;
    upload_put: (
      bucket: string,
      key: string,
      body: Blob | ArrayBuffer | string,
      options?: { contentType?: string },
    ) => Promise<ModelObject>;
  };

  url: {
    presign: (input: PresignURLInput) => Promise<PresignURLResponse>;
  };
}

export function useFileStorage(client: LayrClient): FileStorageService {
  const config = new BaseConfiguration(client.config);

  const service: FileStorageService = {
    client,

    from: (bucket: string) => new BucketClient(client, bucket),

    objects: {
      delete: (bucket: string, key: string, params = {}) =>
        handleApiCall(() =>
          new ObjectsApi(config).fileStorageObjectsDelete({ bucket, key, ...params }),
        ),
      download: (bucket: string, key: string, params = {}) =>
        handleApiCall(() =>
          new ObjectsApi(config).fileStorageObjectsDownload({ bucket, key, ...params }),
        ),
      head: (bucket: string, key: string, params = {}) =>
        handleApiCall(async () => {
          const raw = await new ObjectsApi(config).fileStorageObjectsHeadRaw({
            bucket,
            key,
            ...params,
          });
          return raw.raw.headers;
        }),
      upload_post: (bucket: string, key: string, body: Blob | ArrayBuffer | string) =>
        handleApiCall(() =>
          new ObjectsApi(config).fileStorageObjectsUploadPost({
            bucket,
            key,
            body: body as unknown as object,
          }),
        ),
      upload_put: (
        bucket: string,
        key: string,
        body: Blob | ArrayBuffer | string,
        options?: { contentType?: string },
      ) =>
        handleApiCall(() => {
          const contentType =
            options?.contentType ??
            (typeof Blob !== "undefined" && body instanceof Blob && body.type
              ? body.type
              : undefined);
          const initOverrides = contentType
            ? async ({ init }: { init: RequestInit }) => ({
                headers: {
                  ...(init.headers as Record<string, string>),
                  "Content-Type": contentType,
                },
              })
            : undefined;
          return new ObjectsApi(config).fileStorageObjectsUploadPut(
            { bucket, key, body: body as unknown as object },
            initOverrides,
          );
        }),
    },

    url: {
      presign: (input: PresignURLInput) =>
        handleApiCall(() => new URLsApi(config).fileStorageUrlPresign({ presignURLInput: input })),
    },

    [ATTACH_CONTROL_PLANE]: (configuration: ConfigurationParameters): FileStorageControlPlane => {
      const elevatedConfig = new Configuration(configuration);
      return {
        buckets: {
          create: (input: CreateBucketInput) =>
            handleApiCall(() =>
              new BucketsApi(elevatedConfig).fileStorageBucketsCreate({ createBucketInput: input }),
            ),
          delete: (bucket: string) =>
            handleApiCall(() =>
              new BucketsApi(elevatedConfig).fileStorageBucketsDelete({ bucket }),
            ),
          get: (bucket: string) =>
            handleApiCall(() => new BucketsApi(elevatedConfig).fileStorageBucketsGet({ bucket })),
          list: () => handleApiCall(() => new BucketsApi(elevatedConfig).fileStorageBucketsList()),
          list_objects: (bucket: string, params = {}) =>
            handleApiCall(() =>
              new BucketsApi(elevatedConfig).fileStorageBucketsListObjects({ bucket, ...params }),
            ),
          update: (bucket: string, input: UpdateBucketInput) =>
            handleApiCall(() =>
              new BucketsApi(elevatedConfig).fileStorageBucketsUpdate({
                bucket,
                updateBucketInput: input,
              }),
            ),
        },
        config: {
          get: () => handleApiCall(() => new ConfigApi(elevatedConfig).fileStorageConfigGet()),
          update: (input: Config) =>
            handleApiCall(() =>
              new ConfigApi(elevatedConfig).fileStorageConfigUpdate({ config: input }),
            ),
        },
      };
    },
  };

  return service;
}
