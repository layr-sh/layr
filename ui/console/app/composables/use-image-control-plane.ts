import { LayrAPIError } from "layr-client";
import type { ImageControlPlane, ImageControlPlaneModels } from "layr-client";

export default function (image: ImageControlPlane) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const config = ref<ImageControlPlaneModels.Config | null>(null);
  const presets = ref<ImageControlPlaneModels.ListPresetsResponse | null>(null);
  const selectedPreset = ref<ImageControlPlaneModels.Preset | null>(null);
  const stats = ref<ImageControlPlaneModels.GetStatsResponse | null>(null);
  const signedUrl = ref<ImageControlPlaneModels.SignURLResponse | null>(null);

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
  async function fetchPresets() {
    presets.value = await image.presets.list();
  }
  function load(section: "configuration" | "presets" | "statistics") {
    return run(async () => {
      if (section === "configuration") config.value = await image.config.get();
      if (section === "presets") await fetchPresets();
      if (section === "statistics") stats.value = await image.stats.get();
    });
  }
  function saveConfig(input: ImageControlPlaneModels.Config) {
    return run(async () => {
      config.value = await image.config.update(input);
    }, "Image Configuration Saved");
  }
  function selectPreset(presetId: string) {
    return run(async () => {
      selectedPreset.value = await image.presets.get(presetId);
    });
  }
  function createPreset(input: ImageControlPlaneModels.CreatePresetInput) {
    return run(async () => {
      await image.presets.create(input);
      await fetchPresets();
    }, "Image Preset Created");
  }
  function updatePreset(presetId: string, input: ImageControlPlaneModels.UpdatePresetInput) {
    return run(async () => {
      selectedPreset.value = await image.presets.update(presetId, input);
      await fetchPresets();
    }, "Image Preset Updated");
  }
  function deletePreset(presetId: string) {
    return run(async () => {
      await image.presets.delete(presetId);
      selectedPreset.value = null;
      await fetchPresets();
    }, "Image Preset Deleted");
  }
  function signUrl(path: string) {
    signedUrl.value = null;
    return run(async () => {
      signedUrl.value = await image.url.sign({ path: path.trim() });
    }, "Image URL Signed");
  }
  return {
    busy,
    error,
    notice,
    config,
    presets,
    selectedPreset,
    stats,
    signedUrl,
    run,
    load,
    saveConfig,
    selectPreset,
    createPreset,
    updatePreset,
    deletePreset,
    signUrl,
  };
}
