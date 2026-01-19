import { createFileRoute, useNavigate, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Play, X } from 'lucide-react'
import { Button, Badge } from '../../components/ui'
import { useApi, useProjectReady } from '../../lib/api'
import type { Schedule, RunScheduleResponse } from '../../lib/types'

export const Route = createFileRoute('/schedules/$id')({
  component: ScheduleDetailPage,
})

function formatDateTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
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

function ScheduleDetailPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()
  const projectReady = useProjectReady()

  const { data: schedule, isLoading, error } = useQuery({
    queryKey: ['schedules', id],
    queryFn: () => api<Schedule>(`/api/v1/schedules/${id}`),
    enabled: projectReady,
  })

  const cancelMutation = useMutation({
    mutationFn: () =>
      api(`/api/v1/schedules/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
      navigate({ to: '/schedules' })
    },
  })

  const runMutation = useMutation({
    mutationFn: () =>
      api<RunScheduleResponse>(`/api/v1/schedules/${id}/run`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })

  const handleCancel = () => {
    if (confirm('Are you sure you want to cancel this scheduled event?')) {
      cancelMutation.mutate()
    }
  }

  const handleRun = () => {
    runMutation.mutate()
  }

  if (isLoading) {
    return (
      <div className="p-8 text-center text-neutral-500">Loading schedule...</div>
    )
  }

  if (error) {
    return (
      <div className="p-8 text-center text-error">
        Failed to load schedule: {error.message}
      </div>
    )
  }

  if (!schedule) {
    return (
      <div className="p-8 text-center text-neutral-500">Schedule not found</div>
    )
  }

  return (
    <div className="p-4 max-w-2xl">
      {/* Back link */}
      <Link
        to="/schedules"
        className="inline-flex items-center gap-1 text-sm text-neutral-500 hover:text-neutral-700 mb-4"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Schedules
      </Link>

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold text-neutral-900">Schedule Details</h1>
        <StatusBadge status={schedule.status} />
      </div>

      {/* Details */}
      <div className="border border-neutral-200 bg-white divide-y divide-neutral-100">
        <div className="px-4 py-3">
          <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">ID</div>
          <div className="text-sm font-mono text-neutral-700">{schedule.id}</div>
        </div>

        <div className="px-4 py-3">
          <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Topic</div>
          <div className="text-sm font-medium text-primary-600">{schedule.topic}</div>
        </div>

        <div className="px-4 py-3">
          <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Scheduled For</div>
          <div className="text-sm text-neutral-700">{formatDateTime(schedule.scheduled_for)}</div>
        </div>

        <div className="px-4 py-3">
          <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Created At</div>
          <div className="text-sm text-neutral-700">{formatDateTime(schedule.created_at)}</div>
        </div>

        {schedule.executed_at && (
          <div className="px-4 py-3">
            <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Executed At</div>
            <div className="text-sm text-neutral-700">{formatDateTime(schedule.executed_at)}</div>
          </div>
        )}

        {schedule.error && (
          <div className="px-4 py-3">
            <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Error</div>
            <div className="text-sm text-error">{schedule.error}</div>
          </div>
        )}

        <div className="px-4 py-3">
          <div className="text-xs text-neutral-500 uppercase tracking-wide mb-1">Payload</div>
          <pre className="mt-2 p-3 bg-neutral-50 text-sm font-mono text-neutral-700 overflow-x-auto">
            {JSON.stringify(schedule.data, null, 2)}
          </pre>
        </div>
      </div>

      {/* Actions */}
      {schedule.status === 'pending' && (
        <div className="mt-6 flex gap-2">
          <Button
            onClick={handleRun}
            disabled={runMutation.isPending}
          >
            <Play className="w-4 h-4 mr-1.5" />
            {runMutation.isPending ? 'Running...' : 'Run Now'}
          </Button>
          <Button
            variant="danger"
            onClick={handleCancel}
            disabled={cancelMutation.isPending}
          >
            <X className="w-4 h-4 mr-1.5" />
            {cancelMutation.isPending ? 'Cancelling...' : 'Cancel'}
          </Button>
        </div>
      )}

      {/* Mutation errors */}
      {(cancelMutation.error || runMutation.error) && (
        <div className="mt-4 text-sm text-error">
          {cancelMutation.error?.message || runMutation.error?.message}
        </div>
      )}
    </div>
  )
}
