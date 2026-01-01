import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect, useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { EventRow } from '../components/events/EventRow'
import { EventDetail } from '../components/events/EventDetail'
import { LiveIndicator } from '../components/events/LiveIndicator'
import { SlideOver } from '../components/layout/SlideOver'
import { useApi } from '../lib/api'
import { useEventStream } from '../lib/websocket'
import type { StoredEvent } from '../lib/types'

// UI event format used in components
export type UIEvent = {
  id: string
  topic: string
  data: Record<string, unknown>
  timestamp: Date
}

function toUIEvent(stored: StoredEvent): UIEvent {
  return {
    id: stored.event.id,
    topic: stored.event.topic,
    data: stored.event.data || {},
    timestamp: new Date(stored.event.timestamp),
  }
}

export const Route = createFileRoute('/')({
  component: EventsPage,
})

function EventsPage() {
  const [selectedEvent, setSelectedEvent] = useState<UIEvent | null>(null)
  const [liveEvents, setLiveEvents] = useState<UIEvent[]>([])
  const api = useApi()
  const queryClient = useQueryClient()

  const { data: eventsResponse, isLoading, error } = useQuery({
    queryKey: ['events'],
    queryFn: () => api<{ events: StoredEvent[]; count: number }>('/api/v1/events'),
  })
  const events = eventsResponse?.events

  // Handle incoming WebSocket events
  const handleNewEvent = useCallback((event: StoredEvent) => {
    const uiEvent = toUIEvent(event)
    setLiveEvents(prev => {
      // Avoid duplicates
      if (prev.some(e => e.id === uiEvent.id)) return prev
      // Keep most recent 100 events
      return [uiEvent, ...prev].slice(0, 100)
    })
    // Also invalidate the query to refresh the full list
    queryClient.invalidateQueries({ queryKey: ['events'] })
  }, [queryClient])

  const { isConnected, connect, disconnect } = useEventStream(handleNewEvent)

  // Toggle live mode
  const toggleLive = () => {
    if (isConnected) {
      disconnect()
    } else {
      connect()
    }
  }

  // Auto-connect on mount
  useEffect(() => {
    connect()
    return () => disconnect()
  }, [connect, disconnect])

  // Merge live events with fetched events
  const fetchedEvents = events?.map(toUIEvent) ?? []
  const allEvents = [...liveEvents, ...fetchedEvents]
    .filter((event, index, self) =>
      index === self.findIndex(e => e.id === event.id)
    )
    .sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime())

  return (
    <div className="h-full">
      {/* Filter bar */}
      <div className="h-10 px-4 flex items-center justify-between border-b border-neutral-200 bg-white">
        <div className="flex items-center gap-2">
          <select className="px-2 py-1 text-sm border border-neutral-200 bg-white text-neutral-700">
            <option>All topics</option>
            <option>orders.*</option>
            <option>users.*</option>
            <option>payments.*</option>
          </select>
          <select className="px-2 py-1 text-sm border border-neutral-200 bg-white text-neutral-700">
            <option>Last hour</option>
            <option>Last 24 hours</option>
            <option>Last 7 days</option>
          </select>
        </div>
        <LiveIndicator connected={isConnected} onClick={toggleLive} />
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="p-8 text-center text-neutral-500">Loading events...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-8 text-center text-error">
          Failed to load events: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && allEvents.length === 0 && (
        <div className="p-8 text-center text-neutral-500">
          No events yet. Publish your first event to get started.
        </div>
      )}

      {/* Event stream */}
      <div className="divide-y divide-neutral-100">
        {allEvents.map((event) => (
          <EventRow
            key={event.id}
            event={event}
            selected={selectedEvent?.id === event.id}
            onClick={() => setSelectedEvent(event)}
          />
        ))}
      </div>

      {/* Load more */}
      {allEvents.length > 0 && (
        <div className="p-4 text-center">
          <button className="text-sm text-neutral-500 hover:text-neutral-700">
            ↑ Load more
          </button>
        </div>
      )}

      {/* Detail slide-over */}
      <SlideOver
        open={!!selectedEvent}
        onClose={() => setSelectedEvent(null)}
        title={selectedEvent?.topic}
      >
        {selectedEvent && <EventDetail event={selectedEvent} />}
      </SlideOver>
    </div>
  )
}
