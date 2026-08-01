import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.E2E_BASE_URL || 'https://httpsms.bigt.ai'
// Default to real Google Chrome so the Google OAuth test works (Google blocks
// sign-in from the automated bundled Chromium). Override with
// E2E_CHANNEL=chromium to use the bundled browser, or E2E_HEADLESS=true to
// run headless.
const channel = process.env.E2E_CHANNEL || 'chrome'
const headless = process.env.E2E_HEADLESS === 'true'

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never' }]]
    : [['list']],
  timeout: 60_000,
  expect: {
    timeout: 15_000,
  },
  use: {
    baseURL,
    channel,
    headless,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    launchOptions: {
      args: ['--disable-blink-features=AutomationControlled'],
    },
  },
  projects: [
    {
      name: channel === 'chromium' ? 'chromium' : 'google-chrome',
      use: { ...devices['Desktop Chrome'], channel, headless },
    },
  ],
})
