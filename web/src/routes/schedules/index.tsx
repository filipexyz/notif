import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Play, X, Eye } from 'lucide-react'
import { Button, Badge } from '../../components/ui'
import { useApi } from '../../lib/api'
import type { Schedule, SchedulesResponse, RunScheduleResponse } from '../../lib/types'

export const Route = createFileRoute('/schedules/')({
  component: SchedulesPage,
})

const STATUS_FILTERS = ['all', 'pending', 'completed', 'cancelled', 'failed'] as const
type StatusFilter = typeof STATUS_FILTERS[number]

function formatDateTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

function StatusBadge({ status }: { status: Schedule['status'] }) {
  const variants: Record<Schedule['status'], 'info' | 'success' | 'default' | 'error'> = {
    pending: 'info',
    completed: 'success',
    cancelled: 'default',
    failed: 'error',
  }
  return <Badge variant={variants[status]}>{status}</Badge>
}

function SchedulesPage() {
  const api = useApi()
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')

  const { data: schedulesResponse, isLoading, error } = useQuery({
    queryKey: ['schedules', statusFilter],
    queryFn: () => {
      const params = new URLSearchParams()
      if (statusFilter !== 'all') params.set('status', statusFilter)
      params.set('limit', '50')
      const query = params.toString()
      return api<SchedulesResponse>(`/api/v1/schedules${query ? `?${query}` : ''}`)
    },
  })
  const schedules = schedulesResponse?.schedules ?? []

  const cancelMutation = useMutation({
    mutationFn: (id: string) =>
      api(`/api/v1/schedules/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })

  const runMutation = useMutation({
    mutationFn: (id: string) =>
      api<RunScheduleResponse>(`/api/v1/schedules/${id}/run`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })

  const handleCancel = (id: string) => {
    if (confirm('Are you sure you want to cancel this scheduled event?')) {
      cancelMutation.mutate(id)
    }
  }

  const handleRun = (id: string) => {
    runMutation.mutate(id)
  }

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold text-neutral-900">Schedules</h1>
      </div>

      {/* Status filter tabs */}
      <div className="flex gap-1 mb-4">
        {STATUS_FILTERS.map((status) => (
          <button
            key={status}
            onClick={() => setStatusFilter(status)}
            className={`px-3 py-1.5 text-sm font-medium transition-colors ${
              statusFilter === status
                ? 'text-primary-600 bg-primary-50'
                : 'text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50'
            }`}
          >
            {status.charAt(0).toUpperCase() + status.slice(1)}
          </button>
        ))}
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="py-12 text-center text-neutral-500">Loading schedules...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="py-12 text-center text-error">
          Failed to load schedules: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && schedules.length === 0 && (
        <div className="py-12 text-center text-neutral-500">
          No scheduled events
        </div>
      )}

      {/* Table */}
      {schedules.length > 0 && (
        <div className="border border-neutral-200 bg-white">
          <table className="w-full">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">ID</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Topic</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Scheduled For</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Status</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Created</th>
                <th className="px-4 py-2 text-right text-sm font-medium text-neutral-700">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {schedules.map((schedule) => (
                <tr key={schedule.id} className="hover:bg-neutral-50">
                  <td className="px-4 py-3">
                    <Link
                      to="/schedules/$id"
                      params={{ id: schedule.id }}
                      className="text-sm font-mono text-neutral-700 hover:text-primary-600"
                    >
                      {schedule.id.slice(0, 12)}...
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm font-medium text-primary-600">{schedule.topic}</span>
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-600">
                    {formatDateTime(schedule.scheduled_for)}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={schedule.status} />
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-500">
                    {formatDateTime(schedule.created_at)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex justify-end gap-1">
                      <Link
                        to="/schedules/$id"
                        params={{ id: schedule.id }}
                        className="p-1.5 text-neutral-400 hover:text-primary-600 hover:bg-neutral-100"
                        title="View details"
                      >
                        <Eye className="w-4 h-4" />
                      </Link>
                      {schedule.status === 'pending' && (
                        <>
                          <button
                            onClick={() => handleRun(schedule.id)}
                            disabled={runMutation.isPending}
                            className="p-1.5 text-neutral-400 hover:text-success hover:bg-neutral-100 disabled:opacity-50"
                            title="Run now"
                          >
                            <Play className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleCancel(schedule.id)}
                            disabled={cancelMutation.isPending}
                            className="p-1.5 text-neutral-400 hover:text-error hover:bg-neutral-100 disabled:opacity-50"
                            title="Cancel"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </>
                      )}
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
