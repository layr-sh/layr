export default function () {
  const nuxtApp = useNuxtApp();
  const auth = nuxtApp.$auth;

  function hasScope(required: string) {
    const scopes = auth.state.value?.scopes || [];
    if (!scopes.length) return false;
    if (!required) return true;
    const [service, rest = "*"] = required.trim().split(":", 2);
    const [resource, action = "*"] = rest.split(".", 2);
    return scopes.some((scope) => {
      const granted = scope.trim();
      if (granted === "*" || granted === required) return true;
      const [grantedService, grantedRest = "*"] = granted.split(":", 2);
      const [grantedResource, grantedAction = "*"] = grantedRest.split(".", 2);
      if (grantedService === service && grantedResource === "*") return true;
      return (
        (grantedService === service || grantedService === "*") &&
        (grantedResource === resource || grantedResource === "*") &&
        (grantedAction === action ||
          grantedAction === "*" ||
          (grantedAction === "write" && action === "read"))
      );
    });
  }

  return { ...auth, hasScope };
}
