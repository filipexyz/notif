import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router'
import { ClerkProvider, SignedIn, SignedOut, SignInButton } from '@clerk/tanstack-react-start'
import { QueryClientProvider } from '@tanstack/react-query'

import { TopNav } from '../components/layout/TopNav'
import { queryClient } from '../lib/query'

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
        title: 'notif',
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

function RootComponent() {
  return (
    <ClerkProvider>
      <QueryClientProvider client={queryClient}>
        <html lang="en">
          <head>
            <HeadContent />
          </head>
          <body>
            <SignedIn>
              <div className="min-h-screen flex flex-col">
                <TopNav />
                <main className="flex-1">
                  <Outlet />
                </main>
              </div>
            </SignedIn>
            <SignedOut>
              <div className="min-h-screen flex items-center justify-center bg-neutral-50">
                <div className="text-center">
                  <h1 className="text-2xl font-semibold text-neutral-900 mb-2">notif</h1>
                  <p className="text-neutral-500 mb-6">Managed pub/sub event hub</p>
                  <SignInButton mode="modal">
                    <button className="px-6 py-2.5 bg-primary-500 text-white font-medium hover:bg-primary-600 transition-colors">
                      Sign in
                    </button>
                  </SignInButton>
                </div>
              </div>
            </SignedOut>
            <Scripts />
          </body>
        </html>
      </QueryClientProvider>
    </ClerkProvider>
  )
}
