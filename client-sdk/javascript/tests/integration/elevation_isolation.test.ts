import { describe, it, expect, vi } from "vitest";
import { createClient, withServiceAccount } from "../../src/core/client";
import { useAuth, useData } from "../../src/services";

describe("Security & Elevation Isolation Integration Tests (SEC-01 to SEC-05)", () => {
  const publishableKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
  const serviceAccountKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210";

  it("SEC-01: Base Plane Header Isolation sends Publishable-Key and no Service-Account-Key", async () => {
    let capturedHeaders: Record<string, string> = {};
    const mockFetch = vi
      .fn()
      .mockImplementation(async (_url: string, requestOptions: RequestInit) => {
        capturedHeaders = requestOptions.headers as Record<string, string>;
        return new Response(JSON.stringify({ session_token: "tok" }), { status: 200 });
      });
    const client = createClient({
      endpoint: "https://api.layr.sh",
      publishableKey,
      fetch: mockFetch,
    });

    const auth = useAuth(client);
    await auth.sign_up({
      email: "test@example.com",
      password: "Password123!",
      phone: "",
      properties: {},
    });

    expect(capturedHeaders["Publishable-Key"]).toBe(publishableKey);
    expect(capturedHeaders["Service-Account-Key"]).toBeUndefined();
  });

  it("SEC-02: Session JWT Header Isolation sends Bearer and no Service-Account-Key", async () => {
    let capturedHeaders: Record<string, string> = {};
    const mockFetch = vi
      .fn()
      .mockImplementation(async (_url: string, requestOptions: RequestInit) => {
        capturedHeaders = requestOptions.headers as Record<string, string>;
        return new Response(JSON.stringify([]), { status: 200 });
      });
    const client = createClient({
      endpoint: "https://api.layr.sh",
      publishableKey,
      sessionToken: "user_jwt_token",
      fetch: mockFetch,
    });

    const data = useData(client);
    await data.from("items").select("*");

    expect(capturedHeaders["Authorization"]).toBe("Bearer user_jwt_token");
    expect(capturedHeaders["Publishable-Key"]).toBe(publishableKey);
    expect(capturedHeaders["Service-Account-Key"]).toBeUndefined();
  });

  it("SEC-03: Control Plane Header Injection sends Service-Account-Key", async () => {
    let capturedHeaders: Record<string, string> = {};
    const mockFetch = vi
      .fn()
      .mockImplementation(async (_url: string, requestOptions: RequestInit) => {
        capturedHeaders = requestOptions.headers as Record<string, string>;
        return new Response(JSON.stringify([]), { status: 200 });
      });
    const client = createClient({
      endpoint: "https://api.layr.sh",
      publishableKey,
      fetch: mockFetch,
    });

    const data = useData(client);
    const elevatedData = withServiceAccount(data, { serviceAccountKey });

    await elevatedData.cp.tables.list();

    expect(capturedHeaders["Service-Account-Key"]).toBe(serviceAccountKey);
  });

  it("SEC-04: Base Client Elevation Guard prevents un-elevated .cp access", () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const data = useData(client);

    expect((data as unknown as { cp?: unknown }).cp).toBeUndefined();
    expect(() => (data as unknown as { cp: { tables: unknown } }).cp.tables).toThrow();
  });

  it("SEC-05: Elevation Immutability preserves unelevated state of base instance", () => {
    const client = createClient({ endpoint: "https://api.layr.sh" });
    const auth = useAuth(client);

    const elevatedAuth = withServiceAccount(auth, { serviceAccountKey });

    expect(elevatedAuth.cp).toBeDefined();
    expect((auth as unknown as { cp?: unknown }).cp).toBeUndefined();
    expect(client.headers["Service-Account-Key"]).toBeUndefined();
  });
});
