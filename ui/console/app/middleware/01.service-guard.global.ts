export default defineNuxtRouteMiddleware(async ({ path: toPath }) => {
  const manifest = useManifest();

  const serviceRouteMap: Record<string, string> = {
    "/data": "data",
    "/auth": "auth",
    "/file-storage": "file_storage",
    "/function": "function",
    "/tasks": "tasks",
    "/image": "image",
  };

  const matchedPrefix = Object.keys(serviceRouteMap).find(
    (prefix) => toPath === prefix || toPath.startsWith(`${prefix}/`),
  );

  if (matchedPrefix) {
    if (!manifest.state.value) {
      await manifest.initialize().catch(() => {});
    }
    const serviceKey = serviceRouteMap[matchedPrefix];
    if (serviceKey) {
      const enabledServices = new Set(manifest.state.value?.enabledServices || []);
      if (!enabledServices.has(serviceKey)) {
        return navigateTo("/");
      }
    }
  }
});
