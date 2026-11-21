import { LayrAPIError } from "layr-client";
import type { AuthControlPlane, AuthControlPlaneModels as Models } from "layr-client";
import { authConfigInput } from "../utils/auth-forms";

export default function (service: AuthControlPlane) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const stats = ref<Models.StatsResponse | null>(null);
  const users = ref<Models.ListUsersResponse | null>(null);
  const user = ref<Models.User | null>(null);
  const sessions = ref<Models.ListUserSessionsResponse | null>(null);
  const config = ref<Models.Config | null>(null);
  const offset = ref(0);
  const limit = 20;
  const filters = reactive({ search: "", role: "" });
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
  async function fetchUsers(nextOffset: number) {
    users.value = await service.users.list({
      limit,
      offset: nextOffset,
      search: filters.search.trim(),
      role: filters.role.trim(),
    });
    offset.value = nextOffset;
  }
  function loadStats() {
    return run(async () => {
      stats.value = await service.stats.get();
    });
  }
  function listUsers(nextOffset = 0) {
    return run(() => fetchUsers(nextOffset));
  }
  function getUser(id: string) {
    user.value = null;
    return run(async () => {
      user.value = await service.users.get(id);
    });
  }
  function createUser(input: Models.CreateUserInput) {
    return run(async () => {
      await service.users.create(input);
      await fetchUsers(0);
    }, "Auth User Created");
  }
  function deleteUser(id: string) {
    return run(async () => {
      await service.users.delete(id);
      user.value = null;
      await fetchUsers(Math.max(0, offset.value - (users.value?.data?.length === 1 ? limit : 0)));
    }, "Auth User Deleted");
  }
  function lockUser(id: string, lockedUntil?: Date) {
    return run(async () => {
      user.value = await service.users.lock(id, { lockedUntil } as Models.LockUserInput);
      await fetchUsers(offset.value);
    }, "Auth User Locked");
  }
  function unlockUser(id: string) {
    return run(async () => {
      user.value = await service.users.unlock(id);
      await fetchUsers(offset.value);
    }, "Auth User Unlocked");
  }
  function listSessions(id: string) {
    user.value = null;
    sessions.value = null;
    return run(async () => {
      const nextUser = await service.users.get(id);
      const nextSessions = await service.users.sessions.list(id);
      user.value = nextUser;
      sessions.value = nextSessions;
    });
  }
  function revokeSessions(id: string) {
    return run(async () => {
      await service.users.sessions.revoke(id);
      sessions.value = await service.users.sessions.list(id);
    }, "Auth Sessions Revoked");
  }
  function loadConfig() {
    return run(async () => {
      config.value = await service.config.get();
    });
  }
  function saveConfig(input: Models.Config) {
    return run(async () => {
      config.value = await service.config.update(authConfigInput(input));
    }, "Auth Configuration Saved");
  }
  return {
    busy,
    error,
    notice,
    stats,
    users,
    user,
    sessions,
    config,
    offset,
    limit,
    filters,
    run,
    loadStats,
    listUsers,
    getUser,
    createUser,
    deleteUser,
    lockUser,
    unlockUser,
    listSessions,
    revokeSessions,
    loadConfig,
    saveConfig,
  };
}
