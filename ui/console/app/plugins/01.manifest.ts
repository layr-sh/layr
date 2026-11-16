import type { CoreBaseModels } from "layr-client";

export default defineNuxtPlugin(async ({ $client }) => {
  const state = useState<CoreBaseModels.GetManifestResponse>("manifest");

  const initialize = async () => {
    state.value = await $client.value.core.manifest().catch((err) => {
      console.error("Failed to fetch manifest", err);
      throw createError({
        status: 500,
        message: "Something went wrong",
        fatal: true,
      });
    });
  };

  callOnce(initialize);

  return {
    provide: {
      manifest: {
        state,
        initialize,
        refresh: initialize,
      },
    },
  };
});
