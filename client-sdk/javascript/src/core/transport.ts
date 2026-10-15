import {
  LayrAPIError,
  type ErrorResponse,
  type OAuthErrorResponse,
  type LayrAPIErrorResponse,
} from './errors';

export * from './errors';

export function normalizeEndpointURL(url?: string): string {
  if (!url || typeof url !== 'string') {
    return 'http://localhost:5297';
  }
  const trimmed = url.trim();
  if (trimmed === '') {
    return 'http://localhost:5297';
  }
  return trimmed.replace(/\/+$/, '');
}

export interface HeaderOptions {
  publishableKey?: string;
  sessionToken?: string;
  serviceAccountKey?: string;
  headers?: Record<string, string>;
}

export function buildHeaders(options: HeaderOptions = {}): Record<string, string> {
  const headers: Record<string, string> = {};

  if (options.headers) {
    for (const [key, val] of Object.entries(options.headers)) {
      if (val !== undefined && val !== null) {
        headers[key] = String(val);
      }
    }
  }

  if (options.publishableKey) {
    headers['Publishable-Key'] = options.publishableKey;
  }

  if (options.sessionToken) {
    const cleanToken = options.sessionToken.replace(/^Bearer\s+/i, '').trim();
    if (cleanToken) {
      headers['Authorization'] = `Bearer ${cleanToken}`;
    }
  }

  if (options.serviceAccountKey) {
    headers['Service-Account-Key'] = options.serviceAccountKey;
  }

  return headers;
}

export interface RetryOptions {
  maxRetries?: number;
  initialDelayMs?: number;
  maxDelayMs?: number;
  timeoutMs?: number;
  fetch?: typeof fetch;
}

export function parseRetryAfter(header: string | null): number | null {
  if (!header) return null;
  const trimmed = header.trim();
  if (/^\d+$/.test(trimmed)) {
    return parseInt(trimmed, 10) * 1000;
  }
  const parsedDate = Date.parse(trimmed);
  if (!isNaN(parsedDate)) {
    const diff = parsedDate - Date.now();
    return diff > 0 ? diff : 0;
  }
  return null;
}

export function createFetchWithRetry(options: RetryOptions = {}): typeof fetch {
  const maxRetries = options.maxRetries ?? 3;
  const initialDelayMs = options.initialDelayMs ?? 100;
  const maxDelayMs = options.maxDelayMs ?? 2000;
  const timeoutMs = options.timeoutMs;
  const customFetch = options.fetch ?? globalThis.fetch;

  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    let attempt = 0;

    while (true) {
      let timer: ReturnType<typeof setTimeout> | undefined;
      let initToUse = init;

      if (timeoutMs && timeoutMs > 0) {
        const controller = new AbortController();
        if (init?.signal) {
          init.signal.addEventListener('abort', () => controller.abort());
        }
        timer = setTimeout(() => {
          controller.abort(new DOMException('Request timeout', 'TimeoutError'));
        }, timeoutMs);
        initToUse = { ...init, signal: controller.signal };
      }

      try {
        const response = await customFetch(input, initToUse);
        if (timer) clearTimeout(timer);

        if ((response.status === 429 || response.status === 503) && attempt < maxRetries) {
          try {
            await response.text();
          } catch {
            // ignore drain error
          }

          let delay = parseRetryAfter(response.headers.get('Retry-After'));
          if (delay === null || delay < 0) {
            const exponential = Math.min(maxDelayMs, initialDelayMs * Math.pow(2, attempt));
            delay = exponential + Math.random() * initialDelayMs;
          }

          attempt++;
          await new Promise((resolve) => setTimeout(resolve, delay));
          continue;
        }

        return response;
      } catch (err: unknown) {
        if (timer) clearTimeout(timer);
        throw err;
      }
    }
  };
}

export async function handleApiCall<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err && (err as { response: unknown }).response instanceof Response) {
      throw await LayrAPIError.fromResponse((err as { response: Response }).response);
    }
    throw err;
  }
}
