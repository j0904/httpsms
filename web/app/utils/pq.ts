/**
 * ML-DSA-87 did:key support for the httpSMS browser DID login.
 *
 * Mirrors @aifeeds/did/pq (the dai wallet) so the browser signs the exact
 * bapp wire format the httpSMS Go verifier checks:
 *   did:key:z base58btc(uvarint(0x300001) ‖ 0x05 ‖ KeyBundle)
 *   SignatureBundle over sha256("MLDSA-SIG-DOMAIN" ‖ sha256("BIGTANGLE-PQ-TX-v1" ‖ nonce))
 * Dual keys add an SLH-DSA-SHA2-256s entry, signed over
 * sha256("SLHDSA-SIG-DOMAIN" ‖ txHash).
 *
 * The private key never leaves the browser; only the signature is sent.
 */
import { sha256 } from '@noble/hashes/sha2.js'
import { ml_dsa87 } from '@noble/post-quantum/ml-dsa.js'
import { slh_dsa_sha2_256s } from '@noble/post-quantum/slh-dsa.js'

const BASE58 = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz'
const MULTICODEC_MLDSA87 = 0x300001
const ALG_ML_DSA_87 = 1
const ALG_SLH_DSA_SHA2_256S = 2
const BUNDLE_VERSION = 1
const PQ_PUB_PREFIX = 0x05
const MLDSA_PRIVATE_BYTES = 4896
const SLHDSA_SEED_BYTES = 96
const TX_DOMAIN = 'BIGTANGLE-PQ-TX-v1'
const MLDSA_SIG_DOMAIN = 'MLDSA-SIG-DOMAIN'
const SLHDSA_SIG_DOMAIN = 'SLHDSA-SIG-DOMAIN'

export interface PQKey {
  mlPrivateKey: Uint8Array
  publicKey: Uint8Array
  slhPrivateKey?: Uint8Array
  slhPublicKey?: Uint8Array
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

function hexToBytes(hex: string): Uint8Array {
  const clean = hex.trim().toLowerCase().replace(/^0x/, '')
  if (clean.length % 2 !== 0 || !/^[0-9a-f]*$/.test(clean)) {
    throw new Error('not a hex string')
  }
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++)
    out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
  return out
}

function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0))
  let offset = 0
  for (const part of parts) {
    out.set(part, offset)
    offset += part.length
  }
  return out
}

function base58Encode(input: Uint8Array): string {
  let zeros = 0
  while (zeros < input.length && input[zeros] === 0) zeros++

  let num = 0n
  for (const byte of input.subarray(zeros)) num = (num << 8n) + BigInt(byte)

  let out = ''
  while (num > 0n) {
    out = BASE58[Number(num % 58n)] + out
    num /= 58n
  }
  return '1'.repeat(zeros) + out
}

function encodeUvarint(value: number): Uint8Array {
  const out: number[] = []
  let v = value
  while (v > 0x7f) {
    out.push((v & 0x7f) | 0x80)
    v >>>= 7
  }
  out.push(v)
  return Uint8Array.from(out)
}

function domainSeparatedHash(data: Uint8Array, domain: string): Uint8Array {
  return sha256(concat(new TextEncoder().encode(domain), data))
}

// BC DigestRandomGenerator(SHA256): seed → ξ, matching bigtangle-ts sha256Drbg.
function sha256Drbg(seed: Uint8Array, outputLen: number): Uint8Array {
  const hashLen = 32
  let d = sha256.create()
  if (seed.length > 0) d.update(seed)
  let seedBuf = new Uint8Array(hashLen)
  d.update(seedBuf)
  seedBuf = new Uint8Array(d.digest())

  let stateBuf = new Uint8Array(hashLen)
  let counter = 1n
  const result = new Uint8Array(outputLen)
  let offset = 0
  while (offset < outputLen) {
    const oldCounter = counter
    counter += 1n
    d = sha256.create()
    for (let i = 0; i < 8; i++) {
      d.update(new Uint8Array([Number((oldCounter >> BigInt(i * 8)) & 0xffn)]))
    }
    d.update(stateBuf)
    d.update(seedBuf)
    stateBuf = new Uint8Array(d.digest())
    const copyLen = Math.min(hashLen, outputLen - offset)
    result.set(stateBuf.subarray(0, copyLen), offset)
    offset += copyLen
  }
  return result
}

function mlKeyFromSeed(
  seed: Uint8Array,
): Pick<PQKey, 'mlPrivateKey' | 'publicKey'> {
  const { secretKey, publicKey } = ml_dsa87.keygen(sha256Drbg(seed, 32))
  return { mlPrivateKey: secretKey, publicKey }
}

function keyFromSeed(seed: Uint8Array): PQKey {
  return mlKeyFromSeed(seed)
}

function dualKeyFromSeeds(mlSeed: Uint8Array, slhSeed: Uint8Array): PQKey {
  const ml = mlKeyFromSeed(mlSeed)
  const { secretKey, publicKey } = slh_dsa_sha2_256s.keygen(
    sha256Drbg(slhSeed, SLHDSA_SEED_BYTES),
  )
  return { ...ml, slhPrivateKey: secretKey, slhPublicKey: publicKey }
}

/**
 * Parse a bigtangle-ts PQKey private key: a 32-byte ML-DSA seed (64 hex),
 * a 64-byte dual seed (128 hex), or the full serialized private key
 * (`getPrivateKeyHex()`, ~9808 hex / longer for dual keys).
 */
export function pqKeyFromPrivateKeyHex(hex: string): PQKey {
  const bytes = hexToBytes(hex)
  if (bytes.length === 32) return keyFromSeed(bytes)
  if (bytes.length === 64)
    return dualKeyFromSeeds(bytes.subarray(0, 32), bytes.subarray(32, 64))

  if (bytes.length >= 8) {
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
    const mlLen = view.getUint32(0, false)
    if (mlLen === MLDSA_PRIVATE_BYTES && 4 + mlLen + 4 <= bytes.length) {
      const mlPrivateKey = bytes.slice(4, 4 + mlLen)
      const key: PQKey = {
        mlPrivateKey,
        publicKey: ml_dsa87.getPublicKey(mlPrivateKey),
      }

      const slLen = view.getUint32(4 + mlLen, false)
      if (slLen > 0 && 8 + mlLen + slLen <= bytes.length) {
        const slhPrivateKey = bytes.slice(8 + mlLen, 8 + mlLen + slLen)
        key.slhPrivateKey = slhPrivateKey
        key.slhPublicKey = slh_dsa_sha2_256s.getPublicKey(slhPrivateKey)
      }
      return key
    }
  }
  throw new Error('unrecognized ML-DSA private key')
}

// A bundle entry: algorithm ‖ uint16 big-endian length ‖ bytes.
function bundleEntry(algorithm: number, bytes: Uint8Array): Uint8Array {
  return concat(
    Uint8Array.from([
      algorithm,
      (bytes.length >>> 8) & 0xff,
      bytes.length & 0xff,
    ]),
    bytes,
  )
}

/** Prefixed public key: 0x05 ‖ KeyBundle(ML-DSA-87 [, SLH-DSA-SHA2-256s]). */
function prefixedPublicKey(key: PQKey): Uint8Array {
  const entries = [bundleEntry(ALG_ML_DSA_87, key.publicKey)]
  if (key.slhPublicKey)
    entries.push(bundleEntry(ALG_SLH_DSA_SHA2_256S, key.slhPublicKey))
  return concat(
    Uint8Array.from([PQ_PUB_PREFIX, BUNDLE_VERSION, entries.length]),
    ...entries,
  )
}

export function didFromPQKey(key: PQKey): string {
  return `did:key:z${base58Encode(concat(encodeUvarint(MULTICODEC_MLDSA87), prefixedPublicKey(key)))}`
}

/** Sign a hex nonce; returns the hex-serialized SignatureBundle. */
export function signChallengePQ(key: PQKey, nonceHex: string): string {
  const txHash = domainSeparatedHash(hexToBytes(nonceHex), TX_DOMAIN)

  const mlMsg = domainSeparatedHash(txHash, MLDSA_SIG_DOMAIN)
  const entries = [
    bundleEntry(ALG_ML_DSA_87, ml_dsa87.sign(mlMsg, key.mlPrivateKey)),
  ]

  if (key.slhPrivateKey) {
    const slhMsg = domainSeparatedHash(txHash, SLHDSA_SIG_DOMAIN)
    entries.push(
      bundleEntry(
        ALG_SLH_DSA_SHA2_256S,
        slh_dsa_sha2_256s.sign(slhMsg, key.slhPrivateKey),
      ),
    )
  }

  return bytesToHex(
    concat(Uint8Array.from([BUNDLE_VERSION, entries.length]), ...entries),
  )
}
