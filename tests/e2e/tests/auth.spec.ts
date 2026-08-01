import { test, expect } from '@playwright/test'
import {
  randomEmail,
  signUpWithEmail,
  signInWithEmail,
  signInWithGoogle,
} from '../helpers/auth'

const E2E_PASSWORD = process.env.E2E_PASSWORD || 'e2e-password-123'

test('sign up with email reaches the threads dashboard', async ({ page }) => {
  const email = randomEmail()
  await signUpWithEmail(page, email, E2E_PASSWORD)
  await expect(page).toHaveTitle(/Threads/i)
})

test('email sign-up auto-creates the DB user with an API key', async ({
  page,
}) => {
  const email = randomEmail()
  await signUpWithEmail(page, email, E2E_PASSWORD)

  await page.goto('/settings')
  await expect(page.getByRole('heading', { name: 'API Key' })).toBeVisible()

  const apiKeyField = page.locator('input[name="api-key"]')
  await expect(apiKeyField).toBeVisible()
  const apiKey = await apiKeyField.inputValue()
  expect(apiKey).toMatch(/^uk_/)

  await expect(page.getByText(email).first()).toBeVisible()
})

test('existing account can sign in', async ({ page }) => {
  test.skip(!process.env.E2E_EMAIL, 'E2E_EMAIL not set')

  await signInWithEmail(page, process.env.E2E_EMAIL!, E2E_PASSWORD)
  await expect(page).toHaveTitle(/Threads/i)
})

test('google sign-in', async ({ page }, testInfo) => {
  test.skip(
    !process.env.E2E_GOOGLE_EMAIL || !process.env.E2E_GOOGLE_PASSWORD,
    'set E2E_GOOGLE_EMAIL and E2E_GOOGLE_PASSWORD to run',
  )
  test.skip(
    testInfo.project.use.channel !== 'chrome',
    'Google sign-in requires real Chrome (default)',
  )

  await signInWithGoogle(
    page,
    process.env.E2E_GOOGLE_EMAIL!,
    process.env.E2E_GOOGLE_PASSWORD!,
  )
  await expect(page).toHaveTitle(/Threads/i)
})
