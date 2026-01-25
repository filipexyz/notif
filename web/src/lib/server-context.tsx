import { createContext, useContext, useState, useEffect, ReactNode } from 'react'

export interface ServerConfig {
  type: 'cloud' | 'self-hosted'
  url: string
  apiKey?: string
  name?: string
}

const DEFAULT_CLOUD_SERVER: ServerConfig = {
  type: 'cloud',
  url: import.meta.env.VITE_API_URL || 'https://api.notif.sh',
  name: 'notif.sh Cloud',
}

const STORAGE_KEY = 'notif_server_config'

interface ServerContextValue {
  server: ServerConfig | null
  isConnected: boolean
  isLoading: boolean
  savedServers: ServerConfig[]
  manualDisconnect: boolean
  connect: (config: ServerConfig) => Promise<boolean>
  disconnect: () => void
  testConnection: (config: ServerConfig) => Promise<{ ok: boolean; error?: string }>
  saveServer: (config: ServerConfig) => void
  removeServer: (url: string) => void
}

const ServerContext = createContext<ServerContextValue | null>(null)

export function useServer() {
  const ctx = useContext(ServerContext)
  if (!ctx) {
    throw new Error('useServer must be used within ServerProvider')
  }
  return ctx
}

export function useServerUrl() {
  const { server } = useServer()
  return server?.url || DEFAULT_CLOUD_SERVER.url
}

export function useServerApiKey() {
  const { server } = useServer()
  return server?.apiKey
}

interface StoredData {
  currentServer: ServerConfig | null
  savedServers: ServerConfig[]
}

function loadFromStorage(): StoredData {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch (e) {
    console.error('Failed to load server config from localStorage', e)
  }
  return { currentServer: null, savedServers: [] }
}

function saveToStorage(data: StoredData) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch (e) {
    console.error('Failed to save server config to localStorage', e)
  }
}

export function ServerProvider({ children }: { children: ReactNode }) {
  const [server, setServer] = useState<ServerConfig | null>(null)
  const [savedServers, setSavedServers] = useState<ServerConfig[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [manualDisconnect, setManualDisconnect] = useState(false)

  // Load from localStorage on mount
  useEffect(() => {
    const { currentServer, savedServers: saved } = loadFromStorage()
    setServer(currentServer)
    setSavedServers(saved)
    setIsLoading(false)
  }, [])

  // Save to localStorage on change
  useEffect(() => {
    if (!isLoading) {
      saveToStorage({ currentServer: server, savedServers })
    }
  }, [server, savedServers, isLoading])

  const testConnection = async (config: ServerConfig): Promise<{ ok: boolean; error?: string }> => {
    try {
      const headers: Record<string, string> = {}
      if (config.apiKey) {
        headers['Authorization'] = `Bearer ${config.apiKey}`
      }
      
      // Try health endpoint first (no auth needed)
      const healthRes = await fetch(`${config.url}/health`, { 
        method: 'GET',
        headers,
      })
      
      if (!healthRes.ok) {
        return { ok: false, error: `Server returned ${healthRes.status}` }
      }

      // For self-hosted, verify API key works
      if (config.type === 'self-hosted' && config.apiKey) {
        const statusRes = await fetch(`${config.url}/api/v1/bootstrap/status`, {
          method: 'GET',
          headers,
        })
        
        if (!statusRes.ok) {
          return { ok: false, error: 'Failed to verify server status' }
        }

        const status = await statusRes.json()
        if (!status.self_hosted) {
          return { ok: false, error: 'Server is not in self-hosted mode' }
        }

        // Test API key by listing events
        const eventsRes = await fetch(`${config.url}/api/v1/events?limit=1`, {
          method: 'GET',
          headers,
        })

        if (!eventsRes.ok) {
          return { ok: false, error: 'Invalid API key' }
        }
      }

      return { ok: true }
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : 'Connection failed' }
    }
  }

  const connect = async (config: ServerConfig): Promise<boolean> => {
    // Clear manual disconnect flag when connecting
    setManualDisconnect(false)
    
    // For cloud mode, use default URL and skip connection test (Clerk handles auth)
    if (config.type === 'cloud') {
      setServer({
        ...DEFAULT_CLOUD_SERVER,
        ...config,
      })
      return true
    }
    
    // For self-hosted, test connection first
    const result = await testConnection(config)
    if (result.ok) {
      setServer(config)
      return true
    }
    return false
  }

  const disconnect = () => {
    // Set flag to prevent auto-reconnect
    setManualDisconnect(true)
    setServer(null)
  }

  const saveServer = (config: ServerConfig) => {
    setSavedServers(prev => {
      const existing = prev.findIndex(s => s.url === config.url)
      if (existing >= 0) {
        const updated = [...prev]
        updated[existing] = config
        return updated
      }
      return [...prev, config]
    })
  }

  const removeServer = (url: string) => {
    setSavedServers(prev => prev.filter(s => s.url !== url))
    if (server?.url === url) {
      setServer(null)
    }
  }

  return (
    <ServerContext.Provider
      value={{
        server,
        isConnected: server !== null,
        isLoading,
        savedServers,
        manualDisconnect,
        connect,
        disconnect,
        testConnection,
        saveServer,
        removeServer,
      }}
    >
      {children}
    </ServerContext.Provider>
  )
}
