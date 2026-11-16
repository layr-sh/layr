import { describe, expect, it } from "vitest";
import {
  AuthBaseModels,
  AuthControlPlaneModels,
  ConsoleControlPlaneModels,
  DataControlPlaneModels,
  FileStorageControlPlaneModels,
  FunctionControlPlaneModels,
  ImageControlPlaneModels,
  TasksControlPlaneModels,
  useAuth,
  useConsole,
  useData,
  useFileStorage,
  useFunction,
  useImage,
  useTasks,
  withServiceAccount,
  type LayrClient,
} from "../../../src";
import { createMockServiceContext } from "./helpers";

describe("Service schema isolation", () => {
  const cases = [
    {
      name: "auth",
      response: { password: { min_length: 14 } },
      expected: { password: { minLength: 14 } },
      get: (client: LayrClient) => withServiceAccount(useAuth(client)).cp.config.get(),
    },
    {
      name: "console",
      response: { session_idle_timeout_seconds: 600, mfa_enforced: true },
      expected: { sessionIdleTimeoutSeconds: 600, mfaEnforced: true },
      get: (client: LayrClient) => withServiceAccount(useConsole(client)).cp.config.get(),
    },
    {
      name: "data",
      response: { schemas: ["public"], rest: { max_limit: 200 } },
      expected: { schemas: ["public"] },
      get: (client: LayrClient) => withServiceAccount(useData(client)).cp.config.get(),
    },
    {
      name: "file_storage",
      response: { enabled: true, default_max_file_size_bytes: 1024 },
      expected: { enabled: true, defaultMaxFileSizeBytes: 1024 },
      get: (client: LayrClient) => withServiceAccount(useFileStorage(client)).cp.config.get(),
    },
    {
      name: "function",
      response: { default_runtime: "workerd", default_memory_limit_mb: 128 },
      expected: { defaultRuntime: "workerd", defaultMemoryLimitMb: 128 },
      get: (client: LayrClient) => withServiceAccount(useFunction(client)).cp.config.get(),
    },
    {
      name: "image",
      response: { default_quality: 80, strip_metadata: true },
      expected: { defaultQuality: 80, stripMetadata: true },
      get: (client: LayrClient) => withServiceAccount(useImage(client)).cp.config.get(),
    },
    {
      name: "tasks",
      response: { concurrency_limit: 6, retry_max_attempts: 4 },
      expected: { concurrencyLimit: 6, retryMaxAttempts: 4 },
      get: (client: LayrClient) => withServiceAccount(useTasks(client)).cp.config.get(),
    },
  ];

  for (const testCase of cases) {
    it(`uses ${testCase.name}'s own config schema`, async () => {
      const { client } = createMockServiceContext(testCase.response);
      const config = await testCase.get(client);
      expect(config).toMatchObject(testCase.expected);
      if (testCase.name !== "auth") {
        expect(config).not.toHaveProperty("password");
      }
    });
  }

  it("serializes Console configuration with Console fields only", async () => {
    const input = {
      mfaEnforced: true,
      sessionIdleTimeoutSeconds: 600,
      sessionMaxLifetimeSeconds: 3600,
    };
    const { client, getLastInit } = createMockServiceContext();
    const service = withServiceAccount(useConsole(client), { serviceAccountKey: "console-key" });
    await service.cp.config.update(input);
    expect(JSON.parse(getLastInit()?.body as string)).toEqual({
      mfa_enforced: true,
      session_idle_timeout_seconds: 600,
      session_max_lifetime_seconds: 3600,
    });
    expect(getLastInit()?.headers).toMatchObject({ "Service-Account-Key": "console-key" });
  });

  it("keeps Auth and Console user/session models separate", async () => {
    const user = {
      id: "console-user",
      email: "operator@example.com",
      is_enabled: true,
      mfa_enabled: false,
      created_at: "2026-11-15T00:00:00Z",
      last_updated_at: "2026-11-15T00:00:00Z",
    };
    const { client } = createMockServiceContext((url) => {
      if (url.endsWith("/sessions")) {
        return [
          {
            id: "console-session",
            user_id: user.id,
            ip_address: "127.0.0.1",
            user_agent: "test",
            expires_at: "2026-11-16T00:00:00Z",
            created_at: "2026-11-15T00:00:00Z",
          },
        ];
      }
      return user;
    });
    const service = withServiceAccount(useConsole(client));
    const result = await service.cp.users.get(user.id);
    expect(result.isEnabled).toBe(true);
    expect(result).not.toHaveProperty("phone");
    expect(result).not.toHaveProperty("isAnonymous");
    const sessions = await service.cp.users.sessions.list(user.id);
    expect(sessions[0]?.ipAddress).toBe("127.0.0.1");
    expect(sessions[0]).not.toHaveProperty("clientId");
    expect(sessions[0]).not.toHaveProperty("refreshTokenHash");
  });

  it("preserves session changes after constructing a service-local runtime", async () => {
    const { client, getLastInit } = createMockServiceContext({});
    const auth = useAuth(client);
    client.setSession("new-session");
    await auth.user.get();
    expect(getLastInit()?.headers).toMatchObject({ Authorization: "Bearer new-session" });
    client.clearSession();
    await auth.user.get();
    expect(getLastInit()?.headers).not.toHaveProperty("Authorization");
  });

  it("exports models in unambiguous service/plane namespaces", () => {
    expect(AuthBaseModels.UserFromJSON).toBeTypeOf("function");
    for (const models of [
      AuthControlPlaneModels,
      ConsoleControlPlaneModels,
      DataControlPlaneModels,
      FileStorageControlPlaneModels,
      FunctionControlPlaneModels,
      ImageControlPlaneModels,
      TasksControlPlaneModels,
    ]) {
      expect(models.ConfigFromJSON).toBeTypeOf("function");
    }
    expect(ConsoleControlPlaneModels.ConfigFromJSON).not.toBe(
      AuthControlPlaneModels.ConfigFromJSON,
    );
    expect(FunctionControlPlaneModels.GetStatsResponseFromJSON).not.toBe(
      ImageControlPlaneModels.GetStatsResponseFromJSON,
    );
    expect(TasksControlPlaneModels.ListExecutionsResponseFromJSON).not.toBe(
      FunctionControlPlaneModels.ListExecutionsResponseFromJSON,
    );
  });
});
