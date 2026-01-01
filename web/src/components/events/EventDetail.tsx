import { Copy, RotateCcw, Check, X } from 'lucide-react'
import { Button } from '../ui'
import type { UIEvent } from '../../routes/index'

interface EventDetailProps {
  event: UIEvent
}

// Mock delivery data
const mockDeliveries = [
  { id: 'del_1', webhook: 'api.acme.com/hook', status: 'success', latency: 45 },
  { id: 'del_2', webhook: 'slack.com/webhook', status: 'failed', error: 'timeout' },
]

export function EventDetail({ event }: EventDetailProps) {
  const handleCopy = () => {
    navigator.clipboard.writeText(JSON.stringify(event.data, null, 2))
  }

  return (
    <div className="space-y-6">
      {/* Metadata */}
      <div className="space-y-2">
        <div className="flex justify-between text-sm">
          <span className="text-neutral-500">ID</span>
          <span className="font-mono text-neutral-700">{event.id}</span>
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
        <div className="space-y-1">
          {mockDeliveries.map((delivery) => (
            <div
              key={delivery.id}
              className="flex items-center justify-between py-1.5 px-2 bg-neutral-50 border border-neutral-200"
            >
              <div className="flex items-center gap-2">
                {delivery.status === 'success' ? (
                  <Check className="w-3.5 h-3.5 text-success" />
                ) : (
                  <X className="w-3.5 h-3.5 text-error" />
                )}
                <span className="text-sm font-mono text-neutral-600">
                  {delivery.webhook}
                </span>
              </div>
              <span className="text-xs text-neutral-500">
                {delivery.status === 'success'
                  ? `${delivery.latency}ms`
                  : delivery.error}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-2">
        <Button variant="secondary" size="sm">
          <RotateCcw className="w-3.5 h-3.5 mr-1.5" />
          Replay
        </Button>
        <Button variant="secondary" size="sm" onClick={handleCopy}>
          <Copy className="w-3.5 h-3.5 mr-1.5" />
          Copy JSON
        </Button>
      </div>
    </div>
  )
}
