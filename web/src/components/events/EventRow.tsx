import { ChevronRight } from 'lucide-react'
import type { UIEvent } from '../../routes/index'

interface EventRowProps {
  event: UIEvent
  selected?: boolean
  onClick?: () => void
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function truncateJson(data: Record<string, unknown>, maxLength = 60): string {
  const json = JSON.stringify(data)
  if (json.length <= maxLength) return json
  return json.slice(0, maxLength) + '...'
}

export function EventRow({ event, selected, onClick }: EventRowProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full px-4 py-2 flex items-center gap-4 text-left hover:bg-neutral-50 transition-colors ${
        selected ? 'bg-primary-50' : 'bg-white'
      }`}
    >
      {/* Timestamp */}
      <span className="text-sm font-mono text-neutral-500 shrink-0">
        {formatTime(event.timestamp)}
      </span>

      {/* Topic */}
      <span className="text-sm font-medium text-primary-600 shrink-0 min-w-[140px]">
        {event.topic}
      </span>

      {/* Payload preview */}
      <span className="text-sm font-mono text-neutral-600 truncate flex-1">
        {truncateJson(event.data)}
      </span>

      {/* Arrow */}
      <ChevronRight className="w-4 h-4 text-neutral-300 shrink-0" />
    </button>
  )
}
