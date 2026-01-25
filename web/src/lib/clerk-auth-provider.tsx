// Clerk auth provider that bridges Clerk's useAuth to our AuthContext
// Used in cloud mode to provide real Clerk authentication

import { ReactNode } from 'react'
import { useAuth as useClerkAuth } from '@clerk/tanstack-react-start'
import { AuthContext } from './auth'

export function ClerkAuthProvider({ children }: { children: ReactNode }) {
  const clerkAuth = useClerkAuth()
  
  const authValue = {
    isLoaded: clerkAuth.isLoaded,
    isSignedIn: clerkAuth.isSignedIn ?? false,
    userId: clerkAuth.userId ?? null,
    sessionId: clerkAuth.sessionId ?? null,
    orgId: clerkAuth.orgId ?? null,
    getToken: clerkAuth.getToken,
    signOut: clerkAuth.signOut,
  }
  
  return (
    <AuthContext.Provider value={authValue}>
      {children}
    </AuthContext.Provider>
  )
}
