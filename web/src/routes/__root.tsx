import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router'
import { ClerkProvider, SignedIn, SignedOut } from '@clerk/tanstack-react-start'
import { QueryClientProvider } from '@tanstack/react-query'

import { TopNav } from '../components/layout/TopNav'
import { ServerConnect } from '../components/auth/ServerConnect'
import { queryClient } from '../lib/query'
import { ServerProvider, useServer } from '../lib/server-context'
import { ProjectProvider } from '../lib/project-context'

import appCss from '../styles.css?url'

export const Route = createRootRoute({
  head: () => ({
    meta: [
      {
        charSet: 'utf-8',
      },
      {
        name: 'viewport',
        content: 'width=device-width, initial-scale=1',
      },
      {
        title: 'notif.sh',
      },
    ],
    links: [
      {
        rel: 'stylesheet',
        href: appCss,
      },
      {
        rel: 'preconnect',
        href: 'https://fonts.googleapis.com',
      },
      {
        rel: 'preconnect',
        href: 'https://fonts.gstatic.com',
        crossOrigin: 'anonymous',
      },
      {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap',
      },
    ],
  }),

  component: RootComponent,
})

function AppContent() {
  return (
    <div className="h-screen flex flex-col overflow-hidden">
      <TopNav />
      <main className="flex-1 min-h-0">
        <Outlet />
      </main>
    </div>
  )
}

function SelfHostedApp() {
  const { server } = useServer()
  
  return (
    <ProjectProvider>
      <AppContent />
      <div className="fixed bottom-4 right-4 px-3 py-1.5 bg-amber-100 text-amber-800 text-xs font-medium flex items-center gap-2">
        <span>🏠</span>
        <span>{server?.name || server?.url}</span>
      </div>
    </ProjectProvider>
  )
}

function AuthenticatedApp() {
  return (
    <ProjectProvider>
      <AppContent />
    </ProjectProvider>
  )
}

function ServerRouter() {
  const { server, isConnected, isLoading } = useServer()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-neutral-50">
        <div className="text-neutral-500">Loading...</div>
      </div>
    )
  }

  // No server selected - show connection screen
  if (!isConnected) {
    return <ServerConnect />
  }

  // Self-hosted server - bypass Clerk, use API key auth
  if (server?.type === 'self-hosted') {
    return <SelfHostedApp />
  }

  // Cloud server - use Clerk auth
  return (
    <>
      <SignedIn>
        <AuthenticatedApp />
      </SignedIn>
      <SignedOut>
        <ServerConnect />
      </SignedOut>
    </>
  )
}

function RootComponent() {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        <ClerkProvider>
          <QueryClientProvider client={queryClient}>
            <ServerProvider>
              <ServerRouter />
            </ServerProvider>
          </QueryClientProvider>
        </ClerkProvider>
        <Scripts />
      </body>
    </html>
  )
}
