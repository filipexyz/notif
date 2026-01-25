import { useState, ReactNode } from 'react'
import { SignInButton } from '@clerk/tanstack-react-start'
import { useServer, ServerConfig } from '../../lib/server-context'

// Check if Clerk is configured
const CLERK_AVAILABLE = !!import.meta.env.VITE_CLERK_PUBLISHABLE_KEY

interface ServerConnectProps {
  isModal?: boolean
  onClose?: () => void
}

export function ServerConnect({ isModal = false, onClose }: ServerConnectProps) {
  const { connect, testConnection, savedServers, isLoading, clearManualDisconnect, closeServerModal } = useServer()
  // If Clerk not configured, skip to self-hosted directly
  const [mode, setMode] = useState<'select' | 'cloud' | 'self-hosted'>(
    CLERK_AVAILABLE ? 'select' : 'self-hosted'
  )
  const [serverUrl, setServerUrl] = useState('http://localhost:8080')
  const [apiKey, setApiKey] = useState('')
  const [serverName, setServerName] = useState('')
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testSuccess, setTestSuccess] = useState(false)

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-neutral-50">
        <div className="text-neutral-500">Loading...</div>
      </div>
    )
  }

  const handleTestConnection = async () => {
    setTesting(true)
    setError(null)
    setTestSuccess(false)

    const result = await testConnection({
      type: 'self-hosted',
      url: serverUrl,
      apiKey,
    })
    
    setTesting(false)
    if (result.ok) {
      setTestSuccess(true)
    } else {
      setError(result.error || 'Connection failed')
    }
  }

  const handleClose = () => {
    if (onClose) onClose()
    else closeServerModal()
  }

  const handleConnect = async () => {
    const config: ServerConfig = {
      type: 'self-hosted',
      url: serverUrl,
      apiKey,
      name: serverName || undefined,
    }
    const success = await connect(config)
    if (success && isModal) {
      handleClose()
    }
  }

  const handleConnectSaved = async (server: ServerConfig) => {
    const success = await connect(server)
    if (success && isModal) {
      handleClose()
    }
  }

  // Wrapper for modal vs fullscreen
  const Container = ({ children }: { children: ReactNode }) => {
    if (isModal) {
      return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={handleClose}>
          <div className="bg-neutral-50 w-full max-w-md max-h-[90vh] overflow-y-auto relative" onClick={e => e.stopPropagation()}>
            <button
              onClick={handleClose}
              className="absolute top-4 right-4 text-neutral-400 hover:text-neutral-600 z-10"
              aria-label="Close"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            {children}
          </div>
        </div>
      )
    }
    return <div className="min-h-screen flex items-center justify-center bg-neutral-50">{children}</div>
  }

  // Server selection screen
  if (mode === 'select') {
    return (
      <Container>
        <div className="w-full max-w-md p-8">
          <div className="text-center mb-8">
            <h1 className="text-3xl font-bold text-neutral-900 mb-2">notif.sh</h1>
            <p className="text-neutral-500">Choose how to connect</p>
          </div>

          <div className="space-y-3">
            {CLERK_AVAILABLE && (
              <button
                onClick={() => {
                  clearManualDisconnect()  // Allow auto-connect after Sign In
                  setMode('cloud')
                }}
                className="w-full p-4 text-left bg-white border border-neutral-200 hover:border-primary-500 hover:bg-primary-50 transition-colors group"
              >
                <div className="flex items-center gap-3">
                  <span className="text-2xl">☁️</span>
                  <div>
                    <div className="font-medium text-neutral-900 group-hover:text-primary-600">
                      notif.sh Cloud
                    </div>
                    <div className="text-sm text-neutral-500">
                      Managed service — sign in with your account
                    </div>
                  </div>
                </div>
              </button>
            )}

            <button
              onClick={() => setMode('self-hosted')}
              className="w-full p-4 text-left bg-white border border-neutral-200 hover:border-primary-500 hover:bg-primary-50 transition-colors group"
            >
              <div className="flex items-center gap-3">
                <span className="text-2xl">🏠</span>
                <div>
                  <div className="font-medium text-neutral-900 group-hover:text-primary-600">
                    Self-hosted server
                  </div>
                  <div className="text-sm text-neutral-500">
                    Connect to your own notif.sh instance
                  </div>
                </div>
              </div>
            </button>
          </div>

          {/* Saved servers */}
          {savedServers.length > 0 && (
            <div className="mt-8">
              <div className="text-sm font-medium text-neutral-500 mb-3">Recent servers</div>
              <div className="space-y-2">
                {savedServers.map((server, i) => (
                  <button
                    key={i}
                    onClick={() => handleConnectSaved(server)}
                    className="w-full p-3 text-left bg-white border border-neutral-200 hover:border-neutral-300 transition-colors text-sm"
                  >
                    <div className="flex items-center gap-2">
                      <span>{server.type === 'cloud' ? '☁️' : '🏠'}</span>
                      <span className="font-medium text-neutral-700">
                        {server.name || server.url || 'notif.sh Cloud'}
                      </span>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </Container>
    )
  }

  // Cloud sign-in
  if (mode === 'cloud') {
    return (
      <Container>
        <div className="w-full max-w-md p-8">
          <button
            onClick={() => setMode('select')}
            className="text-neutral-500 hover:text-neutral-700 mb-6 flex items-center gap-1"
          >
            ← Back
          </button>

          <div className="text-center">
            <div className="text-4xl mb-4">☁️</div>
            <h1 className="text-2xl font-semibold text-neutral-900 mb-2">notif.sh Cloud</h1>
            <p className="text-neutral-500 mb-8">Sign in to access your events and webhooks</p>
            
            <SignInButton mode="modal">
              <button className="w-full px-6 py-3 bg-primary-500 text-white font-medium hover:bg-primary-600 transition-colors">
                Sign In
              </button>
            </SignInButton>
          </div>
        </div>
      </Container>
    )
  }

  // Self-hosted connection
  return (
    <Container>
      <div className="w-full max-w-md p-8">
        {CLERK_AVAILABLE && (
          <button
            onClick={() => setMode('select')}
            className="text-neutral-500 hover:text-neutral-700 mb-6 flex items-center gap-1"
          >
            ← Back
          </button>
        )}

        <div className="text-center mb-8">
          <div className="text-4xl mb-4">🏠</div>
          <h1 className="text-2xl font-semibold text-neutral-900 mb-2">Self-hosted Server</h1>
          <p className="text-neutral-500">Connect to your notif.sh instance</p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">
              Server URL
            </label>
            <input
              type="url"
              value={serverUrl}
              onChange={(e) => setServerUrl(e.target.value)}
              placeholder="http://localhost:8080"
              className="w-full px-3 py-2 border border-neutral-300 focus:border-primary-500 focus:ring-1 focus:ring-primary-500 outline-none"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">
              API Key
            </label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="nsh_..."
              className="w-full px-3 py-2 border border-neutral-300 focus:border-primary-500 focus:ring-1 focus:ring-primary-500 outline-none font-mono"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">
              Name (optional)
            </label>
            <input
              type="text"
              value={serverName}
              onChange={(e) => setServerName(e.target.value)}
              placeholder="My Server"
              className="w-full px-3 py-2 border border-neutral-300 focus:border-primary-500 focus:ring-1 focus:ring-primary-500 outline-none"
            />
          </div>

          {error && (
            <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm">
              {error}
            </div>
          )}

          {testSuccess && (
            <div className="p-3 bg-green-50 border border-green-200 text-green-700 text-sm">
              ✓ Connection successful!
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={handleTestConnection}
              disabled={testing || !serverUrl || !apiKey}
              className="flex-1 px-4 py-2 border border-neutral-300 text-neutral-700 font-medium hover:bg-neutral-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {testing ? 'Testing...' : 'Test Connection'}
            </button>
            <button
              onClick={handleConnect}
              disabled={!testSuccess}
              className="flex-1 px-4 py-2 bg-primary-500 text-white font-medium hover:bg-primary-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Connect
            </button>
          </div>
        </div>
      </div>
    </Container>
  )
}
