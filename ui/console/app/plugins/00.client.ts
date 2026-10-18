import {
  createClient,
  useAuth,
  useConsole,
  useCore,
  useData,
  useFileStorage,
  useFunction,
  useImage,
  useTasks,
  withServiceAccount,
} from "layr-client";

export default defineNuxtPlugin(async () => {
  const baseClient = createClient({ endpoint: location.origin });

  const client = computed(() =>
    Object.assign(baseClient, {
      core: withServiceAccount(useCore(baseClient)),
      data: withServiceAccount(useData(baseClient)),
      auth: withServiceAccount(useAuth(baseClient)),
      file_storage: withServiceAccount(useFileStorage(baseClient)),
      tasks: withServiceAccount(useTasks(baseClient)),
      image: withServiceAccount(useImage(baseClient)),
      function: withServiceAccount(useFunction(baseClient)),
      console: withServiceAccount(useConsole(baseClient)),
    }),
  );

  return {
    provide: {
      client,
    },
  };
});
