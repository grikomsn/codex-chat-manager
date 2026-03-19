import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
  },
  resolve: {
    alias: {
      "@raycast/api": "./tests/__mocks__/@raycast/api.ts",
    },
  },
});
