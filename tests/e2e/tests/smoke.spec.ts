import { test, expect } from '@playwright/test'

test('landing page loads for httpsms.bigt.ai', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(/httpSMS/)
  await expect(page.getByText(/SMS gateway/i).first()).toBeVisible()
})

test('api health endpoint is reachable', async ({ request }) => {
  const resp = await request.get('/health')
  expect(resp.status()).toBe(200)
  expect(await resp.text()).toBe('OK')
})

test('unauthenticated users are redirected to /login', async ({ page }) => {
  await page.goto('/threads')
  await expect(page).toHaveURL(/\/login/)
  await expect(page.getByText('Welcome')).toBeVisible()
})

test('login page shows Google, GitHub and email options', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('button', { name: 'Continue with Google' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Continue with GitHub' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Continue with email' })).toBeVisible()
})
