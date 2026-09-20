<script setup lang="ts">
import { mdiFingerprint, mdiKeyVariant, mdiUpload } from '@mdi/js'
import {
  clearEncryptedAgent,
  decryptKeyFileContent,
  getStoredAgentDid,
  hasEncryptedAgent,
  importFromFileContent,
  isEncryptedKeyFileContent,
  storeEncryptedAgent,
  unlockAgent,
  type StoredAgent,
} from '~/utils/didKey'
import {
  didFromPQKey,
  pqKeyFromPrivateKeyHex,
  signChallengePQ,
} from '~/utils/pq'
import type { EntitiesUser } from '~~/shared/types/api'

const props = withDefaults(
  defineProps<{
    to?: string
  }>(),
  { to: '/threads' },
)

const router = useRouter()
const authStore = useAuthStore()
const notificationsStore = useNotificationsStore()

const { apiFetch } = useApi()

const showDid = ref(false)
const hasStored = ref(false)
const storedDid = ref<string | null>(null)

const fileName = ref('')
const fileText = ref('')
const encrypted = ref(false)
const agent = ref<StoredAgent | null>(null)

const password = ref('')
const confirmPassword = ref('')

const loading = ref(false)
const error = ref('')

const fileInputRef = ref<HTMLInputElement | null>(null)

onMounted(() => {
  if (hasEncryptedAgent()) {
    hasStored.value = true
    storedDid.value = getStoredAgentDid()
    showDid.value = true
  }
})

function resetLoader() {
  fileName.value = ''
  fileText.value = ''
  encrypted.value = false
  agent.value = null
  password.value = ''
  confirmPassword.value = ''
  error.value = ''
}

function resetToLoader() {
  clearEncryptedAgent()
  hasStored.value = false
  storedDid.value = null
  resetLoader()
}

async function loadFromText(
  text: string,
  pw: string,
): Promise<StoredAgent | null> {
  if (isEncryptedKeyFileContent(text)) {
    if (!pw) {
      error.value = 'This key file is password-protected. Enter its password.'
      agent.value = null
      return null
    }
    const plain = await decryptKeyFileContent(text, pw)
    if (!plain) {
      error.value = 'Wrong password for key file.'
      agent.value = null
      return null
    }
    const parsed = importFromFileContent(plain)
    if (!parsed?.key) {
      error.value = 'Could not read an ML-DSA key from this file.'
      agent.value = null
      return null
    }
    agent.value = parsed as StoredAgent
    error.value = ''
    return parsed as StoredAgent
  }

  const parsed = importFromFileContent(text)
  if (!parsed?.key) {
    error.value =
      'Unrecognized key file format. Expected DID=/KEY=, raw ML-DSA hex, or {salt,iv,data} JSON.'
    agent.value = null
    return null
  }
  agent.value = parsed as StoredAgent
  error.value = ''
  return parsed as StoredAgent
}

async function handleFile(file: File | undefined) {
  if (!file) return
  fileName.value = file.name
  error.value = ''
  try {
    const text = await file.text()
    fileText.value = text
    encrypted.value = isEncryptedKeyFileContent(text)
    agent.value = null
    await loadFromText(text, password.value)
  } catch {
    error.value = 'Failed to read file.'
    agent.value = null
  }
  if (fileInputRef.value) fileInputRef.value.value = ''
}

async function onUnlockFile() {
  if (!fileText.value) {
    error.value = 'Select a key file first.'
    return
  }
  await loadFromText(fileText.value, password.value)
}

async function verifyAndSignIn(
  currentAgent: StoredAgent,
  storagePassword: string,
) {
  const key = pqKeyFromPrivateKeyHex(currentAgent.key)
  const did = didFromPQKey(key)

  const challengeResponse = await apiFetch<{
    data: { id: string; nonce: string }
  }>('/v1/did-auth/challenge', {
    method: 'POST',
    body: { did },
  })

  const verifyResponse = await apiFetch<{ data: EntitiesUser }>(
    '/v1/did-auth/verify',
    {
      method: 'POST',
      body: {
        id: challengeResponse.data.id,
        did,
        nonce: challengeResponse.data.nonce,
        signature: signChallengePQ(key, challengeResponse.data.nonce),
        scheme: 'mldsa',
      },
    },
  )

  await authStore.signInWithDid({
    apiKey: verifyResponse.data.api_key,
    user: verifyResponse.data,
  })

  if (storagePassword) {
    await storeEncryptedAgent({ did, key: currentAgent.key }, storagePassword)
  }

  notificationsStore.addNotification({
    message: 'Login successful!',
    type: 'success',
  })
  router.push({ path: props.to })
}

async function onUnlockStored() {
  error.value = ''
  loading.value = true
  try {
    const unlocked = await unlockAgent(password.value)
    if (!unlocked) {
      error.value = 'Wrong password.'
      return
    }
    await verifyAndSignIn(unlocked, '')
  } catch (err: unknown) {
    error.value =
      (err as { data?: { message?: string }; message?: string }).data
        ?.message ||
      (err as { message?: string }).message ||
      'Unlock failed.'
  } finally {
    loading.value = false
  }
}

async function handleSignIn() {
  error.value = ''
  if (!agent.value) {
    error.value = 'Load a key file first.'
    return
  }
  if (!encrypted.value && !password.value) {
    error.value = 'Set a password to encrypt the key locally.'
    return
  }
  if (!encrypted.value && password.value.length < 6) {
    error.value = 'Password must be at least 6 characters.'
    return
  }
  if (!encrypted.value && password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match.'
    return
  }

  loading.value = true
  try {
    await verifyAndSignIn(agent.value, password.value)
  } catch (err: unknown) {
    const msg =
      (err as { data?: { message?: string }; message?: string }).data
        ?.message ||
      (err as { message?: string }).message ||
      'Authentication failed.'
    error.value = msg
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <template v-if="!showDid">
      <div class="d-flex align-center justify-center mb-4">
        <VDivider class="mr-3" />
        <span class="text-caption text-medium-emphasis">or</span>
        <VDivider class="ml-3" />
      </div>
      <v-btn
        block
        size="large"
        variant="outlined"
        color="primary"
        class="mb-3"
        :disabled="loading"
        @click="showDid = true"
      >
        <v-icon :icon="mdiFingerprint" class="mr-2" />
        Sign in with DID
      </v-btn>
    </template>

    <template v-else>
      <div class="d-flex align-center justify-center mb-4">
        <VDivider class="mr-3" />
        <span class="text-caption text-medium-emphasis"
          >DID Authentication</span
        >
        <VDivider class="ml-3" />
      </div>

      <v-alert v-if="error" type="error" density="compact" class="mb-3">
        {{ error }}
      </v-alert>

      <!-- Returning user: unlock the stored agent with a password -->
      <template v-if="hasStored">
        <p
          v-if="storedDid"
          class="text-caption text-medium-emphasis break-all text-center mb-2"
        >
          {{ storedDid }}
        </p>
        <v-text-field
          v-model="password"
          label="Password"
          type="password"
          color="primary"
          variant="outlined"
          density="comfortable"
          class="mb-2"
          :disabled="loading"
          @keydown.enter="onUnlockStored"
        />
        <v-btn
          block
          size="large"
          color="primary"
          :loading="loading"
          :disabled="loading || !password"
          @click="onUnlockStored"
        >
          Unlock
        </v-btn>
        <v-btn
          block
          variant="text"
          size="small"
          color="warning"
          class="mt-2"
          :disabled="loading"
          @click="resetToLoader"
        >
          Use a different key
        </v-btn>
      </template>

      <!-- File loader -->
      <template v-else>
        <v-text-field
          v-if="fileText"
          v-model="password"
          :label="
            encrypted
              ? 'Key file password'
              : 'Set a password to encrypt locally'
          "
          type="password"
          color="primary"
          variant="outlined"
          density="comfortable"
          class="mb-2"
          :disabled="loading"
        />

        <div
          class="d-flex flex-column align-center justify-center ga-2 pa-8 text-center cursor-pointer mb-2 did-dropzone"
          @click="fileInputRef?.click"
        >
          <v-icon size="32" color="medium-emphasis" :icon="mdiUpload" />
          <span class="text-body-medium font-weight-medium">
            {{ fileName || 'Load key file' }}
          </span>
          <span class="text-caption text-medium-emphasis"
            >Click to browse (.key, .json, .txt)</span
          >
        </div>
        <input
          ref="fileInputRef"
          type="file"
          accept=".key,.json,.txt"
          class="d-none"
          @change="handleFile(($event.target as HTMLInputElement).files?.[0])"
        />

        <!-- Encrypted file: explicit unlock button -->
        <v-btn
          v-if="fileText && encrypted && !agent"
          block
          size="large"
          variant="outlined"
          color="primary"
          class="mb-2"
          :disabled="!password || loading"
          @click="onUnlockFile"
        >
          Unlock key
        </v-btn>

        <!-- Decrypted agent: show the derived DID -->
        <p
          v-if="agent"
          class="text-caption text-medium-emphasis break-all text-center mb-2"
        >
          {{ agent.did || 'Resolving DID...' }}
        </p>

        <!-- Plain file: capture + confirm the local-storage password -->
        <v-text-field
          v-if="agent && !encrypted && !password"
          v-model="password"
          label="Set a password to encrypt locally"
          type="password"
          color="primary"
          variant="outlined"
          density="comfortable"
          class="mb-2"
          :disabled="loading"
        />
        <v-text-field
          v-if="agent && !encrypted && password"
          v-model="confirmPassword"
          label="Confirm password"
          type="password"
          color="primary"
          variant="outlined"
          density="comfortable"
          class="mb-2"
          :disabled="loading"
        />

        <v-btn
          block
          size="large"
          color="primary"
          :loading="loading"
          :disabled="loading || !agent"
          @click="handleSignIn"
        >
          <v-icon :icon="mdiKeyVariant" class="mr-2" />
          Sign In with DID
        </v-btn>
      </template>

      <v-btn
        block
        variant="text"
        size="small"
        color="warning"
        class="mt-2"
        :disabled="loading"
        @click="
          () => {
            resetLoader()
            showDid = false
          }
        "
      >
        Back to sign in
      </v-btn>
    </template>
  </div>
</template>

<style scoped>
.did-dropzone {
  border: 2px dashed rgb(var(--v-border-color));
  border-radius: 8px;
}
</style>
