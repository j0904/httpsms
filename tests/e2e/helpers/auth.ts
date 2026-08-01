import { expect, Page } from '@playwright/test'

const LAST_LOGIN_METHOD_KEY = 'httpsms_last_login_method'

export function randomEmail() {
  const suffix = Math.random().toString(36).slice(2, 10)
  return `e2e-${suffix}@httpsms.bigt.ai`
}

export async function signUpWithEmail(page: Page, email: string, password: string) {
  await page.goto('/login')
  await expect(page.getByText('Welcome')).toBeVisible()

  await page.getByRole('button', { name: 'Continue with email' }).click()
  await page.getByRole('button', { name: 'No account? Sign Up' }).click()

  const emailInput = page.getByLabel('Email Address')
  await emailInput.fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign Up' }).click()

  await expect(page).toHaveURL(/\/threads/, { timeout: 30_000 })
}

export async function signInWithEmail(page: Page, email: string, password: string) {
  await page.goto('/login')
  await expect(page.getByText('Welcome')).toBeVisible()

  await page.getByRole('button', { name: 'Continue with email' }).click()

  await page.getByLabel('Email Address').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign In' }).click()

  await expect(page).toHaveURL(/\/threads/, { timeout: 30_000 })
}

export async function signInWithGoogle(
  page: Page,
  googleEmail: string,
  googlePassword: string,
) {
  // Mask the automation fingerprint so Google doesn't reject the browser.
  // Must be registered on the context before the popup navigates.
  const context = page.context()
  await context.addInitScript(() => {
    Object.defineProperty(navigator, 'webdriver', { get: () => undefined })
  })

  await page.goto('/login')
  await expect(page.getByText('Welcome')).toBeVisible()

  const popupPromise = page.waitForEvent('popup', { timeout: 30_000 })
  await page.getByRole('button', { name: 'Continue with Google' }).click()
  const popup = await popupPromise
  await popup.waitForLoadState('domcontentloaded')

  // If a Google account is already signed in, the chooser appears instead of
  // the email identifier page. Use the "Use another account" path.
  if (await popup.getByRole('button', { name: 'Use another account' }).isVisible()) {
    await popup.getByRole('button', { name: 'Use another account' }).click()
  }

  const emailInput = popup.getByRole('textbox', { name: /email|phone/i })
  await emailInput.waitFor({ state: 'visible', timeout: 15_000 })
  await emailInput.fill(googleEmail)
  await popup.getByRole('button', { name: 'Next' }).click()

  const passwordInput = popup.getByRole('textbox', { name: /password/i })
  await passwordInput.waitFor({ state: 'visible', timeout: 15_000 })
  await passwordInput.fill(googlePassword)
  await popup.getByRole('button', { name: 'Next' }).click()

  await expect(page).toHaveURL(/\/threads/, { timeout: 60_000 })

  await page.evaluate(
    (key) => localStorage.setItem(key, 'google'),
    LAST_LOGIN_METHOD_KEY,
  )
}

export async function signOut(page: Page) {
  await page.getByRole('button', { name: /sign out|logout|account/i }).click()
  await expect(page).toHaveURL(/\/login/, { timeout: 15_000 })
}
