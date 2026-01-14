import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Terminal, Maximize2, Minimize2 } from 'lucide-react'
import { WebTerminal } from '../components/terminal/WebTerminal'

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'error'

export const Route = createFileRoute('/')({
  component: TerminalPage,
})

function TerminalPage() {
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected')

  const stateColors: Record<ConnectionState, string> = {
    connected: 'bg-success/10 text-success',
    connecting: 'bg-warning/10 text-warning',
    disconnected: 'bg-neutral-100 text-neutral-500',
    error: 'bg-error/10 text-error',
  }

  return (
    <div className={`h-full flex flex-col ${isFullscreen ? 'fixed inset-0 z-50 bg-white' : ''}`}>
      {/* Toolbar */}
      <div className="h-10 px-4 flex items-center justify-between border-b border-neutral-200 bg-white shrink-0">
        <div className="flex items-center gap-2">
          <Terminal className="w-4 h-4 text-neutral-500" />
          <span className="text-sm font-medium text-neutral-700">Terminal</span>
          <span className={`text-xs px-2 py-0.5 ${stateColors[connectionState]}`}>
            {connectionState}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="p-1.5 text-neutral-500 hover:text-neutral-700 hover:bg-neutral-100"
          >
            {isFullscreen ? (
              <Minimize2 className="w-4 h-4" />
            ) : (
              <Maximize2 className="w-4 h-4" />
            )}
          </button>
        </div>
      </div>

      {/* Terminal */}
      <div className="flex-1 min-h-0">
        <WebTerminal
          className="h-full"
          onConnectionChange={setConnectionState}
        />
      </div>
    </div>
  )
}
