import { useAuth } from '@clerk/tanstack-react-start'

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
  token?: string
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
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

// Hook for authenticated requests
export function useApi() {
  const { getToken } = useAuth()

  return async <T>(path: string, options?: RequestInit): Promise<T> => {
    // In anonymous mode, use the dev API key
    if (ANONYMOUS_MODE && DEV_API_KEY) {
      return apiFetch<T>(path, options, DEV_API_KEY)
    }
    const token = await getToken()
    return apiFetch<T>(path, options, token || undefined)
  }
}

// Export for use in components
export const isAnonymousMode = ANONYMOUS_MODE
