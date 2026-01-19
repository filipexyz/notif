import { useEffect, useRef, useState, useCallback } from 'react'
import { useAuth } from '@clerk/tanstack-react-start'
import type { Terminal as TerminalType } from '@xterm/xterm'
import type { FitAddon as FitAddonType } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useProject } from '../../lib/project-context'

const WS_BASE = import.meta.env.VITE_API_URL?.replace('http', 'ws') || 'ws://localhost:8080'

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error'

interface TerminalMessage {
  type: string
  data?: string
  sessionId?: string
  cols?: number
  rows?: number
  reason?: string
  code?: string
  message?: string
}

interface WebTerminalProps {
  className?: string
  onConnectionChange?: (state: ConnectionState) => void
}

export function WebTerminal({ className, onConnectionChange }: WebTerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<TerminalType | null>(null)
  const fitAddonRef = useRef<FitAddonType | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const { getToken } = useAuth()
  const { projectId, isHydrated } = useProject()
  const [state, setState] = useState<ConnectionState>('disconnected')

  const updateState = useCallback((newState: ConnectionState) => {
    setState(newState)
    onConnectionChange?.(newState)
  }, [onConnectionChange])

  const sendResize = useCallback(() => {
    const term = termRef.current
    const ws = wsRef.current
    if (term && ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'resize',
        cols: term.cols,
        rows: term.rows,
      }))
    }
  }, [])

  const connect = useCallback(async () => {
    const term = termRef.current
    if (!term) return

    // Wait for project to be selected before connecting
    if (!projectId) {
      term.writeln('\x1b[33m[Waiting for project selection...]\x1b[0m')
      return
    }

    updateState('connecting')

    try {
      const token = await getToken()
      const url = new URL('/ws/terminal', WS_BASE)
      if (token) {
        url.searchParams.set('token', token)
      }
      // Include project ID for proper context
      url.searchParams.set('project_id', projectId)

      const ws = new WebSocket(url.toString())
      wsRef.current = ws

      ws.onopen = () => {
        // Send connect message with terminal size
        ws.send(JSON.stringify({
          type: 'connect',
          cols: term.cols,
          rows: term.rows,
        }))
      }

      ws.onmessage = (e) => {
        try {
          const msg: TerminalMessage = JSON.parse(e.data)

          switch (msg.type) {
            case 'connected':
              updateState('connected')
              break
            case 'output':
              if (msg.data) {
                term.write(msg.data)
              }
              break
            case 'closed':
              updateState('disconnected')
              term.writeln(`\r\n\x1b[33m[Session ended: ${msg.reason || 'unknown'}]\x1b[0m`)
              break
            case 'error':
              updateState('error')
              term.writeln(`\r\n\x1b[31m[Error: ${msg.message || 'unknown'}]\x1b[0m`)
              break
          }
        } catch {
          // Ignore parse errors
        }
      }

      ws.onerror = () => {
        updateState('error')
      }

      ws.onclose = () => {
        updateState('disconnected')
        wsRef.current = null
      }

      // Handle terminal input
      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'input', data }))
        }
      })

    } catch (err) {
      updateState('error')
      termRef.current?.writeln(`\r\n\x1b[31m[Connection failed]\x1b[0m`)
    }
  }, [getToken, updateState, projectId])

  // Initialize terminal
  useEffect(() => {
    if (!containerRef.current) return

    // Dynamic import for SSR compatibility (xterm requires DOM)
    let mounted = true
    let cleanupFn: (() => void) | undefined

    const initTerminal = async () => {
      const [{ Terminal }, { FitAddon }, { WebLinksAddon }] = await Promise.all([
        import('@xterm/xterm'),
        import('@xterm/addon-fit'),
        import('@xterm/addon-web-links'),
      ])

      if (!mounted || !containerRef.current) return

      const term = new Terminal({
        cursorBlink: true,
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 14,
        lineHeight: 1.2,
        theme: {
          background: '#fafafa',
          foreground: '#171717',
          cursor: '#a855f7',
          cursorAccent: '#fafafa',
          selectionBackground: '#a855f740',
          black: '#171717',
          red: '#ef4444',
          green: '#22c55e',
          yellow: '#f59e0b',
          blue: '#3b82f6',
          magenta: '#a855f7',
          cyan: '#06b6d4',
          white: '#e5e5e5',
          brightBlack: '#737373',
          brightRed: '#f87171',
          brightGreen: '#4ade80',
          brightYellow: '#fbbf24',
          brightBlue: '#60a5fa',
          brightMagenta: '#c084fc',
          brightCyan: '#22d3ee',
          brightWhite: '#fafafa',
        },
      })

      const fitAddon = new FitAddon()
      const webLinksAddon = new WebLinksAddon()

      term.loadAddon(fitAddon)
      term.loadAddon(webLinksAddon)
      term.open(containerRef.current)
      fitAddon.fit()

      termRef.current = term
      fitAddonRef.current = fitAddon

      // Focus terminal
      term.focus()

      // Handle window resize
      const handleResize = () => {
        fitAddon.fit()
        sendResize()
      }
      window.addEventListener('resize', handleResize)

      // Connect on mount
      connect()

      // Store cleanup function
      cleanupFn = () => {
        window.removeEventListener('resize', handleResize)
      }
    }

    initTerminal()

    return () => {
      mounted = false
      cleanupFn?.()
      wsRef.current?.close()
      termRef.current?.dispose()
    }
  }, [connect, sendResize])

  // Connect when projectId becomes available
  useEffect(() => {
    // Only try to connect if terminal is ready and we have a project but aren't connected yet
    if (termRef.current && projectId && state === 'disconnected' && !wsRef.current) {
      connect()
    }
  }, [projectId, state, connect])

  // Re-fit on container resize
  useEffect(() => {
    const container = containerRef.current
    if (!container || !fitAddonRef.current) return

    const resizeObserver = new ResizeObserver(() => {
      fitAddonRef.current?.fit()
      sendResize()
    })

    resizeObserver.observe(container)
    return () => resizeObserver.disconnect()
  }, [sendResize])

  return (
    <div
      ref={containerRef}
      className={`${className || ''} bg-neutral-50 border border-neutral-200`}
      style={{ padding: '8px' }}
    />
  )
}

export function useTerminalState() {
  const [state, setState] = useState<ConnectionState>('disconnected')
  return { state, setState }
}
