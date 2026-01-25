import { useState, useRef, useEffect } from 'react'
import { useServer } from '../../lib/server-context'

export function ServerSwitcher() {
  const { server, savedServers, connect, openServerModal } = useServer()
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleSwitchServer = () => {
    openServerModal()
    setIsOpen(false)
  }

  const serverLabel = server?.type === 'cloud' 
    ? 'notif.sh' 
    : (server?.name || server?.url || 'Server')

  const serverIcon = server?.type === 'cloud' ? '☁️' : '🏠'

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-2 py-1 text-sm text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 rounded transition-colors"
      >
        <span>{serverIcon}</span>
        <span className="max-w-[150px] truncate">{serverLabel}</span>
        <svg
          className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 w-64 bg-white border border-neutral-200 shadow-lg z-50">
          {/* Current server */}
          <div className="px-3 py-2 border-b border-neutral-100">
            <div className="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-1">
              Connected to
            </div>
            <div className="flex items-center gap-2">
              <span>{serverIcon}</span>
              <span className="font-medium text-neutral-900 truncate">{serverLabel}</span>
              <span className="ml-auto text-green-500">●</span>
            </div>
          </div>

          {/* Saved servers */}
          {savedServers.length > 0 && (
            <div className="py-1 border-b border-neutral-100">
              <div className="px-3 py-1 text-xs font-medium text-neutral-400 uppercase tracking-wide">
                Saved Servers
              </div>
              {savedServers
                .filter(s => s.url !== server?.url)
                .map((s, i) => (
                  <button
                    key={i}
                    onClick={() => {
                      connect(s)
                      setIsOpen(false)
                    }}
                    className="w-full px-3 py-2 text-left text-sm hover:bg-neutral-50 flex items-center gap-2"
                  >
                    <span>{s.type === 'cloud' ? '☁️' : '🏠'}</span>
                    <span className="truncate">{s.name || s.url || 'notif.sh'}</span>
                  </button>
                ))}
            </div>
          )}

          {/* Actions */}
          <div className="py-1">
            <button
              onClick={handleSwitchServer}
              className="w-full px-3 py-2 text-left text-sm text-neutral-600 hover:bg-neutral-50 flex items-center gap-2"
            >
              <span>🔄</span>
              <span>Switch Server</span>
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
