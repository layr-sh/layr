import { defineConfig } from "vitest/config";
import { defineVitestProject } from "@nuxt/test-utils/config";
// import { resolve } from "node:path";

export default defineConfig({
  resolve: {
    alias: {
      // "@nuxt/test-utils/e2e": resolve(process.cwd(), "server/tests/helper.ts"),
    },
  },
  test: {
    globals: true,
    // globalSetup: ["server/tests/global-setup.ts"],
    testTimeout: 120000,
    hookTimeout: 120000,
    pool: "forks",
    isolate: true,
    projects: [
      {
        test: {
          name: "server",
          include: ["server/tests/**/*.test.ts"],
          environment: "node",
        },
      },
      await defineVitestProject({
        test: {
          name: "unit",
          include: ["app/tests/unit/**/*.test.ts"],
          environment: "nuxt",
          // setupFiles: ["app/tests/setup.ts"],
        },
      }),
      await defineVitestProject({
        test: {
          name: "integration",
          include: ["app/tests/integration/**/*.test.ts"],
          environment: "nuxt",
          // setupFiles: ["app/tests/setup.ts"],
        },
      }),
      await defineVitestProject({
        test: {
          name: "e2e",
          include: ["app/tests/e2e/**/*.test.ts"],
          environment: "nuxt",
          // setupFiles: ["app/tests/setup.ts"],
        },
      }),
      {
        test: {
          name: "tools",
          include: ["tools/**/*.test.ts"],
          environment: "node",
        },
      },
    ],
  },
});
