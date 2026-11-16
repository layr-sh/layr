export default defineNuxtRouteMiddleware(({ path: toPath }) => {
  const auth = useAuth();

  if (auth.state.value?.mfa_required && toPath !== "/control-plane/account-security") {
    return navigateTo("/control-plane/account-security");
  }
  if (toPath === "/sign-in" && auth.state.value) {
    return navigateTo("/");
  }
  if (toPath !== "/sign-in" && !auth.state.value) {
    return navigateTo("/sign-in");
  }
});
