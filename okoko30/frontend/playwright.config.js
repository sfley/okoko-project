import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
 testMatch: "**/*.spec.js",
  timeout: 30000,
  retries: 0,
  workers: 1,
  reporter: [
    ["list"],
    ["json", { outputFile: "../test-results/browser-results.json" }],
  ],
  use: {
    baseURL: process.env.OKOKO_TEST_URL || "http://127.0.0.1:8888",
    headless: true,
    launchOptions: {
      executablePath:
        process.env.CHROMIUM_PATH ||
        (process.platform === "win32" ? undefined : "/usr/bin/chromium"),
      args: ["--no-sandbox", "--disable-dev-shm-usage"],
    },
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  outputDir: "../test-results/browser-artifacts",
});
