import type { GetManifestResponse } from "layr-client";

export default defineNuxtPlugin(async ({ $client }) => {
  const state = useState<GetManifestResponse>("manifest");

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
