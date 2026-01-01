import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { Button, Input, Badge } from '../../components/ui'
import { useApi } from '../../lib/api'
import type { Webhook, UpdateWebhookRequest } from '../../lib/types'

export const Route = createFileRoute('/webhooks/$id')({
  component: EditWebhookPage,
})

function EditWebhookPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()

  const { data: webhook, isLoading, error } = useQuery({
    queryKey: ['webhooks', id],
    queryFn: () => api<Webhook>(`/api/v1/webhooks/${id}`),
  })

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
    </div>
  )
}
