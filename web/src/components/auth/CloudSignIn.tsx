// This component is only loaded when Clerk is available
// It's dynamically imported to avoid bundling Clerk when not configured

import { SignInButton } from '@clerk/tanstack-react-start'

interface CloudSignInProps {
  onBack: () => void
}

export function CloudSignIn({ onBack }: CloudSignInProps) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-neutral-50">
      <div className="w-full max-w-md p-8">
        <button
          onClick={onBack}
          className="text-neutral-500 hover:text-neutral-700 mb-6 flex items-center gap-1"
        >
          ← Back
        </button>

        <div className="text-center">
          <div className="text-4xl mb-4">☁️</div>
          <h1 className="text-2xl font-semibold text-neutral-900 mb-2">notif.sh Cloud</h1>
          <p className="text-neutral-500 mb-8">Sign in to access your events and webhooks</p>
          
          <SignInButton mode="modal">
            <button className="w-full px-6 py-3 bg-primary-500 text-white font-medium hover:bg-primary-600 transition-colors">
              Sign in with Clerk
            </button>
          </SignInButton>
        </div>
      </div>
    </div>
  )
}
