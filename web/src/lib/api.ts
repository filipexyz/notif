import { useAuth } from '@clerk/tanstack-react-start'
import { useProjectId } from './project-context'

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const ANONYMOUS_MODE = import.meta.env.VITE_ANONYMOUS_MODE === 'true'
const DEV_API_KEY = import.meta.env.VITE_DEV_API_KEY

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
  projectId?: string | null
): Promise<T> {
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

  const res = await fetch(`${API_BASE}${path}`, {
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

// Hook for authenticated requests with project context
export function useApi() {
  const { getToken } = useAuth()
  const projectId = useProjectId()

  return async <T>(path: string, options?: RequestInit): Promise<T> => {
    // In anonymous mode, use the dev API key (project derived from key)
    if (ANONYMOUS_MODE && DEV_API_KEY) {
      return apiFetch<T>(path, options, DEV_API_KEY, null)
    }
    const token = await getToken()
    return apiFetch<T>(path, options, token || undefined, projectId)
  }
}

// Export for use in components
export const isAnonymousMode = ANONYMOUS_MODE
