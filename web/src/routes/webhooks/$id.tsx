import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, RotateCcw, Copy, CheckCircle, XCircle, Clock } from 'lucide-react'
import { Button, Input, Badge } from '../../components/ui'
import { useApi, useProjectReady } from '../../lib/api'
import type { Webhook, UpdateWebhookRequest, WebhookDelivery } from '../../lib/types'

export const Route = createFileRoute('/webhooks/$id')({
  component: EditWebhookPage,
})

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function StatusIcon({ status }: { status: string }) {
  if (status === 'success') return <CheckCircle className="w-4 h-4 text-success" />
  if (status === 'failed') return <XCircle className="w-4 h-4 text-error" />
  return <Clock className="w-4 h-4 text-neutral-400" />
}

function EditWebhookPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()
  const projectReady = useProjectReady()

  const { data: webhook, isLoading, error } = useQuery({
    queryKey: ['webhooks', id],
    queryFn: () => api<Webhook>(`/api/v1/webhooks/${id}`),
    enabled: projectReady,
  })

  const { data: deliveriesResponse } = useQuery({
    queryKey: ['webhooks', id, 'deliveries'],
    queryFn: () => api<{ deliveries: WebhookDelivery[]; count: number }>(`/api/v1/webhooks/${id}/deliveries`),
    enabled: !!webhook,
  })
  const deliveries = deliveriesResponse?.deliveries ?? []

  const [url, setUrl] = useState('')
  const [topics, setTopics] = useState('')
  const [enabled, setEnabled] = useState(true)

  // Initialize form when webhook data loads
  useEffect(() => {
    if (webhook) {
      setUrl(webhook.url)
      setTopics(webhook.topics.join(', '))
      setEnabled(webhook.enabled)
    }
  }, [webhook])

  const updateWebhook = useMutation({
    mutationFn: (data: UpdateWebhookRequest) =>
      api<Webhook>(`/api/v1/webhooks/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      navigate({ to: '/webhooks' })
    },
  })

  const deleteWebhook = useMutation({
    mutationFn: () =>
      api(`/api/v1/webhooks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      navigate({ to: '/webhooks' })
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    updateWebhook.mutate({
      url,
      topics: topics.split(',').map(t => t.trim()),
      enabled,
    })
  }

  const handleDelete = () => {
    if (confirm('Are you sure you want to delete this webhook?')) {
      deleteWebhook.mutate()
    }
  }

  if (isLoading) {
    return (
      <div className="p-8 text-center text-neutral-500">Loading webhook...</div>
    )
  }

  if (error) {
    return (
      <div className="p-8 text-center text-error">
        Failed to load webhook: {error.message}
      </div>
    )
  }

  return (
    <div className="p-4 max-w-xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-neutral-900">Edit Webhook</h1>
        <Badge variant={enabled ? 'success' : 'default'}>
          {enabled ? 'active' : 'inactive'}
        </Badge>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label="URL"
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />

        <Input
          label="Topics"
          value={topics}
          onChange={(e) => setTopics(e.target.value)}
        />
        <p className="text-xs text-neutral-500 -mt-2">
          Comma-separated topic patterns. Use * for wildcard.
        </p>

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="enabled"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="w-4 h-4"
          />
          <label htmlFor="enabled" className="text-sm text-neutral-700">
            Enabled
          </label>
        </div>

        {(updateWebhook.error || deleteWebhook.error) && (
          <div className="text-sm text-error">
            {updateWebhook.error?.message || deleteWebhook.error?.message}
          </div>
        )}

        <div className="flex justify-between pt-4">
          <div className="flex gap-2">
            <Button type="submit" disabled={updateWebhook.isPending}>
              {updateWebhook.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => navigate({ to: '/webhooks' })}
            >
              Cancel
            </Button>
          </div>
          <Button
            type="button"
            variant="danger"
            onClick={handleDelete}
            disabled={deleteWebhook.isPending}
          >
            <Trash2 className="w-4 h-4 mr-1.5" />
            {deleteWebhook.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </div>
      </form>

      {/* Deliveries */}
      <div className="mt-8">
        <h2 className="text-lg font-medium text-neutral-900 mb-4">Recent Deliveries</h2>

        {deliveries.length === 0 ? (
          <div className="py-8 text-center text-neutral-500 border border-neutral-200 bg-white">
            No deliveries yet
          </div>
        ) : (
          <div className="border border-neutral-200 bg-white divide-y divide-neutral-100">
            {deliveries.map((delivery) => (
              <div key={delivery.id} className="px-4 py-3 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <StatusIcon status={delivery.status} />
                  <div>
                    <div className="text-sm font-medium text-neutral-700">
                      {delivery.topic}
                    </div>
                    <div className="text-xs text-neutral-500">
                      {formatTime(delivery.created_at)}
                      {delivery.response_status && (
                        <span className="ml-2">HTTP {delivery.response_status}</span>
                      )}
                      {delivery.error && (
                        <span className="ml-2 text-error">{delivery.error}</span>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => navigator.clipboard.writeText(delivery.event_id)}
                    className="p-1.5 text-neutral-400 hover:text-neutral-600 hover:bg-neutral-100"
                    title="Copy event ID"
                  >
                    <Copy className="w-4 h-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
