import { defineConfig, devices } from '@playwright/test'
export default defineConfig({
  testDir: './tests/browser',
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  use: {
    baseURL: process.env.TEST_BASE_URL || 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // page.route() cannot see requests made by the media service worker.
    serviceWorkers: 'block',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } },
    },
  ],
  webServer: process.env.TEST_EXTERNAL_SERVER
    ? undefined
    : {
        command: 'npm run build && node .output/server/index.mjs',
        url: 'http://127.0.0.1:4173',
        reuseExistingServer: false,
        env: { HOST: '127.0.0.1', PORT: '4173', NUXT_DEMO: 'true', NUXT_PUBLIC_API_BASE: '/api/v1', NUXT_PUBLIC_KLIPY_API_KEY: 'test-app-key' },
      },
})
