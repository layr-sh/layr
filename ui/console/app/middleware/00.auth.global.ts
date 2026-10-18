export default defineNuxtRouteMiddleware(({ path: toPath }) => {
  const auth = useAuth();

  console.log(auth.state.value);

  if (toPath === "/sign-in" && auth.state.value) {
    return navigateTo("/");
  }
  if (toPath !== "/sign-in" && !auth.state.value) {
    return navigateTo("/sign-in");
  }
});
