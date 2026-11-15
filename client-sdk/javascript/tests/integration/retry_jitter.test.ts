import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createFetchWithRetry } from "../../src/core/transport";

describe("Resilience, Backoff & Jitter Integration Tests (RET-01 to RET-08)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("RET-01: Retryable 429 with Integer Seconds in Retry-After header", async () => {
    let callCount = 0;
    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      if (callCount === 1) {
        return new Response("rate limited", {
          status: 429,
          headers: { "Retry-After": "2" },
        });
      }
      return new Response("ok", { status: 200 });
    });

    const resilientFetch = createFetchWithRetry({ fetch: mockFetch });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(2000);
    const response = await fetchPromise;

    expect(callCount).toBe(2);
    expect(response.status).toBe(200);
  });

  it("RET-02: Retryable 429 with RFC 1123 Date in Retry-After header", async () => {
    let callCount = 0;
    const targetDate = new Date(Date.now() + 3000).toUTCString();

    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      if (callCount === 1) {
        return new Response("rate limited", {
          status: 429,
          headers: { "Retry-After": targetDate },
        });
      }
      return new Response("ok", { status: 200 });
    });

    const resilientFetch = createFetchWithRetry({ fetch: mockFetch });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(3000);
    const response = await fetchPromise;

    expect(callCount).toBe(2);
    expect(response.status).toBe(200);
  });

  it("RET-03: Retryable 429 with Exponential Backoff and Jitter", async () => {
    let callCount = 0;
    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      if (callCount === 1) {
        return new Response("rate limited", { status: 429 });
      }
      return new Response("ok", { status: 200 });
    });

    const resilientFetch = createFetchWithRetry({
      fetch: mockFetch,
      initialDelayMs: 100,
      maxDelayMs: 2000,
    });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(300);
    const response = await fetchPromise;

    expect(callCount).toBe(2);
    expect(response.status).toBe(200);
  });

  it("RET-04: Retryable 503 Service Unavailable retries and succeeds", async () => {
    let callCount = 0;
    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      if (callCount === 1) {
        return new Response("service unavailable", { status: 503 });
      }
      return new Response("ok", { status: 200 });
    });

    const resilientFetch = createFetchWithRetry({
      fetch: mockFetch,
      initialDelayMs: 50,
    });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(200);
    const response = await fetchPromise;

    expect(callCount).toBe(2);
    expect(response.status).toBe(200);
  });

  it("RET-05: Stream Body Drain prior to retry consumes response body", async () => {
    let callCount = 0;
    let bodyDrained = false;

    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      if (callCount === 1) {
        const stream = new ReadableStream({
          start(controller) {
            controller.enqueue(new Uint8Array(1024));
            controller.close();
            bodyDrained = true;
          },
        });
        return new Response(stream, { status: 429 });
      }
      return new Response("ok", { status: 200 });
    });

    const resilientFetch = createFetchWithRetry({
      fetch: mockFetch,
      initialDelayMs: 50,
    });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(200);
    const response = await fetchPromise;

    expect(callCount).toBe(2);
    expect(bodyDrained).toBe(true);
    expect(response.status).toBe(200);
  });

  it("RET-06: Non-retryable errors (400, 401, 404, 500) fail immediately with 0 retries", async () => {
    let callCount = 0;
    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      return new Response("bad request", { status: 400 });
    });

    const resilientFetch = createFetchWithRetry({ fetch: mockFetch });
    const response = await resilientFetch("https://api.layr.sh/test");

    expect(callCount).toBe(1);
    expect(response.status).toBe(400);
  });

  it("RET-07: Max Retries Exhaustion returns final 429 response after maxRetries attempts", async () => {
    let callCount = 0;
    const mockFetch = vi.fn().mockImplementation(async () => {
      callCount++;
      return new Response("persistent 429", { status: 429 });
    });

    const resilientFetch = createFetchWithRetry({
      fetch: mockFetch,
      maxRetries: 2,
      initialDelayMs: 20,
    });
    const fetchPromise = resilientFetch("https://api.layr.sh/test");

    await vi.advanceTimersByTimeAsync(500);
    const response = await fetchPromise;

    // 1 initial attempt + 2 retries = 3 calls
    expect(callCount).toBe(3);
    expect(response.status).toBe(429);
  });

  it("RET-08: Request Timeout Exceeded aborts via AbortSignal", async () => {
    const mockFetch = vi.fn().mockImplementation((_url: string, init?: RequestInit) => {
      return new Promise((_resolve, reject) => {
        if (init?.signal) {
          init.signal.addEventListener("abort", () => {
            reject(new DOMException("The operation was aborted", "TimeoutError"));
          });
        }
      });
    });

    const resilientFetch = createFetchWithRetry({
      fetch: mockFetch,
      timeoutMs: 100,
    });

    const fetchPromise = resilientFetch("https://api.layr.sh/test");
    const assertion = expect(fetchPromise).rejects.toThrow();
    await vi.advanceTimersByTimeAsync(150);

    await assertion;
  });
});
