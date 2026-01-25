// Re-export useAuth from Clerk or mock based on availability
// Components should import useAuth from here, not directly from Clerk

import { useAuth as useClerkAuth } from '@clerk/tanstack-react-start'
import { useMockAuth } from './clerk-mock'
import { useServer } from './server-context'

/**
 * Universal useAuth hook that works in both cloud and self-hosted modes.
 * - In cloud mode: uses real Clerk auth
 * - In self-hosted mode: uses mock auth (returns noop implementations)
 * 
 * Note: This must be called within either ClerkProvider or MockClerkProvider
 */
export function useAuth() {
  const { server } = useServer()
  
  // In self-hosted mode, we're inside MockClerkProvider
  // In cloud mode, we're inside ClerkProvider
  // Both provide their respective useAuth implementations
  
  if (server?.type === 'self-hosted') {
    return useMockAuth()
  }
  
  // Cloud mode - use real Clerk auth
  // This is safe because we're wrapped in ClerkProvider
  return useClerkAuth()
}
