import { Configuration, type ConfigurationParameters } from "../../generated/core/base/runtime";
import {
  buildHeaders,
  createFetchWithRetry,
  normalizeEndpointURL,
  type RetryOptions,
} from "./transport";
export type { ConfigurationParameters } from "../../generated/core/base/runtime";
export type { SchemaClient } from "../services/data";

export interface ClientOptions extends RetryOptions {
  endpoint: string;
  publishableKey?: string;
  sessionToken?: string;
  headers?: Record<string, string>;
}

export interface LayrClient {
  readonly endpoint: string;
  readonly options: ClientOptions;
  readonly config: Configuration;
  readonly headers: Record<string, string>;
  sessionToken?: string;
  setSession(token: string): void;
  clearSession(): void;
  _singletons: Map<string, unknown>;
}

export function createClient(options: ClientOptions): LayrClient {
  const normalizedURL = normalizeEndpointURL(options?.endpoint);
  const activeHeaders = buildHeaders(options);
  const fetchWithRetry = createFetchWithRetry(options);

  const config = new Configuration({
    basePath: normalizedURL,
    headers: activeHeaders,
    fetchApi: fetchWithRetry,
  });

  const client: LayrClient = {
    endpoint: normalizedURL,
    options,
    config,
    headers: activeHeaders,
    sessionToken: options?.sessionToken,
    _singletons: new Map<string, unknown>(),

    setSession(token: string) {
      this.sessionToken = token;
      const cleanToken = token.replace(/^Bearer\s+/i, "").trim();
      if (cleanToken) {
        this.headers["Authorization"] = `Bearer ${cleanToken}`;
      } else {
        delete this.headers["Authorization"];
      }
    },

    clearSession() {
      delete this.sessionToken;
      delete this.headers["Authorization"];
    },
  };

  return client;
}

export const ATTACH_CONTROL_PLANE = Symbol("attachControlPlane");

export interface ElevatableService<TCP = unknown> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: ConfigurationParameters) => TCP;
}

export type Elevated<T> = T extends ElevatableService<infer TCP> ? T & { cp: TCP } : T;

export interface WithServiceAccountOptions {
  serviceAccountKey?: string;
}

export function withServiceAccount<T extends ElevatableService<unknown>>(
  service: T,
  options?: WithServiceAccountOptions,
): Elevated<T> {
  const elevatable = service as ElevatableService<unknown>;
  if (!elevatable || !elevatable.client) {
    throw new Error("Provided service is not a valid Layr service client");
  }

  const elevatedHeaders: Record<string, string> = {
    ...elevatable.client.headers,
  };

  const key = options?.serviceAccountKey?.trim();
  if (key) {
    elevatedHeaders["Service-Account-Key"] = key;
  }

  const elevatedConfig: ConfigurationParameters = {
    basePath: elevatable.client.endpoint,
    headers: elevatedHeaders,
    fetchApi: elevatable.client.config.fetchApi,
  };

  const cp =
    typeof elevatable[ATTACH_CONTROL_PLANE] === "function"
      ? elevatable[ATTACH_CONTROL_PLANE](elevatedConfig)
      : undefined;

  return Object.assign(Object.create(Object.getPrototypeOf(service)), service, {
    cp,
  }) as Elevated<T>;
}
