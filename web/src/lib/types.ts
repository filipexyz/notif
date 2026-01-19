// Project
export type Project = {
  id: string
  org_id: string
  name: string
  slug: string
  created_at: string
  updated_at: string
}

export type CreateProjectRequest = {
  name: string
  slug?: string
}

export type UpdateProjectRequest = {
  name?: string
  slug?: string
}

export type ProjectsResponse = {
  projects: Project[]
  count: number
}

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
  project_id: string
  project_name?: string
  created_at: string
  last_used_at?: string
}

export type CreateAPIKeyRequest = {
  name: string
  project_id: string
}

export type CreateAPIKeyResponse = {
  id: string
  name?: string
  full_key: string // Full key, only returned on creation
  key_prefix: string
  project_id: string
  created_at: string
}

// Stats
export type Stats = {
  events_24h: number
  webhooks_active: number
  dlq_count: number
}

// Webhook Delivery (legacy, for webhook-specific views)
export type WebhookDelivery = {
  id: string
  webhook_id: string
  webhook_url?: string
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

// Unified Event Delivery (all receiver types)
export type EventDelivery = {
  id: string
  event_id: string
  receiver_type: 'webhook' | 'websocket'
  receiver_id?: string       // For webhooks
  consumer_name?: string     // For websocket
  client_id?: string         // For websocket
  webhook_url?: string       // Populated for webhooks
  status: 'pending' | 'delivered' | 'acked' | 'nacked' | 'dlq'
  attempt: number
  created_at: string
  delivered_at?: string
  acked_at?: string
  error?: string
}

// Schedule
export type Schedule = {
  id: string
  topic: string
  data: Record<string, unknown>
  scheduled_for: string
  status: 'pending' | 'completed' | 'cancelled' | 'failed'
  error?: string
  created_at: string
  executed_at?: string
}

export type SchedulesResponse = {
  schedules: Schedule[]
  total: number
}

export type RunScheduleResponse = {
  schedule_id: string
  event_id: string
}
