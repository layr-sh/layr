// Matches core/error.go ErrorResponse (RFC 9457 Problem Details without type URI)
export interface ErrorResponse {
  title: string;
  status: number;
  detail: string;
  instance?: string;
  timestamp: string;
}

// Matches core/error.go OAuthErrorResponse (RFC 6749 Section 5.2)
export interface OAuthErrorResponse {
  error: string;
  error_description?: string;
}

export type LayrAPIErrorResponse = Partial<ErrorResponse> & Partial<OAuthErrorResponse>;

export class LayrAPIError extends Error {
  readonly title: string;
  readonly status: number;
  readonly detail: string;
  readonly instance?: string;
  readonly timestamp: string;
  readonly error?: string;
  readonly error_description?: string;
  readonly response?: Response;

  constructor(payload?: LayrAPIErrorResponse | string, response?: Response) {
    const p = typeof payload === 'string' ? { detail: payload } : payload;
    const status = p?.status ?? response?.status ?? 500;
    const detail = p?.detail || p?.error_description || response?.statusText || `HTTP ${status}`;
    super(detail);
    this.name = 'LayrAPIError';
    this.status = status;
    this.title = p?.title || response?.statusText || `HTTP ${status}`;
    this.detail = detail;
    this.instance = p?.instance;
    this.timestamp = p?.timestamp || new Date().toISOString();
    this.error = p?.error;
    this.error_description = p?.error_description;
    this.response = response;
  }

  static async fromResponse(response: Response): Promise<LayrAPIError> {
    let bodyText = '';
    try {
      bodyText = await response.text();
    } catch {
      // ignore empty body read
    }

    let payload: LayrAPIErrorResponse | undefined;
    if (bodyText) {
      try {
        payload = JSON.parse(bodyText) as LayrAPIErrorResponse;
      } catch {
        payload = { detail: bodyText };
      }
    }

    return new LayrAPIError(payload, response);
  }
}
