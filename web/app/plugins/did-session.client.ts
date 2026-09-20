import { useAuthStore } from '../stores/auth'

// Restore the persisted DID session on startup so route middleware never
// hangs waiting for auth readiness. This replaces the Firebase auth plugin:
// sign-in is DID + API key based, no Google services involved.
export default defineNuxtPlugin(async () => {
  const authStore = useAuthStore()
  await authStore.restoreDidSession()
})
