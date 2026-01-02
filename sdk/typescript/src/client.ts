import { DEFAULT_SERVER, DEFAULT_TIMEOUT, ENV_VAR_NAME, API_KEY_PREFIX } from './constants.js'
import { AuthError, APIError, ConnectionError, NotifError } from './errors.js'
import { NotifOptions, SubscribeOptions, EmitResponse } from './types.js'
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

  get serverUrl(): string {
    return this.server
  }
}
