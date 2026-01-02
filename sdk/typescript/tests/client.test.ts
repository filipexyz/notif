import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { Notif, AuthError, APIError, ConnectionError } from '../src/index.js'

describe('Notif', () => {
  const validApiKey = 'nsh_testkey12345678901234567890'

  beforeEach(() => {
    vi.stubEnv('NOTIF_API_KEY', '')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.restoreAllMocks()
  })

  describe('constructor', () => {
    it('throws AuthError when no API key provided', () => {
      expect(() => new Notif()).toThrow(AuthError)
      expect(() => new Notif()).toThrow(/API key not provided/)
    })

    it('throws AuthError when API key has invalid prefix', () => {
      expect(() => new Notif({ apiKey: 'invalid_key' })).toThrow(AuthError)
      expect(() => new Notif({ apiKey: 'invalid_key' })).toThrow(/must start with 'nsh_'/)
    })

    it('accepts valid API key from options', () => {
      const client = new Notif({ apiKey: validApiKey })
      expect(client).toBeInstanceOf(Notif)
    })

    it('reads API key from environment variable', () => {
      vi.stubEnv('NOTIF_API_KEY', validApiKey)
      const client = new Notif()
      expect(client).toBeInstanceOf(Notif)
    })

    it('prefers options API key over environment variable', () => {
      vi.stubEnv('NOTIF_API_KEY', 'nsh_envkey12345678901234567890')
      const client = new Notif({ apiKey: validApiKey })
      expect(client.serverUrl).toBe('https://api.notif.sh')
    })

    it('uses default server URL', () => {
      const client = new Notif({ apiKey: validApiKey })
      expect(client.serverUrl).toBe('https://api.notif.sh')
    })

    it('uses custom server URL from options', () => {
      const client = new Notif({
        apiKey: validApiKey,
        server: 'http://localhost:8080',
      })
      expect(client.serverUrl).toBe('http://localhost:8080')
    })
  })

  describe('emit', () => {
    it('sends POST request with correct headers and body', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          id: 'evt_123',
          topic: 'test.topic',
          created_at: '2025-01-01T00:00:00Z',
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const result = await client.emit('test.topic', { foo: 'bar' })

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/emit',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${validApiKey}`,
          },
          body: JSON.stringify({ topic: 'test.topic', data: { foo: 'bar' } }),
        })
      )
      expect(result.id).toBe('evt_123')
      expect(result.topic).toBe('test.topic')
    })

    it('throws AuthError on 401 response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.emit('test', {})).rejects.toThrow(AuthError)
    })

    it('throws APIError on 400 response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        json: () => Promise.resolve({ error: 'topic is required' }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.emit('', {})).rejects.toThrow(APIError)
      await expect(client.emit('', {})).rejects.toThrow('topic is required')
    })

    it('throws APIError on 500 response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.resolve({ error: 'internal error' }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.emit('test', {})).rejects.toThrow(APIError)
    })

    it('throws ConnectionError on network failure', async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error('Network error'))
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.emit('test', {})).rejects.toThrow(ConnectionError)
    })

    it('throws ConnectionError on timeout', async () => {
      const mockFetch = vi.fn().mockImplementation(() => {
        const error = new Error('Aborted')
        error.name = 'AbortError'
        return Promise.reject(error)
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey, timeout: 100 })
      await expect(client.emit('test', {})).rejects.toThrow(ConnectionError)
      await expect(client.emit('test', {})).rejects.toThrow('request timeout')
    })
  })

  describe('subscribe', () => {
    it('throws error when no topics provided', () => {
      const client = new Notif({ apiKey: validApiKey })
      expect(() => client.subscribe([])).toThrow('At least one topic is required')
    })

    it('accepts single topic as string', () => {
      const client = new Notif({ apiKey: validApiKey })
      const stream = client.subscribe('test.*')
      expect(stream).toBeDefined()
      stream.close()
      client.close()
    })

    it('accepts multiple topics', () => {
      const client = new Notif({ apiKey: validApiKey })
      const stream = client.subscribe('orders.*', 'leads.*')
      expect(stream).toBeDefined()
      stream.close()
      client.close()
    })

    it('accepts topics array', () => {
      const client = new Notif({ apiKey: validApiKey })
      const stream = client.subscribe(['orders.*', 'leads.*'])
      expect(stream).toBeDefined()
      stream.close()
      client.close()
    })

    it('accepts options as last argument', () => {
      const client = new Notif({ apiKey: validApiKey })
      const stream = client.subscribe('test.*', { autoAck: false, from: 'beginning' })
      expect(stream).toBeDefined()
      stream.close()
      client.close()
    })
  })

  describe('close', () => {
    it('closes all active streams', () => {
      const client = new Notif({ apiKey: validApiKey })
      const stream1 = client.subscribe('topic1')
      const stream2 = client.subscribe('topic2')

      client.close()

      expect(stream1.isConnected).toBe(false)
      expect(stream2.isConnected).toBe(false)
    })
  })
})
