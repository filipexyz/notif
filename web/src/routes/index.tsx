import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Send } from 'lucide-react'
import { EventRow } from '../components/events/EventRow'
import { EventDetail } from '../components/events/EventDetail'
import { LiveIndicator } from '../components/events/LiveIndicator'
import { SlideOver } from '../components/layout/SlideOver'
import { Button } from '../components/ui'
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
  const [showEmitModal, setShowEmitModal] = useState(false)
  const [emitTopic, setEmitTopic] = useState('test.event')
  const [emitData, setEmitData] = useState('{\n  "message": "Hello world"\n}')
  const api = useApi()
  const queryClient = useQueryClient()

  const emitMutation = useMutation({
    mutationFn: (payload: { topic: string; data: unknown }) =>
      api<{ id: string }>('/api/v1/emit', {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      setShowEmitModal(false)
      queryClient.invalidateQueries({ queryKey: ['events'] })
    },
  })

  const handleEmit = () => {
    try {
      const data = JSON.parse(emitData)
      emitMutation.mutate({ topic: emitTopic, data })
    } catch {
      alert('Invalid JSON')
    }
  }

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
        <div className="flex items-center gap-3">
          <Button size="sm" onClick={() => setShowEmitModal(true)}>
            <Send className="w-3.5 h-3.5 mr-1.5" />
            Send Event
          </Button>
          <LiveIndicator connected={isConnected} onClick={toggleLive} />
        </div>
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

      {/* Emit modal */}
      {showEmitModal && (
        <>
          <div
            className="fixed inset-0 bg-neutral-900/20 z-40"
            onClick={() => setShowEmitModal(false)}
          />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-md bg-white border border-neutral-200 p-6 z-50">
            <h3 className="text-lg font-medium text-neutral-900 mb-4">Send Event</h3>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">
                  Topic
                </label>
                <input
                  type="text"
                  value={emitTopic}
                  onChange={(e) => setEmitTopic(e.target.value)}
                  placeholder="orders.created"
                  className="w-full px-3 py-2 text-sm border border-neutral-200 font-mono"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">
                  Data (JSON)
                </label>
                <textarea
                  value={emitData}
                  onChange={(e) => setEmitData(e.target.value)}
                  rows={6}
                  className="w-full px-3 py-2 text-sm border border-neutral-200 font-mono"
                />
              </div>

              {emitMutation.error && (
                <div className="text-sm text-error">
                  {emitMutation.error.message}
                </div>
              )}

              <div className="flex gap-2 pt-2">
                <Button onClick={handleEmit} disabled={emitMutation.isPending}>
                  {emitMutation.isPending ? 'Sending...' : 'Send'}
                </Button>
                <Button variant="secondary" onClick={() => setShowEmitModal(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
