// Mock Clerk context for self-hosted mode
// Provides the same interface as Clerk but with noop implementations

import { ReactNode } from 'react'
import { AuthContext } from './auth'

// Self-hosted auth values
const selfHostedAuth = {
  isLoaded: true,
  isSignedIn: true, // Treat self-hosted as "signed in" via API key
  userId: 'self-hosted-user',
  sessionId: 'self-hosted-session',
  orgId: 'self-hosted-org',
  getToken: async () => null,
  signOut: async () => {},
}

export function MockAuthProvider({ children }: { children: ReactNode }) {
  return (
    <AuthContext.Provider value={selfHostedAuth}>
      {children}
    </AuthContext.Provider>
  )
}
