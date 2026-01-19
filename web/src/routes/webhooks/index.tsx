import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { Button, Badge } from '../../components/ui'
import { useApi, useProjectReady } from '../../lib/api'
import type { Webhook } from '../../lib/types'

export const Route = createFileRoute('/webhooks/')({
  component: WebhooksPage,
})

function WebhooksPage() {
  const api = useApi()
  const projectReady = useProjectReady()

  const { data: webhooksResponse, isLoading, error } = useQuery({
    queryKey: ['webhooks'],
    queryFn: () => api<{ webhooks: Webhook[]; count: number }>('/api/v1/webhooks'),
    enabled: projectReady,
  })
  const webhooks = webhooksResponse?.webhooks

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold text-neutral-900">Webhooks</h1>
        <Link to="/webhooks/new">
          <Button size="sm">
            <Plus className="w-4 h-4 mr-1.5" />
            New Webhook
          </Button>
        </Link>
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="p-8 text-center text-neutral-500">Loading webhooks...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-8 text-center text-error">
          Failed to load webhooks: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && webhooks?.length === 0 && (
        <div className="p-8 text-center text-neutral-500">
          No webhooks configured. Create your first webhook to start receiving events.
        </div>
      )}

      {/* Table */}
      {webhooks && webhooks.length > 0 && (
        <div className="border border-neutral-200 bg-white">
          <table className="w-full">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">URL</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Topics</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {webhooks.map((webhook) => (
                <tr key={webhook.id} className="hover:bg-neutral-50">
                  <td className="px-4 py-3">
                    <Link
                      to="/webhooks/$id"
                      params={{ id: webhook.id }}
                      className="text-sm font-mono text-neutral-700 hover:text-primary-600"
                    >
                      {webhook.url}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1 flex-wrap">
                      {webhook.topics.map((topic) => (
                        <Badge key={topic} variant="default">{topic}</Badge>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={webhook.enabled ? 'success' : 'default'}>
                      {webhook.enabled ? 'active' : 'inactive'}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
