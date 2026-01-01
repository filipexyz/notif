import { useQuery } from '@tanstack/react-query'
import { Copy, Check, X, Clock, Globe, Wifi } from 'lucide-react'
import { useApi } from '../../lib/api'
import type { UIEvent } from '../../routes/index'
import type { EventDelivery } from '../../lib/types'

interface EventDetailProps {
  event: UIEvent
}

export function EventDetail({ event }: EventDetailProps) {
  const api = useApi()

  const { data: deliveriesResponse } = useQuery({
    queryKey: ['events', event.id, 'deliveries'],
    queryFn: () => api<{ deliveries: EventDelivery[]; count: number }>(`/api/v1/events/${event.id}/deliveries`),
  })
  const deliveries = deliveriesResponse?.deliveries ?? []

  const getStatusIcon = (status: EventDelivery['status']) => {
    switch (status) {
      case 'acked':
        return <Check className="w-3.5 h-3.5 text-success" />
      case 'nacked':
      case 'dlq':
        return <X className="w-3.5 h-3.5 text-error" />
      default:
        return <Clock className="w-3.5 h-3.5 text-neutral-400" />
    }
  }

  const getReceiverIcon = (type: EventDelivery['receiver_type']) => {
    return type === 'webhook'
      ? <Globe className="w-3 h-3" />
      : <Wifi className="w-3 h-3" />
  }

  const getReceiverName = (delivery: EventDelivery) => {
    if (delivery.receiver_type === 'webhook') {
      return delivery.webhook_url?.replace(/^https?:\/\//, '') ?? 'Unknown'
    }
    return delivery.consumer_name ?? delivery.client_id ?? 'Unknown'
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(JSON.stringify(event.data, null, 2))
  }

  const handleCopyEventId = () => {
    navigator.clipboard.writeText(event.id)
  }

  return (
    <div className="space-y-6">
      {/* Metadata */}
      <div className="space-y-2">
        <div className="flex justify-between text-sm">
          <span className="text-neutral-500">ID</span>
          <button
            onClick={handleCopyEventId}
            className="font-mono text-neutral-700 hover:text-primary-600"
            title="Copy ID"
          >
            {event.id}
          </button>
        </div>
        <div className="flex justify-between text-sm">
          <span className="text-neutral-500">Topic</span>
          <span className="font-medium text-primary-600">{event.topic}</span>
        </div>
        <div className="flex justify-between text-sm">
          <span className="text-neutral-500">Time</span>
          <span className="font-mono text-neutral-700">
            {event.timestamp.toISOString()}
          </span>
        </div>
      </div>

      {/* Payload */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-sm font-medium text-neutral-700">Payload</h3>
          <button
            onClick={handleCopy}
            className="p-1 text-neutral-400 hover:text-neutral-600 hover:bg-neutral-100"
            title="Copy JSON"
          >
            <Copy className="w-3.5 h-3.5" />
          </button>
        </div>
        <pre className="p-3 bg-neutral-50 border border-neutral-200 text-sm font-mono text-neutral-700 overflow-x-auto">
          {JSON.stringify(event.data, null, 2)}
        </pre>
      </div>

      {/* Deliveries */}
      <div>
        <h3 className="text-sm font-medium text-neutral-700 mb-2">Deliveries</h3>
        {deliveries.length === 0 ? (
          <div className="py-4 text-center text-sm text-neutral-500 border border-neutral-200 bg-neutral-50">
            No deliveries yet
          </div>
        ) : (
          <div className="space-y-1">
            {deliveries.map((delivery) => (
              <div
                key={delivery.id}
                className="flex items-center justify-between py-1.5 px-2 bg-neutral-50 border border-neutral-200"
              >
                <div className="flex items-center gap-2">
                  {getStatusIcon(delivery.status)}
                  <span className={`inline-flex items-center gap-1 text-xs px-1.5 py-0.5 ${
                    delivery.receiver_type === 'webhook'
                      ? 'bg-blue-100 text-blue-700'
                      : 'bg-purple-100 text-purple-700'
                  }`}>
                    {getReceiverIcon(delivery.receiver_type)}
                    {delivery.receiver_type}
                  </span>
                  <span className="text-sm font-mono text-neutral-600 truncate max-w-[180px]">
                    {getReceiverName(delivery)}
                  </span>
                </div>
                <span className="text-xs text-neutral-500">
                  {delivery.error ?? delivery.status}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
