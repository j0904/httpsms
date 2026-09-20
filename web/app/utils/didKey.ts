/**
 * Client-side DID key handling for httpSMS.
 *
 * Keys are ML-DSA-87 (FIPS 204), matching the dai/aifeeds wallet: a full
 * serialized private key (`PQKey.getPrivateKeyHex()`) or a 32/64-byte seed.
 * Derivation and challenge signing happen in the browser via ./pq; the private
 * key is never sent to the server. This module only handles key-file parsing
 * and the PBKDF2 -> AES-GCM envelope used for encrypted keys.
 */

// localStorage keys (namespaced "httpsms.did" — independent of the API key
// stored by the auth store for the active session).
const ENCRYPTED_KEY = 'httpsms.did.encrypted'
const DID_KEY = 'httpsms.did'

export interface StoredAgent {
  did: string
  key: string // ML-DSA-87 private key hex (full serialized key, or 32/64-byte seed)
}

/** Shorten a did:key for display (an ML-DSA-87 did is ~3.5 KB). */
export function shortDid(did: string, head = 16, tail = 4): string {
  if (did.length <= head + tail + 1) return did
  return `${did.slice(0, head)}…${did.slice(-tail)}`
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

function hexToBytes(hex: string): Uint8Array<ArrayBuffer> {
  const b = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2)
    b[i / 2] = parseInt(hex.substring(i, i + 2), 16)
  return b
}

/**
 * Validate that a value is an even-length hex ML-DSA private key (>= 64 chars).
 */
export function isHexPrivateKey(value: string): boolean {
  const v = value.trim().toLowerCase().replace(/^0x/, '')
  return v.length >= 64 && v.length % 2 === 0 && /^[0-9a-f]+$/.test(v)
}

/**
 * Parse a plain (decrypted) key file into a {did, key} agent.
 *
 * Accepts:
 *  - `DID=...\nKEY=<hex>` text (bigtai downloadKeyFile format)
 *  - raw hex private key (full serialized key, or 32/64-byte seed)
 *  - JSON {"key": "<hex>"} (optionally nested)
 *  - JSON {"didKey": "<hex>"} (agent did-auth.js key file format)
 *
 * The DID field is optional; it is derived from the key in the browser.
 */
export function importFromFileContent(
  content: string,
): Partial<StoredAgent> | null {
  // JSON envelope (possibly nested)
  try {
    const parsed = JSON.parse(content) as { key?: unknown; didKey?: unknown }
    if (typeof parsed.key === 'string') return importFromFileContent(parsed.key)
    if (typeof parsed.didKey === 'string')
      return importFromFileContent(parsed.didKey)
  } catch {
    /* not JSON, fall through */
  }

  // `DID=<did>\nKEY=<hex>` form
  const didMatch = content.match(/^DID=(.+)$/m)
  const keyMatch = content.match(/^KEY=(.+)$/m)
  if (keyMatch && keyMatch[1]) {
    const key = keyMatch[1].trim().toLowerCase()
    if (!isHexPrivateKey(key)) return null
    const did = didMatch?.[1]?.trim() || undefined
    return { key, ...(did ? { did } : {}) }
  }

  // `didKey=<hex>` or `key=<hex>` form
  const flatKeyMatch = content.match(/^(?:didKey|key)\s*=\s*(.+)$/i)
  if (flatKeyMatch && flatKeyMatch[1]) {
    const key = flatKeyMatch[1].trim().toLowerCase()
    if (!isHexPrivateKey(key)) return null
    return { key }
  }

  // raw 64-char hex
  const cleaned = content
    .replace(/[\r\n\s]+/g, '')
    .trim()
    .toLowerCase()
  if (isHexPrivateKey(cleaned)) {
    return { key: cleaned }
  }

  return null
}

// ── encrypted key file (PBKDF2 -> AES-GCM, bigtai-compatible) ────────────

export function isEncryptedKeyFileContent(content: string): boolean {
  try {
    const parsed = JSON.parse(content) as {
      salt?: unknown
      iv?: unknown
      data?: unknown
    }
    return (
      typeof parsed.salt === 'string' &&
      typeof parsed.iv === 'string' &&
      typeof parsed.data === 'string'
    )
  } catch {
    return false
  }
}

async function deriveAesKey(
  password: string,
  salt: Uint8Array<ArrayBuffer>,
  usage: KeyUsage[],
): Promise<CryptoKey> {
  const pwBytes = new TextEncoder().encode(password)
  const material = await crypto.subtle.importKey(
    'raw',
    pwBytes,
    'PBKDF2',
    false,
    ['deriveKey'],
  )
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: 100_000, hash: 'SHA-256' },
    material,
    { name: 'AES-GCM', length: 256 },
    false,
    usage,
  )
}

/** Decrypt a {salt,iv,data} key file. Returns the plaintext, or null on failure. */
export async function decryptKeyFileContent(
  content: string,
  password: string,
): Promise<string | null> {
  try {
    const parsed = JSON.parse(content) as {
      salt: string
      iv: string
      data: string
    }
    const key = await deriveAesKey(password, hexToBytes(parsed.salt), [
      'decrypt',
    ])
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv: hexToBytes(parsed.iv) },
      key,
      hexToBytes(parsed.data),
    )
    return new TextDecoder().decode(decrypted)
  } catch {
    return null
  }
}

// ── local encrypted storage (bigtai/agentKey parity) ──────────────────────

async function encryptForStorage(
  agent: StoredAgent,
  password: string,
): Promise<string> {
  const salt = new Uint8Array(16)
  crypto.getRandomValues(salt)
  const iv = new Uint8Array(12)
  crypto.getRandomValues(iv)
  const key = await deriveAesKey(password, salt, ['encrypt'])
  const json = JSON.stringify(agent)
  const plaintext = new TextEncoder().encode(json)
  const encrypted = new Uint8Array(
    await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, plaintext),
  )
  return JSON.stringify({
    salt: bytesToHex(salt),
    iv: bytesToHex(iv),
    data: bytesToHex(encrypted),
    v: 1,
  })
}

async function decryptFromStorage(
  encryptedJson: string,
  password: string,
): Promise<StoredAgent | null> {
  try {
    const parsed = JSON.parse(encryptedJson) as {
      salt: string
      iv: string
      data: string
    }
    const key = await deriveAesKey(password, hexToBytes(parsed.salt), [
      'decrypt',
    ])
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv: hexToBytes(parsed.iv) },
      key,
      hexToBytes(parsed.data),
    )
    const agent = JSON.parse(
      new TextDecoder().decode(decrypted),
    ) as Partial<StoredAgent>
    if (!agent.did || !agent.key) return null
    return agent as StoredAgent
  } catch {
    return null
  }
}

export function hasEncryptedAgent(): boolean {
  if (typeof localStorage === 'undefined') return false
  return !!localStorage.getItem(ENCRYPTED_KEY)
}

export function getStoredAgentDid(): string | null {
  if (typeof localStorage === 'undefined') return null
  return localStorage.getItem(DID_KEY)
}

/** Persist the agent encrypted under `password` and stash the DID for display. */
export async function storeEncryptedAgent(
  agent: StoredAgent,
  password: string,
): Promise<void> {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(ENCRYPTED_KEY, await encryptForStorage(agent, password))
  localStorage.setItem(DID_KEY, agent.did)
}

/** Decrypt the stored agent. Returns null if no stored agent or wrong password. */
export async function unlockAgent(
  password: string,
): Promise<StoredAgent | null> {
  if (typeof localStorage === 'undefined') return null
  const encrypted = localStorage.getItem(ENCRYPTED_KEY)
  if (!encrypted) return null
  return decryptFromStorage(encrypted, password)
}

/** Forget the stored agent entirely (used by "use a different key"). */
export function clearEncryptedAgent(): void {
  if (typeof localStorage === 'undefined') return
  localStorage.removeItem(ENCRYPTED_KEY)
  localStorage.removeItem(DID_KEY)
}
