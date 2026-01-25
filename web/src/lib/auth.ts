// Universal auth context for both cloud and self-hosted modes
// Components should import useAuth from here, not directly from Clerk

import { createContext, useContext } from 'react'

interface AuthContextValue {
  isLoaded: boolean
  isSignedIn: boolean
  userId: string | null
  sessionId: string | null
  orgId: string | null
  getToken: () => Promise<string | null>
  signOut: () => Promise<void>
}

// Default noop auth for self-hosted mode
const defaultAuth: AuthContextValue = {
  isLoaded: true,
  isSignedIn: true, // Treat self-hosted as "signed in" via API key
  userId: 'self-hosted-user',
  sessionId: 'self-hosted-session',
  orgId: 'self-hosted-org',
  getToken: async () => null,
  signOut: async () => {},
}

export const AuthContext = createContext<AuthContextValue>(defaultAuth)

/**
 * Universal useAuth hook that works in both cloud and self-hosted modes.
 * The actual implementation is provided by the appropriate provider:
 * - ClerkAuthProvider (cloud mode) - wraps Clerk's useAuth
 * - MockAuthProvider (self-hosted) - provides noop implementations
 */
export function useAuth() {
  return useContext(AuthContext)
}
