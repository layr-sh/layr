// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: false },
  ssr: false,
  css: ["~/assets/styles/main.css"],
  typescript: { strict: true },
  app: {
    baseURL: "/console/",
  },
  devServer: {
    host: "127.0.0.1",
    port: 3000,
  },
  modules: ["@nuxt/eslint", "@vueuse/nuxt", "@nuxt/ui", "@nuxt/image"],
  ui: {
    // We're can't use @nuxt/fonts for static build (without nuxt server)
    fonts: false,
  },
});
