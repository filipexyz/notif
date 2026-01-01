import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button, Input } from '../../components/ui'
import { useApi } from '../../lib/api'
import type { Webhook, CreateWebhookRequest } from '../../lib/types'

export const Route = createFileRoute('/webhooks/new')({
  component: NewWebhookPage,
})

function NewWebhookPage() {
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()

  const [url, setUrl] = useState('')
  const [topics, setTopics] = useState('')

  const createWebhook = useMutation({
    mutationFn: (data: CreateWebhookRequest) =>
      api<Webhook>('/api/v1/webhooks', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      navigate({ to: '/webhooks' })
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createWebhook.mutate({
      url,
      topics: topics ? topics.split(',').map(t => t.trim()) : ['*'],
    })
  }

  return (
    <div className="p-4 max-w-xl">
      <h1 className="text-xl font-semibold text-neutral-900 mb-6">New Webhook</h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label="URL"
          type="url"
          placeholder="https://api.example.com/webhooks"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />

        <Input
          label="Topics"
          placeholder="orders.*, users.signup"
          value={topics}
          onChange={(e) => setTopics(e.target.value)}
        />
        <p className="text-xs text-neutral-500 -mt-2">
          Comma-separated topic patterns. Use * for wildcard. Leave empty for all topics.
        </p>

        {createWebhook.error && (
          <div className="text-sm text-error">
            {createWebhook.error.message}
          </div>
        )}

        <div className="flex gap-2 pt-4">
          <Button type="submit" disabled={createWebhook.isPending}>
            {createWebhook.isPending ? 'Creating...' : 'Create Webhook'}
          </Button>
          <Button
            type="button"
            variant="secondary"
            onClick={() => navigate({ to: '/webhooks' })}
          >
            Cancel
          </Button>
        </div>
      </form>
    </div>
  )
}
