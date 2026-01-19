import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { RotateCcw, Trash2 } from 'lucide-react'
import { Button, Badge } from '../components/ui'
import { useApi, useProjectReady } from '../lib/api'
import type { DLQEntry } from '../lib/types'

export const Route = createFileRoute('/dlq')({
  component: DLQPage,
})

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function DLQPage() {
  const api = useApi()
  const queryClient = useQueryClient()
  const projectReady = useProjectReady()

  const { data: dlqResponse, isLoading, error } = useQuery({
    queryKey: ['dlq'],
    queryFn: () => api<{ messages: DLQEntry[]; count: number }>('/api/v1/dlq'),
    enabled: projectReady,
  })
  const dlqEntries = dlqResponse?.messages

  const replayMutation = useMutation({
    mutationFn: (seq: number) =>
      api(`/api/v1/dlq/${seq}/replay`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dlq'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (seq: number) =>
      api(`/api/v1/dlq/${seq}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dlq'] })
    },
  })

  const replayAllMutation = useMutation({
    mutationFn: () =>
      api('/api/v1/dlq/replay-all', { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dlq'] })
    },
  })

  const purgeMutation = useMutation({
    mutationFn: () =>
      api('/api/v1/dlq', { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dlq'] })
    },
  })

  const handleReplay = (seq: number) => {
    replayMutation.mutate(seq)
  }

  const handleDelete = (seq: number) => {
    deleteMutation.mutate(seq)
  }

  const handleReplayAll = () => {
    replayAllMutation.mutate()
  }

  const handlePurge = () => {
    if (confirm('Are you sure you want to purge all DLQ messages?')) {
      purgeMutation.mutate()
    }
  }

  const entries = dlqEntries ?? []

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold text-neutral-900">Dead Letter Queue</h1>
          {entries.length > 0 && (
            <Badge variant="error">{entries.length}</Badge>
          )}
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            onClick={handleReplayAll}
            disabled={entries.length === 0 || replayAllMutation.isPending}
          >
            <RotateCcw className="w-4 h-4 mr-1.5" />
            {replayAllMutation.isPending ? 'Replaying...' : 'Replay All'}
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={handlePurge}
            disabled={entries.length === 0 || purgeMutation.isPending}
          >
            <Trash2 className="w-4 h-4 mr-1.5" />
            {purgeMutation.isPending ? 'Purging...' : 'Purge'}
          </Button>
        </div>
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="py-12 text-center text-neutral-500">Loading DLQ...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="py-12 text-center text-error">
          Failed to load DLQ: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && entries.length === 0 && (
        <div className="py-12 text-center text-neutral-500">
          No failed events
        </div>
      )}

      {/* Table */}
      {entries.length > 0 && (
        <div className="border border-neutral-200 bg-white">
          <table className="w-full">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Time</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Topic</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Error</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Attempts</th>
                <th className="px-4 py-2 text-right text-sm font-medium text-neutral-700">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {entries.map((entry) => (
                <tr key={entry.seq} className="hover:bg-neutral-50">
                  <td className="px-4 py-3 text-sm font-mono text-neutral-500">
                    {formatTime(entry.created_at)}
                  </td>
                  <td className="px-4 py-3 text-sm font-medium text-primary-600">
                    {entry.topic}
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-600">
                    {entry.error}
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-500">
                    {entry.attempts}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-1">
                      <button
                        onClick={() => handleReplay(entry.seq)}
                        disabled={replayMutation.isPending}
                        className="p-1.5 text-neutral-400 hover:text-primary-600 hover:bg-neutral-100 disabled:opacity-50"
                        title="Replay"
                      >
                        <RotateCcw className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(entry.seq)}
                        disabled={deleteMutation.isPending}
                        className="p-1.5 text-neutral-400 hover:text-error hover:bg-neutral-100 disabled:opacity-50"
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
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
