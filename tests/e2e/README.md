# Browser E2E Tests (Playwright)

End-to-end browser tests for the deployed web app. By default they run against
`https://httpsms.bigt.ai` (the primary deploy target), but any base URL can be
used via `E2E_BASE_URL`.

## Setup

```sh
cd tests/e2e
npm install
npx playwright install chromium
```

## Running

```sh
npm test                      # all tests (headed: npm run test:headed)
```

Config:

| Env var          | Default                    | Description                                  |
| ---------------- | -------------------------- | -------------------------------------------- |
| `E2E_BASE_URL`   | `https://httpsms.bigt.ai`  | Web app base URL to test against             |
| `E2E_CHANNEL`    | `chrome`                   | Browser to drive. Defaults to real Google Chrome (needed for the Google OAuth test). Set to `chromium` to use the bundled browser |
| `E2E_HEADLESS`   | `false`                    | Set to `true` to run headless. Headed needs a display (`DISPLAY`) |
| `E2E_PASSWORD`   | `e2e-password-123`         | Password used for email sign-up/sign-in      |
| `E2E_EMAIL`      | —                          | Optional: existing account for the sign-in test. Sign-up tests use a fresh random email every run and need **no** pre-provisioned account |
| `E2E_GOOGLE_EMAIL` / `E2E_GOOGLE_PASSWORD` | — | Credentials used by the Google sign-in test. When unset the test is skipped |

Example:

```sh
# full default suite (real Chrome; Google sign-in runs if creds are set)
E2E_GOOGLE_EMAIL=you@gmail.com E2E_GOOGLE_PASSWORD='...' npm test

# CI / headless (bundled chromium, Google test skipped)
E2E_CHANNEL=chromium E2E_HEADLESS=true npx playwright test
```

## Auth & DB user

The app authenticates via Firebase (Google/GitHub popup or email/password).
There is **no separate DB user to provision** for tests:

- Google/Firebase sign-in only creates an identity in Firebase Auth.
- The **DB user row is created lazily** on the first authenticated API call
  (e.g. `GET /v1/users/me` right after login) via `UserService.Get` →
  `UserRepository.LoadOrStore` (`api/pkg/repositories/gorm_user_repository.go`),
  which also generates the `uk_`-prefixed API key.
- The `auth.spec.ts` "auto-creates the DB user" test proves this: after
  email sign-up it opens `/settings` and asserts a `uk_` API key is present and
  the account email is shown.

## Deployment prerequisite

The app routes (`/login`, `/threads`, `/settings`) and API routes (`/v1/*`) must
be reachable at the base URL. The host nginx currently 301-redirects unknown
paths to `http://httpsms.bigt.ai:3000` (which returns 404), so only the landing
page and `/health` respond. Fix the routing (serve `/index.html` for SPA routes,
proxy `/v1/*` to the API) before running the auth specs.

## Notes on Google sign-in

Google blocks OAuth sign-in from automated browsers ("This browser or app may
not be secure"), so the suite defaults to real Google Chrome (`E2E_CHANNEL=chrome`)
and the config passes `--disable-blink-features=AutomationControlled` plus
masks `navigator.webdriver` via a context init script.

- The `google sign-in` test is part of the default run. It only skips when
  `E2E_GOOGLE_EMAIL`/`E2E_GOOGLE_PASSWORD` are unset, or when a non-Chrome
  browser (`E2E_CHANNEL=chromium`) is used.
- For a brand-new (unused) Google account there may be additional verification
  steps that the test can't automate; a warmed-up account with a saved session
  is most reliable.
- Prefer the email sign-up flow for pure CI; it exercises the exact same
  Firebase → DB user path as Google sign-in.
