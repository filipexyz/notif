import { useEffect, useRef, useState, useCallback } from 'react'
import { useAuth } from '@clerk/tanstack-react-start'
import { useServer, useServerUrl, useServerApiKey } from './server-context'
import type { StoredEvent } from './types'

type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error'

export function useEventStream(onEvent: (event: StoredEvent) => void) {
  const { getToken } = useAuth()
  const { server } = useServer()
  const serverUrl = useServerUrl()
  const serverApiKey = useServerApiKey()
  
  const wsRef = useRef<WebSocket | null>(null)
  const [state, setState] = useState<ConnectionState>('disconnected')
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>()

  const connect = useCallback(async () => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return
    }

    setState('connecting')

    try {
      // Build WebSocket URL from server URL
      const wsBase = serverUrl.replace('http', 'ws')
      const url = new URL('/ws', wsBase)
      
      // Use appropriate token based on server type
      let token: string | undefined
      if (server?.type === 'self-hosted' && serverApiKey) {
        token = serverApiKey
      } else {
        token = await getToken() || undefined
      }
      
      if (token) {
        url.searchParams.set('token', token)
      }

      const ws = new WebSocket(url.toString())
      wsRef.current = ws

      ws.onopen = () => {
        setState('connected')
        // Subscribe to all events
        ws.send(JSON.stringify({
          action: 'subscribe',
          topics: ['*'],
          options: { auto_ack: true }
        }))
      }

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data)
          if (msg.type === 'event' && msg.event) {
            onEvent(msg.event)
          }
        } catch {
          // Ignore parse errors
        }
      }

      ws.onerror = () => {
        setState('error')
      }

      ws.onclose = () => {
        setState('disconnected')
        wsRef.current = null
        // Reconnect after 3 seconds
        reconnectTimeoutRef.current = setTimeout(connect, 3000)
      }
    } catch {
      setState('error')
    }
  }, [getToken, onEvent, server, serverUrl, serverApiKey])

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setState('disconnected')
  }, [])

  useEffect(() => {
    return () => {
      disconnect()
    }
  }, [disconnect])

  return {
    state,
    connect,
    disconnect,
    isConnected: state === 'connected',
  }
}
