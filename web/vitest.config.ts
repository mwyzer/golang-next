import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [tsconfigPaths(), react()],
  // tsconfig.json sets "jsx": "preserve" for Next's own compiler; without
  // this override esbuild inherits that and falls back to the classic
  // transform, which needs `React` in scope even though these
  // components rely on the automatic runtime.
  esbuild: {
    jsx: "automatic",
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
  },
});
