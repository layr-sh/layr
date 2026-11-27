import { LayrAPIError } from "layr-client";
import type { DataControlPlane, DataControlPlaneModels } from "layr-client";

export interface SQLResult {
  columns: string[] | null;
  rows: Record<string, unknown>[] | null;
  rows_affected: number;
  execution_time_ms: number;
}

export default function (service: DataControlPlane) {
  const auth = useAuth();
  const client = useClient();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const tables = ref<DataControlPlaneModels.Table[]>([]);
  const selected = ref<DataControlPlaneModels.Table | null>(null);
  const policies = ref<DataControlPlaneModels.Policy[]>([]);
  const config = ref<DataControlPlaneModels.Config | null>(null);
  const result = ref<SQLResult | null>(null);
  async function run(operation: () => Promise<void>, message = "") {
    if (busy.value) return false;
    busy.value = true;
    error.value = "";
    notice.value = "";
    try {
      await operation();
      notice.value = message;
      return true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : "The request failed. Please try again.";
      if (err instanceof LayrAPIError && (err.status === 401 || err.status === 403)) {
        await auth.refresh();
        if (!auth.state.value) await navigateTo("/sign-in");
      }
      return false;
    } finally {
      busy.value = false;
    }
  }
  function loadTables() {
    return run(async () => {
      tables.value = (await service.tables.list()).data ?? [];
    });
  }
  function selectTable(schema: string, name: string) {
    return run(async () => {
      const table = await service.tables.get(schema, name);
      const list = await service.tables.policies.list(schema, name);
      selected.value = table;
      policies.value = list.data ?? [];
    });
  }
  function mutate(
    operation: () => Promise<unknown>,
    message: string,
    schema: string,
    name: string,
  ) {
    return run(async () => {
      await operation();
      selected.value = null;
      tables.value = (await service.tables.list()).data ?? [];
      if (tables.value.some((table) => table.schema === schema && table.name === name)) {
        selected.value = await service.tables.get(schema, name);
        policies.value = (await service.tables.policies.list(schema, name)).data ?? [];
      } else policies.value = [];
    }, message);
  }
  function loadConfig() {
    return run(async () => {
      config.value = await service.config.get();
    });
  }
  function saveConfig(input: DataControlPlaneModels.Config) {
    return run(async () => {
      config.value = await service.config.update(input);
    }, "Data Configuration Saved");
  }
  function invalidateCache(input: DataControlPlaneModels.InvalidateCacheInput) {
    return run(async () => {
      await service.cache.invalidate(input);
    }, "Cache Invalidation Request Accepted");
  }
  function flushCache() {
    return run(async () => {
      await service.cache.flush();
    }, "Cache Flush Request Accepted");
  }
  async function requestSQL(sql: string): Promise<SQLResult> {
    // The scratchpad is console-only and intentionally excluded from the public OpenAPI SDK.
    const response = await fetch(`${client.value.endpoint}/v1/_/data/sql`, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(client.value.sessionToken
          ? { Authorization: `Bearer ${client.value.sessionToken.replace(/^Bearer\s+/i, "")}` }
          : {}),
      },
      body: JSON.stringify({ sql }),
    });
    if (!response.ok) throw await LayrAPIError.fromResponse(response);
    return response.json();
  }
  function executeSQL(sql: string) {
    return run(async () => {
      result.value = null;
      result.value = await requestSQL(sql);
    });
  }
  function mutateSQL(sql: string, schema: string, name: string) {
    return mutate(() => requestSQL(sql), "Schema Updated", schema, name);
  }
  return {
    busy,
    error,
    notice,
    tables,
    selected,
    policies,
    config,
    result,
    run,
    loadTables,
    selectTable,
    mutate,
    loadConfig,
    saveConfig,
    invalidateCache,
    flushCache,
    executeSQL,
    mutateSQL,
  };
}
