import { describe, it, expect } from "vitest";
import {
  normalizeEndpointURL,
  buildHeaders,
  LayrAPIError,
  parseRetryAfter,
  createFetchWithRetry,
  handleApiCall,
} from "../../src/core/transport";

describe("Transport & Normalization Unit Tests", () => {
  it("TR-01: Endpoint Normalization defaults to localhost:5297", () => {
    expect(normalizeEndpointURL()).toBe("http://localhost:5297");
    expect(normalizeEndpointURL("")).toBe("http://localhost:5297");
    expect(normalizeEndpointURL("   ")).toBe("http://localhost:5297");
    expect(normalizeEndpointURL(undefined)).toBe("http://localhost:5297");
  });

  it("TR-02: Endpoint Normalization strips trailing slashes", () => {
    expect(normalizeEndpointURL("https://api.layr.sh/")).toBe("https://api.layr.sh");
    expect(normalizeEndpointURL("https://api.layr.sh///")).toBe("https://api.layr.sh");
    expect(normalizeEndpointURL("http://127.0.0.1:5297")).toBe("http://127.0.0.1:5297");
  });

  it("TR-03: Header Injection sets Publishable-Key", () => {
    const publishableKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
    const headers = buildHeaders({ publishableKey });
    expect(headers["Publishable-Key"]).toBe(publishableKey);
  });

  it("TR-04: Header Injection formats Authorization header", () => {
    const h1 = buildHeaders({ sessionToken: "jwt_token_abc" });
    expect(h1["Authorization"]).toBe("Bearer jwt_token_abc");

    const h2 = buildHeaders({ sessionToken: "Bearer jwt_token_abc" });
    expect(h2["Authorization"]).toBe("Bearer jwt_token_abc");

    const h3 = buildHeaders({ sessionToken: "   " });
    expect(h3["Authorization"]).toBeUndefined();
  });

  it("TR-05: Header Injection sets Service-Account-Key", () => {
    const serviceAccountKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210";
    const headers = buildHeaders({ serviceAccountKey });
    expect(headers["Service-Account-Key"]).toBe(serviceAccountKey);
  });

  it("TR-06: Header Merging preserves and merges custom headers", () => {
    const publishableKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
    const headers = buildHeaders({
      publishableKey,
      headers: {
        "Custom-Header": "val",
        "X-Null": null as any,
        "X-Undef": undefined as any,
        "Publishable-Key": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
      },
    });
    expect(headers["Custom-Header"]).toBe("val");
    expect(headers["X-Null"]).toBeUndefined();
    expect(headers["X-Undef"]).toBeUndefined();
    expect(headers["Publishable-Key"]).toBe(publishableKey);
  });

  it("TR-07: Error Parsing handles RFC 9457 Problem Details", async () => {
    const problemJson = JSON.stringify({
      title: "Bad Request",
      status: 400,
      detail: "Invalid parameter: email",
      instance: "/v1/auth/sign-up",
      timestamp: "2026-10-13T00:00:00Z",
    });
    const problemResponse = new Response(problemJson, {
      status: 400,
      statusText: "Bad Request",
      headers: { "Content-Type": "application/problem+json" },
    });

    const err = await LayrAPIError.fromResponse(problemResponse);
    expect(err).toBeInstanceOf(LayrAPIError);
    expect(err.status).toBe(400);
    expect(err.title).toBe("Bad Request");
    expect(err.detail).toBe("Invalid parameter: email");
    expect(err.instance).toBe("/v1/auth/sign-up");
    expect(err.timestamp).toBe("2026-10-13T00:00:00Z");
  });

  it("TR-08: Error Parsing handles non-JSON fallback and missing body", async () => {
    const responseHtml = new Response("<html>502 Gateway Error</html>", {
      status: 502,
      statusText: "Bad Gateway",
    });
    const err = await LayrAPIError.fromResponse(responseHtml);
    expect(err.status).toBe(502);
    expect(err.title).toBe("Bad Gateway");
    expect(err.detail).toBe("<html>502 Gateway Error</html>");

    const responseEmpty = new Response("", { status: 500, statusText: "Server Error" });
    const errEmpty = await LayrAPIError.fromResponse(responseEmpty);
    expect(errEmpty.status).toBe(500);
    expect(errEmpty.title).toBe("Server Error");
    expect(errEmpty.detail).toBe("Unknown error occurred");
    expect(errEmpty.message).toBe("Unknown error occurred");

    const customErr = new LayrAPIError({ status: 404, detail: "Not found message" });
    expect(customErr.status).toBe(404);
    expect(customErr.message).toBe("Not found message");
  });

  it("TR-09: parseRetryAfter correctly parses delta-seconds and HTTP-date", () => {
    expect(parseRetryAfter(null)).toBeNull();
    expect(parseRetryAfter("5")).toBe(5000);
    expect(parseRetryAfter("0")).toBe(0);

    const futureDate = new Date(Date.now() + 10000).toUTCString();
    const delay = parseRetryAfter(futureDate);
    expect(delay).toBeGreaterThan(5000);
    expect(delay).toBeLessThanOrEqual(10000);

    const pastDate = new Date(Date.now() - 5000).toUTCString();
    expect(parseRetryAfter(pastDate)).toBe(0);

    expect(parseRetryAfter("invalid-header")).toBeNull();
  });

  it("TR-10: handleApiCall catches and unwraps response errors", async () => {
    const result = await handleApiCall(async () => "success");
    expect(result).toBe("success");

    const errResponse = new Response('{"detail":"Forbidden"}', { status: 403 });
    await expect(
      handleApiCall(async () => {
        throw { response: errResponse };
      }),
    ).rejects.toThrow("Forbidden");

    await expect(
      handleApiCall(async () => {
        throw new Error("plain error");
      }),
    ).rejects.toThrow("plain error");
  });

  it("TR-11: external AbortSignal aborts request when timeoutMs is set", async () => {
    const userController = new AbortController();
    const fetchWithRetry = createFetchWithRetry({
      timeoutMs: 5000,
      fetch: async (_url, init) => {
        return new Promise((_resolve, reject) => {
          if (init?.signal?.aborted) {
            reject(new DOMException("User aborted", "AbortError"));
          } else {
            init?.signal?.addEventListener("abort", () =>
              reject(new DOMException("User aborted", "AbortError")),
            );
          }
        });
      },
    });
    const promise = fetchWithRetry("https://api.layr.sh", { signal: userController.signal });
    userController.abort();
    await expect(promise).rejects.toThrow();
  });

  it("TR-12: LayrAPIError constructor default overloads", () => {
    const defaultErr = new LayrAPIError();
    expect(defaultErr.status).toBe(500);
    expect(defaultErr.title).toBe("HTTP 500");
    expect(defaultErr.detail).toBe("Unknown error occurred");

    const strErr = new LayrAPIError("custom message");
    expect(strErr.status).toBe(500);
    expect(strErr.detail).toBe("custom message");
  });

  it("TR-13: createFetchWithRetry clears timeout timer on success", async () => {
    const fetchWithRetry = createFetchWithRetry({
      timeoutMs: 5000,
      fetch: async () => new Response("ok", { status: 200 }),
    });
    const res = await fetchWithRetry("https://api.layr.sh");
    expect(res.status).toBe(200);
  });

  it("TR-14: createFetchWithRetry clears timeout timer on error", async () => {
    const fetchWithRetry = createFetchWithRetry({
      timeoutMs: 5000,
      fetch: async () => {
        throw new Error("network down");
      },
    });
    await expect(fetchWithRetry("https://api.layr.sh")).rejects.toThrow("network down");
  });

  it("TR-15: createFetchWithRetry throws on fetch error without timeoutMs", async () => {
    const fetchWithRetry = createFetchWithRetry({
      fetch: async () => {
        throw new Error("raw connection failure");
      },
    });
    await expect(fetchWithRetry("https://api.layr.sh")).rejects.toThrow("raw connection failure");
  });
});
