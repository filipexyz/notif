import { DEFAULT_SERVER, DEFAULT_TIMEOUT, ENV_VAR_NAME, API_KEY_PREFIX } from './constants.js'
import { AuthError, APIError, ConnectionError, NotifError } from './errors.js'
import {
  NotifOptions,
  SubscribeOptions,
  EmitResponse,
  ScheduleOptions,
  CreateScheduleResponse,
  Schedule,
  ListSchedulesOptions,
  ListSchedulesResponse,
  RunScheduleResponse,
} from './types.js'
import { EventStream } from './events.js'

export class Notif {
  private readonly apiKey: string
  private readonly server: string
  private readonly timeout: number
  private activeStreams: Set<EventStream> = new Set()

  constructor(options: NotifOptions = {}) {
    const apiKey = options.apiKey ?? process.env[ENV_VAR_NAME]

    if (!apiKey) {
      throw new AuthError(`API key not provided. Set ${ENV_VAR_NAME} environment variable or pass apiKey option.`)
    }

    if (!apiKey.startsWith(API_KEY_PREFIX)) {
      throw new AuthError(`API key must start with '${API_KEY_PREFIX}'`)
    }

    this.apiKey = apiKey
    this.server = options.server ?? DEFAULT_SERVER
    this.timeout = options.timeout ?? DEFAULT_TIMEOUT
  }

  async emit(topic: string, data: Record<string, unknown>): Promise<EmitResponse> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(`${this.server}/api/v1/emit`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.apiKey}`,
        },
        body: JSON.stringify({ topic, data }),
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'emit failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'emit failed')
      }

      return await response.json() as EmitResponse
    } catch (error) {
      clearTimeout(timeoutId)

      if (error instanceof NotifError) throw error

      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }

      throw new ConnectionError('network error', error as Error)
    }
  }

  /**
   * Subscribe to events on the given topics.
   *
   * @param topics - One or more topic patterns (e.g., 'orders.*')
   * @param options - Subscription options
   * @returns AsyncIterable that yields events
   *
   * @example
   * // Single topic
   * for await (const event of client.subscribe('orders.*')) { ... }
   *
   * // Multiple topics
   * for await (const event of client.subscribe('orders.*', 'leads.*')) { ... }
   *
   * // With options
   * for await (const event of client.subscribe('orders.*', { autoAck: false })) {
   *   await event.ack()
   * }
   */
  subscribe(...args: [...string[], SubscribeOptions] | string[] | [string[]] | [string[], SubscribeOptions]): EventStream {
    // Parse arguments: variadic topics with optional options object at the end
    let topics: string[]
    let options: SubscribeOptions = {}

    const lastArg = args[args.length - 1]
    if (typeof lastArg === 'object' && lastArg !== null && !Array.isArray(lastArg)) {
      options = lastArg as SubscribeOptions
      topics = args.slice(0, -1) as string[]
    } else {
      topics = args as string[]
    }

    // Handle array as first argument: subscribe(['a', 'b']) or subscribe(['a', 'b'], options)
    if (topics.length >= 1 && Array.isArray(topics[0])) {
      topics = topics[0] as unknown as string[]
    }

    if (topics.length === 0) {
      throw new Error('At least one topic is required')
    }

    const stream = new EventStream(this.apiKey, this.server, topics, options)
    this.activeStreams.add(stream)

    // Remove from active streams when closed
    stream.onClose(() => {
      this.activeStreams.delete(stream)
    })

    return stream
  }

  close(): void {
    for (const stream of this.activeStreams) {
      stream.close()
    }
    this.activeStreams.clear()
  }

  /**
   * Schedule an event to be emitted at a future time.
   *
   * @param topic - The topic to emit to
   * @param data - The event payload
   * @param options - Schedule options (scheduledFor or in)
   * @returns The created schedule
   *
   * @example
   * // Schedule for specific time
   * await client.schedule('orders.reminder', { orderId: '123' }, {
   *   scheduledFor: new Date('2024-01-15T10:00:00Z')
   * })
   *
   * // Schedule with relative delay
   * await client.schedule('orders.reminder', { orderId: '123' }, { in: '30m' })
   */
  async schedule(
    topic: string,
    data: Record<string, unknown>,
    options?: ScheduleOptions
  ): Promise<CreateScheduleResponse> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const body: Record<string, unknown> = { topic, data }
      if (options?.scheduledFor) {
        body.scheduled_for = options.scheduledFor.toISOString()
      }
      if (options?.in) {
        body.in = options.in
      }

      const response = await fetch(`${this.server}/api/v1/schedules`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.apiKey}`,
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'schedule failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'schedule failed')
      }

      return await response.json() as CreateScheduleResponse
    } catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof NotifError) throw error
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }
      throw new ConnectionError('network error', error as Error)
    }
  }

  /**
   * List scheduled events.
   *
   * @param options - Filter and pagination options
   * @returns List of schedules with total count
   */
  async listSchedules(options?: ListSchedulesOptions): Promise<ListSchedulesResponse> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const params = new URLSearchParams()
      if (options?.status) params.set('status', options.status)
      if (options?.limit !== undefined) params.set('limit', options.limit.toString())
      if (options?.offset !== undefined) params.set('offset', options.offset.toString())

      const url = `${this.server}/api/v1/schedules${params.toString() ? '?' + params.toString() : ''}`
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
        },
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'list schedules failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'list schedules failed')
      }

      return await response.json() as ListSchedulesResponse
    } catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof NotifError) throw error
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }
      throw new ConnectionError('network error', error as Error)
    }
  }

  /**
   * Get a specific scheduled event.
   *
   * @param id - The schedule ID
   * @returns The schedule details
   */
  async getSchedule(id: string): Promise<Schedule> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(`${this.server}/api/v1/schedules/${id}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
        },
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'get schedule failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'get schedule failed')
      }

      return await response.json() as Schedule
    } catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof NotifError) throw error
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }
      throw new ConnectionError('network error', error as Error)
    }
  }

  /**
   * Cancel a pending scheduled event.
   *
   * @param id - The schedule ID to cancel
   */
  async cancelSchedule(id: string): Promise<void> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(`${this.server}/api/v1/schedules/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
        },
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'cancel schedule failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'cancel schedule failed')
      }
    } catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof NotifError) throw error
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }
      throw new ConnectionError('network error', error as Error)
    }
  }

  /**
   * Execute a scheduled event immediately.
   *
   * @param id - The schedule ID to run
   * @returns The schedule ID and emitted event ID
   */
  async runSchedule(id: string): Promise<RunScheduleResponse> {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), this.timeout)

    try {
      const response = await fetch(`${this.server}/api/v1/schedules/${id}/run`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
        },
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status === 401) {
        throw new AuthError()
      }

      if (!response.ok) {
        const body = await response.json().catch(() => ({ error: 'run schedule failed' })) as { error?: string }
        throw new APIError(response.status, body.error ?? 'run schedule failed')
      }

      return await response.json() as RunScheduleResponse
    } catch (error) {
      clearTimeout(timeoutId)
      if (error instanceof NotifError) throw error
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ConnectionError('request timeout')
      }
      throw new ConnectionError('network error', error as Error)
    }
  }

  get serverUrl(): string {
    return this.server
  }
}
