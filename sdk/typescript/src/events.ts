import WebSocket from 'ws'
import { SubscribeOptions, ResolvedSubscribeOptions, Event, ServerMessage, EventMessage } from './types.js'
import { ConnectionError, APIError } from './errors.js'

const MAX_RECONNECT_ATTEMPTS = 5
const INITIAL_RECONNECT_DELAY = 1000

export class EventStream implements AsyncIterable<Event> {
  private ws: WebSocket | null = null
  private readonly wsUrl: string
  private readonly topics: string[]
  private readonly options: ResolvedSubscribeOptions
  private readonly apiKey: string

  private eventQueue: Event[] = []
  private eventResolvers: Array<(value: IteratorResult<Event>) => void> = []
  private errorResolvers: Array<(error: Error) => void> = []

  private closed = false
  private connected = false
  private connectStarted = false
  private reconnectAttempts = 0
  private closeCallbacks: Array<() => void> = []

  constructor(
    apiKey: string,
    server: string,
    topics: string[],
    options: SubscribeOptions = {}
  ) {
    this.apiKey = apiKey
    this.wsUrl = server.replace('http://', 'ws://').replace('https://', 'wss://')
    this.topics = topics
    this.options = {
      autoAck: options.autoAck ?? true,
      from: options.from ?? 'latest',
      group: options.group,
    }
    // Connection deferred to first iteration
  }

  private connect(): void {
    if (this.closed) return

    try {
      this.ws = new WebSocket(`${this.wsUrl}/ws`, {
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
        },
      })

      this.ws.on('open', () => {
        this.connected = true
        this.reconnectAttempts = 0
        this.sendSubscribe()
      })

      this.ws.on('message', (data: WebSocket.Data) => {
        try {
          const msg = JSON.parse(data.toString()) as ServerMessage
          this.handleMessage(msg)
        } catch {
          // Ignore parse errors
        }
      })

      this.ws.on('error', () => {
        this.rejectPending(new ConnectionError('WebSocket error'))
      })

      this.ws.on('close', () => {
        this.connected = false
        if (!this.closed) {
          this.attemptReconnect()
        }
      })
    } catch (error) {
      this.rejectPending(new ConnectionError('Failed to connect', error as Error))
    }
  }

  private sendSubscribe(): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return

    this.ws.send(JSON.stringify({
      action: 'subscribe',
      topics: this.topics,
      options: {
        auto_ack: this.options.autoAck,
        from: this.options.from,
        group: this.options.group,
      },
    }))
  }

  private handleMessage(msg: ServerMessage): void {
    switch (msg.type) {
      case 'event':
        this.handleEvent(msg)
        break
      case 'error':
        this.rejectPending(new APIError(0, msg.message))
        break
      case 'subscribed':
      case 'pong':
        // Ignore these
        break
    }
  }

  private handleEvent(msg: EventMessage): void {
    const event = this.createEvent(msg)

    // If there's a waiting resolver, resolve immediately
    const resolver = this.eventResolvers.shift()
    if (resolver) {
      resolver({ value: event, done: false })
    } else {
      // Otherwise queue the event
      this.eventQueue.push(event)
    }
  }

  private createEvent(msg: EventMessage): Event {
    const autoAck = this.options.autoAck
    return {
      id: msg.id,
      topic: msg.topic,
      data: msg.data,
      timestamp: msg.timestamp,
      attempt: msg.attempt,
      maxAttempts: msg.max_attempts,
      ack: async () => {
        if (autoAck) return // Server already acked
        this.sendAck(msg.id)
      },
      nack: async (retryIn?: string) => {
        if (autoAck) return // Server already acked, can't nack
        this.sendNack(msg.id, retryIn)
      },
    }
  }

  private sendAck(eventId: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ action: 'ack', id: eventId }))
  }

  private sendNack(eventId: string, retryIn?: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({
      action: 'nack',
      id: eventId,
      retry_in: retryIn ?? '5m',
    }))
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      this.rejectPending(new ConnectionError('max reconnection attempts reached'))
      this.close()
      return
    }

    const delay = INITIAL_RECONNECT_DELAY * Math.pow(2, this.reconnectAttempts)
    this.reconnectAttempts++

    setTimeout(() => {
      if (!this.closed) {
        this.connect()
      }
    }, delay)
  }

  private rejectPending(error: Error): void {
    const rejector = this.errorResolvers.shift()
    if (rejector) {
      rejector(error)
    }
  }

  async *[Symbol.asyncIterator](): AsyncIterator<Event> {
    // Connect on first iteration (deferred connection)
    if (!this.connectStarted) {
      this.connectStarted = true
      this.connect()
    }

    while (!this.closed) {
      // If there are queued events, yield them
      if (this.eventQueue.length > 0) {
        const event = this.eventQueue.shift()!
        yield event
        continue
      }

      // Wait for next event or error
      try {
        const result = await new Promise<IteratorResult<Event>>((resolve, reject) => {
          this.eventResolvers.push(resolve)
          this.errorResolvers.push(reject)
        })

        if (result.done) {
          return
        }

        yield result.value
      } catch (error) {
        // On error, throw to let the caller handle it
        throw error
      }
    }
  }

  onClose(callback: () => void): void {
    this.closeCallbacks.push(callback)
  }

  close(): void {
    if (this.closed) return

    this.closed = true

    // Resolve any pending promises with done
    for (const resolver of this.eventResolvers) {
      resolver({ value: undefined as unknown as Event, done: true })
    }
    this.eventResolvers = []
    this.errorResolvers = []

    // Close WebSocket
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }

    // Call close callbacks
    for (const callback of this.closeCallbacks) {
      callback()
    }
  }

  get isConnected(): boolean {
    return this.connected
  }
}
