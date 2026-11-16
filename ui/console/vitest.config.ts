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
    coverage: {
      provider: "v8",
      reportsDirectory: "../../.cache/coverage/console",
      include: [
        "app/composables/use-control-plane.ts",
        "app/composables/use-auth.ts",
        "app/pages/control-plane.vue",
        "app/pages/control-plane/*.vue",
      ],
      thresholds: { statements: 100, branches: 100, functions: 100, lines: 100 },
    },
    projects: [
      await defineVitestProject({
        test: {
          name: "control-plane",
          include: ["app/tests/control-plane/**/*.test.ts"],
          environment: "nuxt",
          testTimeout: 120000,
          hookTimeout: 120000,
          fileParallelism: false,
          environmentOptions: {
            nuxt: { url: process.env.LAYR__SERVER__BASE_URL },
          },
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
