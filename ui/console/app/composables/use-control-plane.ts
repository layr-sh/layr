import { ref } from "vue";
import { LayrAPIError } from "layr-client";
import type {
  ConsoleControlPlane,
  ConsoleControlPlaneModels,
  CoreControlPlane,
  CoreControlPlaneModels,
} from "layr-client";

type ControlPlaneSection =
  | "platform"
  | "settings"
  | "users"
  | "service-accounts"
  | "event-hooks"
  | "events";

export default function (core: CoreControlPlane, console: ConsoleControlPlane) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const config = ref<ConsoleControlPlaneModels.Config | null>(null);
  const coreConfig = ref<CoreControlPlaneModels.RuntimeConfig | null>(null);
  const users = ref<ConsoleControlPlaneModels.User[]>([]);
  const selectedUser = ref<ConsoleControlPlaneModels.User | null>(null);
  const sessions = ref<ConsoleControlPlaneModels.Session[]>([]);
  const accounts = ref<CoreControlPlaneModels.ServiceAccount[]>([]);
  const selectedAccount = ref<CoreControlPlaneModels.ServiceAccount | null>(null);
  const secretKey = ref("");
  const hooks = ref<CoreControlPlaneModels.EventHook[]>([]);
  const hookFilters = ref({ driver: "all", status: "all" });
  const selectedHook = ref<CoreControlPlaneModels.EventHook | null>(null);
  const deliveries = ref<CoreControlPlaneModels.EventHookDelivery[]>([]);
  const events = ref<CoreControlPlaneModels.ListEventsResponse | null>(null);
  const selectedEvent = ref<CoreControlPlaneModels.Event | null>(null);
  const eventFilters = ref({
    type: "",
    actorType: "",
    actorId: "",
    resourceType: "",
    resourceId: "",
  });
  const eventOffset = ref(0);
  const eventLimit = 25;

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

  async function fetchEvents() {
    events.value = await core.events.list({
      ...Object.fromEntries(
        Object.entries(eventFilters.value)
          .map(([name, value]) => [name, value.trim()])
          .filter(([, value]) => value),
      ),
      limit: eventLimit,
      offset: eventOffset.value,
    });
  }

  function load(section: ControlPlaneSection) {
    return run(async () => {
      if (section === "platform") coreConfig.value = await core.config.get();
      if (section === "settings") config.value = await console.config.get();
      if (section === "users") {
        users.value = await console.users.list();
        if (auth.hasScope("core:service-account.read"))
          accounts.value = await core.service_accounts.list();
      }
      if (section === "service-accounts") accounts.value = await core.service_accounts.list();
      if (section === "event-hooks")
        hooks.value = await core.event_hooks.list({
          ...(hookFilters.value.driver !== "all" ? { driver: hookFilters.value.driver } : {}),
          ...(hookFilters.value.status !== "all"
            ? { isEnabled: hookFilters.value.status === "enabled" }
            : {}),
        });
      if (section === "events") await fetchEvents();
    });
  }

  function saveConfig(input: ConsoleControlPlaneModels.Config) {
    return run(async () => {
      config.value = await console.config.update(input);
      await auth.refresh();
      if (auth.state.value?.mfa_required) await navigateTo("/control-plane/account-security");
    }, "Console Settings Saved");
  }

  function saveCoreConfig(input: CoreControlPlaneModels.RuntimeConfig) {
    return run(async () => {
      coreConfig.value = await core.config.update(input);
    }, "Platform Settings Saved");
  }

  function createUser(input: ConsoleControlPlaneModels.CreateConsoleUserInput) {
    return run(async () => {
      await console.users.create(input);
      users.value = await console.users.list();
      if (auth.hasScope("core:service-account.read"))
        accounts.value = await core.service_accounts.list();
    }, "Console User Created");
  }

  function selectUser(userId: string) {
    return run(async () => {
      selectedUser.value = null;
      sessions.value = [];
      const [user, userSessions] = await Promise.all([
        console.users.get(userId),
        console.users.sessions.list(userId),
      ]);
      selectedUser.value = user;
      sessions.value = userSessions;
    });
  }

  function updateUser(userId: string, input: ConsoleControlPlaneModels.UpdateConsoleUserInput) {
    return run(async () => {
      selectedUser.value = await console.users.update(userId, input);
      users.value = await console.users.list();
      sessions.value = selectedUser.value.isEnabled
        ? await console.users.sessions.list(userId)
        : [];
      if (auth.state.value?.user.id === userId) await auth.refresh();
    }, "Console User Updated");
  }

  function deleteUser(userId: string) {
    return run(async () => {
      await console.users.delete(userId);
      selectedUser.value = null;
      sessions.value = [];
      users.value = await console.users.list();
      accounts.value = accounts.value.filter((account) => account.consoleUserId !== userId);
    }, "Console User Deleted");
  }

  function resetPassword(userId: string, newPassword: string) {
    return run(async () => {
      await console.users.reset_password(userId, { newPassword });
      sessions.value = [];
      if (auth.state.value?.user.id === userId) {
        auth.clear();
        await navigateTo("/sign-in");
      }
    }, "Password Reset");
  }

  function revokeSession(userId: string, sessionId?: string) {
    return run(async () => {
      if (sessionId) await console.users.sessions.delete(userId, sessionId);
      else await console.users.sessions.revoke(userId);
      if (
        auth.state.value?.user.id === userId &&
        (!sessionId || auth.state.value.id === sessionId)
      ) {
        sessions.value = [];
        auth.clear();
        await navigateTo("/sign-in");
      } else {
        sessions.value = await console.users.sessions.list(userId);
      }
    }, "Sessions Revoked");
  }

  function createAccount(input: CoreControlPlaneModels.CreateServiceAccountInput) {
    return run(async () => {
      secretKey.value = "";
      const account = await core.service_accounts.create(input);
      secretKey.value = account.secretKey;
      accounts.value = await core.service_accounts.list();
    }, "Service Account Created");
  }

  function selectAccount(accountId: string) {
    return run(async () => {
      selectedAccount.value = null;
      selectedAccount.value = await core.service_accounts.get(accountId);
    });
  }

  function updateAccount(
    accountId: string,
    input: CoreControlPlaneModels.UpdateServiceAccountInput,
  ) {
    return run(async () => {
      selectedAccount.value = await core.service_accounts.update(accountId, input);
      if (
        selectedAccount.value.consoleUserId &&
        selectedAccount.value.consoleUserId === auth.state.value?.user.id
      ) {
        await auth.refresh();
        if (!auth.state.value) await navigateTo("/sign-in");
      }
      accounts.value = auth.hasScope("core:service-account.read")
        ? await core.service_accounts.list()
        : [];
    }, "Service Account Updated");
  }

  function deleteAccount(accountId: string) {
    return run(async () => {
      await core.service_accounts.delete(accountId);
      selectedAccount.value = null;
      accounts.value = await core.service_accounts.list();
    }, "Service Account Deleted");
  }

  function createHook(input: CoreControlPlaneModels.CreateEventHookInput) {
    return run(async () => {
      await core.event_hooks.create(input);
      hooks.value = await core.event_hooks.list();
    }, "Event Hook Created");
  }

  function selectHook(hookId: string) {
    return run(async () => {
      selectedHook.value = null;
      deliveries.value = [];
      const [hook, hookDeliveries] = await Promise.all([
        core.event_hooks.get(hookId),
        core.event_hooks.deliveries.list(hookId),
      ]);
      selectedHook.value = hook;
      deliveries.value = hookDeliveries;
    });
  }

  function updateHook(hookId: string, input: CoreControlPlaneModels.UpdateEventHookInput) {
    return run(async () => {
      selectedHook.value = await core.event_hooks.update(hookId, input);
      hooks.value = await core.event_hooks.list();
    }, "Event Hook Updated");
  }

  function deleteHook(hookId: string) {
    return run(async () => {
      await core.event_hooks.delete(hookId);
      selectedHook.value = null;
      deliveries.value = [];
      hooks.value = await core.event_hooks.list();
    }, "Event Hook Deleted");
  }

  function retryDelivery(hookId: string, deliveryId: string) {
    return run(async () => {
      await core.event_hooks.deliveries.retry(hookId, deliveryId);
      deliveries.value = await core.event_hooks.deliveries.list(hookId);
    }, "Delivery Queued for Retry");
  }

  function listEvents(offset = 0) {
    return run(async () => {
      eventOffset.value = offset;
      selectedEvent.value = null;
      await fetchEvents();
    });
  }

  function selectEvent(eventId: string) {
    return run(async () => {
      selectedEvent.value = null;
      selectedEvent.value = await core.events.get(eventId);
    });
  }

  return {
    run,
    busy,
    error,
    notice,
    config,
    coreConfig,
    saveCoreConfig,
    users,
    selectedUser,
    sessions,
    accounts,
    selectedAccount,
    secretKey,
    hooks,
    hookFilters,
    selectedHook,
    deliveries,
    events,
    selectedEvent,
    eventFilters,
    eventOffset,
    eventLimit,
    load,
    saveConfig,
    createUser,
    selectUser,
    updateUser,
    deleteUser,
    resetPassword,
    revokeSession,
    createAccount,
    selectAccount,
    updateAccount,
    deleteAccount,
    createHook,
    selectHook,
    updateHook,
    deleteHook,
    retryDelivery,
    listEvents,
    selectEvent,
  };
}
