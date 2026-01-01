import { useAuth } from '@clerk/tanstack-react-start'

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'

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
    const token = await getToken()
    return apiFetch<T>(path, options, token || undefined)
  }
}
