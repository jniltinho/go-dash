import { existsSync } from 'node:fs'

import { defineConfig, devices } from '@playwright/test'

// Regenerates docs/screenshots (see `make screenshots`); not part of the e2e suite.
const chrome = process.env.PW_CHROME_PATH
const executablePath = chrome && existsSync(chrome) ? chrome : undefined

export default defineConfig({
  testDir: './screenshots',
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:8097',
    deviceScaleFactor: 1,
  },
  projects: [
    {
      name: 'chromium',
      // The viewport must come after the device preset, which sets its own.
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 480 },
        launchOptions: { executablePath },
      },
    },
  ],
  webServer: {
    command: 'sh screenshots/serve-demo.sh 8097',
    url: 'http://127.0.0.1:8097/healthz',
    reuseExistingServer: false,
  },
})
