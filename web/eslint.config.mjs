import { defineConfig, globalIgnores } from "eslint/config";
import js from "@eslint/js";
import tseslint from "typescript-eslint";

const eslintConfig = defineConfig([
  js.configs.recommended,
  ...tseslint.configs.recommended,
  globalIgnores([
    "dist/**",
    "src/routeTree.gen.ts",
    "src/routeTree.gen.js",
    "test-results/**",
    "playwright-report/**",
  ]),
]);

export default eslintConfig;
