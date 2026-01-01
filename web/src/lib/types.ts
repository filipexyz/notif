// Event from backend (nested structure)
export type StoredEvent = {
  seq: number
  event: {
    id: string
    topic: string
    data: Record<string, unknown>
    timestamp: string
  }
  timestamp: string
}

// Webhook
export type Webhook = {
  id: string
  url: string
  topics: string[]
  secret?: string // only returned on create
  enabled: boolean
  created_at: string
}

export type CreateWebhookRequest = {
  url: string
  topics: string[]
}

export type UpdateWebhookRequest = Partial<CreateWebhookRequest> & {
  enabled?: boolean
}

// DLQ Entry
export type DLQEntry = {
  seq: number
  original_seq: number
  topic: string
  event_type: string
  payload: Record<string, unknown>
  error: string
  attempts: number
  created_at: string
}

// API Key
export type APIKey = {
  id: string
  name?: string
  key_prefix: string
  created_at: string
  last_used_at?: string
}

export type CreateAPIKeyRequest = {
  name: string
}

export type CreateAPIKeyResponse = {
  id: string
  name?: string
  full_key: string // Full key, only returned on creation
  key_prefix: string
  created_at: string
}

// Stats
export type Stats = {
  events_24h: number
  webhooks_active: number
  dlq_count: number
}

// Webhook Delivery
export type WebhookDelivery = {
  id: string
  webhook_id: string
  event_id: string
  topic: string
  status: 'pending' | 'success' | 'failed'
  attempt: number
  response_status?: number
  response_body?: string
  error?: string
  created_at: string
  delivered_at?: string
}
