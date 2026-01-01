import { defineConfig } from '@tanstack/react-start/config'
import { clerkMiddleware } from '@clerk/tanstack-react-start/server'

export default defineConfig({
  server: {
    middleware: [clerkMiddleware()],
  },
})
