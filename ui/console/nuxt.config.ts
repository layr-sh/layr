// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },
  css: ["~/assets/styles/main.css"],
  modules: ["@nuxt/eslint", "@vueuse/nuxt", "@nuxt/ui"],
  typescript: {
    strict: true,
  },
});
