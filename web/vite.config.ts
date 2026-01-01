import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import viteTsConfigPaths from 'vite-tsconfig-paths'
import tailwindcss from '@tailwindcss/vite'
import { nitro } from 'nitro/vite'
import path from 'path'

const config = defineConfig({
  plugins: [
    devtools(),
    nitro(),
    // this is the plugin that enables path aliases
    viteTsConfigPaths({
      projects: ['./tsconfig.json'],
    }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
  ],
  resolve: {
    alias: {
      // Fix for React 19 - redirect shims to our custom re-exports
      'use-sync-external-store/shim/with-selector.js': path.resolve(__dirname, 'src/lib/use-sync-external-store-with-selector.ts'),
      'use-sync-external-store/shim/with-selector': path.resolve(__dirname, 'src/lib/use-sync-external-store-with-selector.ts'),
      'use-sync-external-store/shim/index.js': path.resolve(__dirname, 'src/lib/use-sync-external-store-shim.ts'),
      'use-sync-external-store/shim': path.resolve(__dirname, 'src/lib/use-sync-external-store-shim.ts'),
    },
  },
})

export default config
