import { existsSync } from 'node:fs'

import { defineConfig, devices } from '@playwright/test'

// Reuse a local Chrome when Playwright's own browser is not installed.
const chrome = process.env.PW_CHROME_PATH
const executablePath = chrome && existsSync(chrome) ? chrome : undefined

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:8099',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], launchOptions: { executablePath } } }],
  webServer: [
    {
      // Color assertions in the suite use the blue-black theme.
      command: 'sh e2e/serve.sh 8099 yes blue-black',
      url: 'http://127.0.0.1:8099/healthz',
      reuseExistingServer: false,
    },
    {
      command: 'sh e2e/serve.sh 8098 no',
      url: 'http://127.0.0.1:8098/healthz',
      reuseExistingServer: false,
    },
    {
      // Served under /panel/; reached directly (prefix kept by the proxy)…
      command: 'sh e2e/serve.sh 8096 yes',
      env: { E2E_BASE_PATH: '/panel/' },
      url: 'http://127.0.0.1:8096/panel/healthz',
      reuseExistingServer: false,
    },
    {
      // …and through a proxy that strips it, like `proxy_pass http://upstream/;`.
      command: 'node e2e/strip-proxy.mjs 8095 8096 /panel/',
      url: 'http://127.0.0.1:8095/panel/healthz',
      reuseExistingServer: false,
    },
  ],
})
