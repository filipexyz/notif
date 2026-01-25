import { useAuth } from './auth'
import { useProjectId, useProject } from './project-context'
import { useServer, useServerUrl, useServerApiKey } from './server-context'

// Legacy env-based config (for backwards compatibility)
const ENV_API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const ANONYMOUS_MODE = import.meta.env.VITE_ANONYMOUS_MODE === 'true'
const DEV_API_KEY = import.meta.env.VITE_DEV_API_KEY

// Hook to check if project context is ready for queries
// Use this with React Query's `enabled` option to prevent queries from firing
// before a project is selected and hydration is complete
export function useProjectReady() {
  const { selectedProject, isHydrated } = useProject()
  const { server, isConnected } = useServer()
  
  // Not connected to any server yet
  if (!isConnected) return false
  
  // Self-hosted: project is derived from API key, always ready once connected
  if (server?.type === 'self-hosted') return true
  
  // Cloud: need project selection
  if (ANONYMOUS_MODE && DEV_API_KEY) return isHydrated
  return isHydrated && selectedProject !== null
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
  token?: string,
  projectId?: string | null,
  baseUrl?: string
): Promise<T> {
  const apiBase = baseUrl || ENV_API_BASE
  
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  // Add project ID header for Clerk-authenticated requests (not API key)
  // API key requests derive project from the key itself
  if (projectId && !token?.startsWith('nsh_')) {
    headers['X-Project-ID'] = projectId
  }

  const res = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: {
      ...headers,
      ...options.headers,
    },
  })

  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text || `API error: ${res.status}`)
  }

  // Handle empty responses
  const text = await res.text()
  if (!text) return undefined as T

  return JSON.parse(text)
}

// Hook for authenticated requests with server and project context
export function useApi() {
  const { getToken } = useAuth()
  const projectId = useProjectId()
  const { server } = useServer()
  const serverUrl = useServerUrl()
  const serverApiKey = useServerApiKey()

  return async <T>(path: string, options?: RequestInit): Promise<T> => {
    // Self-hosted: use API key from server config
    if (server?.type === 'self-hosted' && serverApiKey) {
      return apiFetch<T>(path, options, serverApiKey, null, serverUrl)
    }
    
    // Legacy anonymous mode (env-based)
    if (ANONYMOUS_MODE && DEV_API_KEY) {
      return apiFetch<T>(path, options, DEV_API_KEY, null)
    }
    
    // Cloud: use Clerk token
    const token = await getToken()
    return apiFetch<T>(path, options, token || undefined, projectId, serverUrl)
  }
}

// Export for use in components
export const isAnonymousMode = ANONYMOUS_MODE
