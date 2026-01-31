import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { Notif, AuthError, APIError, ConnectionError } from '../src/index.js'

describe('singleton usage pattern', () => {
  const validApiKey = 'nsh_testkey12345678901234567890'

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shares instance when assigned to module variable', () => {
    const singleton = new Notif({ apiKey: validApiKey })

    // Simulating multiple imports getting same instance
    const ref1 = singleton
    const ref2 = singleton

    expect(ref1).toBe(ref2)
    expect(ref1.serverUrl).toBe(ref2.serverUrl)

    singleton.close()
  })

  it('maintains state across multiple emit calls', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: '1', topic: 'test', seq: 1 })
    })
    vi.stubGlobal('fetch', mockFetch)

    const singleton = new Notif({ apiKey: validApiKey })

    await singleton.emit('topic1', { a: 1 })
    await singleton.emit('topic2', { b: 2 })
    await singleton.emit('topic3', { c: 3 })

    expect(mockFetch).toHaveBeenCalledTimes(3)

    singleton.close()
  })

  it('close() cleans up all active streams', () => {
    const singleton = new Notif({
      apiKey: validApiKey,
      server: 'http://localhost:9999'
    })

    // Create streams (won't connect since no server)
    const stream1 = singleton.subscribe('topic1')
    const stream2 = singleton.subscribe('topic2')

    // Close all
    singleton.close()

    // Streams should be closed
    expect(stream1.isClosed).toBe(true)
    expect(stream2.isClosed).toBe(true)
  })
})

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

  describe('schedule', () => {
    it('sends POST request with scheduledFor option', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 201,
        json: () => Promise.resolve({
          id: 'sch_123',
          topic: 'test.topic',
          scheduled_for: '2025-01-15T10:00:00Z',
          created_at: '2025-01-01T00:00:00Z',
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const scheduledFor = new Date('2025-01-15T10:00:00Z')
      const result = await client.schedule('test.topic', { foo: 'bar' }, { scheduledFor })

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${validApiKey}`,
          },
          body: JSON.stringify({
            topic: 'test.topic',
            data: { foo: 'bar' },
            scheduled_for: '2025-01-15T10:00:00.000Z',
          }),
        })
      )
      expect(result.id).toBe('sch_123')
    })

    it('sends POST request with in option', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 201,
        json: () => Promise.resolve({
          id: 'sch_123',
          topic: 'test.topic',
          scheduled_for: '2025-01-01T00:30:00Z',
          created_at: '2025-01-01T00:00:00Z',
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const result = await client.schedule('test.topic', { foo: 'bar' }, { in: '30m' })

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules',
        expect.objectContaining({
          body: JSON.stringify({
            topic: 'test.topic',
            data: { foo: 'bar' },
            in: '30m',
          }),
        })
      )
      expect(result.id).toBe('sch_123')
    })

    it('throws AuthError on 401 response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.schedule('test', {}, { in: '5m' })).rejects.toThrow(AuthError)
    })
  })

  describe('listSchedules', () => {
    it('sends GET request with query parameters', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          schedules: [
            {
              id: 'sch_123',
              topic: 'test.topic',
              data: { foo: 'bar' },
              scheduled_for: '2025-01-15T10:00:00Z',
              status: 'pending',
              created_at: '2025-01-01T00:00:00Z',
            },
          ],
          total: 1,
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const result = await client.listSchedules({ status: 'pending', limit: 10, offset: 0 })

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules?status=pending&limit=10&offset=0',
        expect.objectContaining({
          method: 'GET',
          headers: {
            'Authorization': `Bearer ${validApiKey}`,
          },
        })
      )
      expect(result.schedules).toHaveLength(1)
      expect(result.total).toBe(1)
    })

    it('sends GET request without parameters', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ schedules: [], total: 0 }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await client.listSchedules()

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules',
        expect.anything()
      )
    })
  })

  describe('getSchedule', () => {
    it('sends GET request with schedule ID', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          id: 'sch_123',
          topic: 'test.topic',
          data: { foo: 'bar' },
          scheduled_for: '2025-01-15T10:00:00Z',
          status: 'pending',
          created_at: '2025-01-01T00:00:00Z',
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const result = await client.getSchedule('sch_123')

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules/sch_123',
        expect.objectContaining({
          method: 'GET',
        })
      )
      expect(result.id).toBe('sch_123')
      expect(result.status).toBe('pending')
    })

    it('throws APIError on 404 response', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        json: () => Promise.resolve({ error: 'schedule not found' }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await expect(client.getSchedule('sch_notfound')).rejects.toThrow(APIError)
    })
  })

  describe('cancelSchedule', () => {
    it('sends DELETE request with schedule ID', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      await client.cancelSchedule('sch_123')

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules/sch_123',
        expect.objectContaining({
          method: 'DELETE',
        })
      )
    })
  })

  describe('runSchedule', () => {
    it('sends POST request to run endpoint', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          schedule_id: 'sch_123',
          event_id: 'evt_456',
        }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const client = new Notif({ apiKey: validApiKey })
      const result = await client.runSchedule('sch_123')

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.notif.sh/api/v1/schedules/sch_123/run',
        expect.objectContaining({
          method: 'POST',
        })
      )
      expect(result.schedule_id).toBe('sch_123')
      expect(result.event_id).toBe('evt_456')
    })
  })
})
