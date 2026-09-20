import { defineStore } from 'pinia'
import { setApiKey } from '~/composables/useApi'
import { shortDid } from '~/utils/didKey'
import type { EntitiesUser } from '~~/shared/types/api'

export interface AuthUser {
  email: string | null
  displayName: string | null
  id: string
}

// DID agents sign in with an API key instead of a Firebase ID token. Persist
// the session so a reload keeps the user signed in without requiring Firebase.
const DID_SESSION_KEY = 'httpsms_did_session'

interface DidSession {
  apiKey: string
  user: EntitiesUser
}

function saveDidSession(session: DidSession) {
  localStorage.setItem(DID_SESSION_KEY, JSON.stringify(session))
}

function loadDidSession(): DidSession | null {
  try {
    const raw = localStorage.getItem(DID_SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<DidSession>
    if (!parsed.user || !parsed.user.api_key || !parsed.apiKey) return null
    return parsed as DidSession
  } catch (error) {
    console.error(error)
    return null
  }
}

function clearDidSession() {
  localStorage.removeItem(DID_SESSION_KEY)
}

export const useAuthStore = defineStore('auth', () => {
  const authStateChanged = ref(false)
  const authUser = ref<AuthUser | null>(null)
  const user = ref<EntitiesUser | null>(null)
  const { apiFetch } = useApi()

  function applyDidSession(session: DidSession) {
    setApiKey(session.apiKey)
    authUser.value = {
      id: session.user.id,
      email: session.user.email,
      displayName: shortDid(session.user.email),
    }
    user.value = session.user
  }

  async function restoreDidSession(): Promise<boolean> {
    const session = loadDidSession()
    if (!session) {
      authUser.value = null
      user.value = null
      authStateChanged.value = true
      setApiKey('')
      return false
    }
    applyDidSession(session)
    authStateChanged.value = true
    await loadPhones()
    return true
  }

  async function loadUser() {
    const response = await apiFetch<{ data: EntitiesUser }>('/v1/users/me')
    user.value = response.data
  }

  async function updateUser(payload: { owner?: string; timezone?: string }) {
    const phonesStore = usePhonesStore()
    if (payload.owner) {
      phonesStore.setOwner(payload.owner)
    }

    const activePhone = phonesStore.activePhone
    if (!activePhone) return

    const response = await apiFetch<{ data: EntitiesUser }>('/v1/users/me', {
      method: 'PUT',
      body: {
        active_phone_id: activePhone.id,
        timezone: payload.timezone ?? user.value?.timezone,
      },
    })

    setApiKey(response.data.api_key)
    user.value = response.data
  }

  async function deleteUserAccount(): Promise<string> {
    const response = await apiFetch<{ message: string }>('/v1/users/me', {
      method: 'DELETE',
    })
    return response.message
  }

  async function rotateApiKey(userId: string): Promise<EntitiesUser> {
    const response = await apiFetch<{ data: EntitiesUser }>(
      `/v1/users/${userId}/api-keys`,
      {
        method: 'DELETE',
      },
    )
    user.value = response.data
    setApiKey(response.data.api_key)
    return response.data
  }

  function resetState() {
    clearDidSession()
    user.value = null
    authUser.value = null
    authStateChanged.value = true
    setApiKey('')
  }

  function loadPhones() {
    const phonesStore = usePhonesStore()
    return phonesStore.loadPhones(false)
  }

  // Complete a DID sign-in with the user returned by /v1/did-auth/verify.
  async function signInWithDid(session: DidSession) {
    saveDidSession(session)
    applyDidSession(session)
    authStateChanged.value = true
    await loadPhones()
  }

  return {
    authStateChanged,
    authUser,
    user,
    loadUser,
    updateUser,
    deleteUserAccount,
    rotateApiKey,
    resetState,
    restoreDidSession,
    signInWithDid,
  }
})
