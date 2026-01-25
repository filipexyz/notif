// Mock Clerk context for self-hosted mode
// Provides the same hooks as Clerk but with noop implementations

import { createContext, useContext, ReactNode } from 'react'

interface MockAuthContextValue {
  isLoaded: boolean
  isSignedIn: boolean
  userId: string | null
  sessionId: string | null
  orgId: string | null
  getToken: () => Promise<string | null>
  signOut: () => Promise<void>
}

const MockAuthContext = createContext<MockAuthContextValue>({
  isLoaded: true,
  isSignedIn: true, // Treat self-hosted as "signed in" via API key
  userId: 'self-hosted-user',
  sessionId: 'self-hosted-session',
  orgId: 'self-hosted-org',
  getToken: async () => null,
  signOut: async () => {},
})

export function MockClerkProvider({ children }: { children: ReactNode }) {
  return (
    <MockAuthContext.Provider
      value={{
        isLoaded: true,
        isSignedIn: true,
        userId: 'self-hosted-user',
        sessionId: 'self-hosted-session',
        orgId: 'self-hosted-org',
        getToken: async () => null,
        signOut: async () => {},
      }}
    >
      {children}
    </MockAuthContext.Provider>
  )
}

export function useMockAuth() {
  return useContext(MockAuthContext)
}

// Mock SignedIn component - always renders children
export function MockSignedIn({ children }: { children: ReactNode }) {
  return <>{children}</>
}

// Mock SignedOut component - never renders children
export function MockSignedOut({ children }: { children: ReactNode }) {
  return null
}
