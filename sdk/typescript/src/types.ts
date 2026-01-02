export interface NotifOptions {
  apiKey?: string
  server?: string
  timeout?: number
}

export interface SubscribeOptions {
  autoAck?: boolean
  from?: 'latest' | 'beginning' | string
  group?: string | undefined
}

export interface ResolvedSubscribeOptions {
  autoAck: boolean
  from: string
  group: string | undefined
}

export interface EmitResponse {
  id: string
  topic: string
  created_at: string
}

export interface Event {
  id: string
  topic: string
  data: Record<string, unknown>
  timestamp: string
  attempt: number
  maxAttempts: number
  ack: () => Promise<void>
  nack: (retryIn?: string) => Promise<void>
}

// Internal types for WebSocket protocol

export interface SubscribeMessage {
  action: 'subscribe'
  topics: string[]
  options: {
    auto_ack: boolean
    from: string
    group?: string
  }
}

export interface AckMessage {
  action: 'ack'
  id: string
}

export interface NackMessage {
  action: 'nack'
  id: string
  retry_in: string
}

export interface PingMessage {
  action: 'ping'
}

export type ClientMessage = SubscribeMessage | AckMessage | NackMessage | PingMessage

export interface SubscribedMessage {
  type: 'subscribed'
  topics: string[]
  consumer_id: string
}

export interface EventMessage {
  type: 'event'
  id: string
  topic: string
  data: Record<string, unknown>
  timestamp: string
  attempt: number
  max_attempts: number
}

export interface ErrorMessage {
  type: 'error'
  code: string
  message: string
}

export interface PongMessage {
  type: 'pong'
}

export type ServerMessage = SubscribedMessage | EventMessage | ErrorMessage | PongMessage
