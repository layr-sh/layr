import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    include: ["tests/**/!(*.d).test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      include: ["src/**/*.ts"],
    },
    typecheck: {
      enabled: true,
      include: ["**/*.d.test.ts"],
    },
  },
});
