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
        "app/composables/use-image-control-plane.ts",
        "app/composables/use-tasks-control-plane.ts",
        "app/composables/use-function-control-plane.ts",
        "app/composables/use-file-storage-control-plane.ts",
        "app/composables/use-auth-control-plane.ts",
        "app/composables/use-data-control-plane.ts",
        "app/composables/use-data-workspace.ts",
        "app/utils/data-forms.ts",
        "app/components/data/*.vue",
        "app/pages/data.vue",
        "app/pages/data/*.vue",
        "app/utils/auth-forms.ts",
        "app/utils/auth-records.ts",
        "app/components/auth/*.vue",
        "app/pages/auth.vue",
        "app/pages/auth/*.vue",
        "app/utils/file-storage-forms.ts",
        "app/pages/file-storage.vue",
        "app/pages/file-storage/*.vue",
        "app/pages/function.vue",
        "app/pages/function/*.vue",
        "app/utils/function-forms.ts",
        "app/utils/task-forms.ts",
        "app/components/tasks/*.vue",
        "app/pages/tasks.vue",
        "app/pages/tasks/*.vue",
        "app/pages/image.vue",
        "app/pages/image/*.vue",
        "app/pages/control-plane.vue",
        "app/pages/control-plane/*.vue",
      ],
      thresholds: { statements: 100, branches: 100, functions: 100, lines: 100 },
    },
    projects: [
      await defineVitestProject({
        test: {
          name: "services",
          include: [
            "app/tests/control-plane/**/*.test.ts",
            "app/tests/image/**/*.test.ts",
            "app/tests/tasks/**/*.test.ts",
            "app/tests/function/**/*.test.ts",
            "app/tests/file-storage/**/*.test.ts",
            "app/tests/auth/**/*.test.ts",
            "app/tests/data/**/*.test.ts",
          ],
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
