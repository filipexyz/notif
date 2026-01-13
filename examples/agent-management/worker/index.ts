#!/usr/bin/env bun
/**
 * Agent Worker CLI
 *
 * Usage:
 *   bun run index.ts --name <agent-name> --repo <path> [options]
 *
 * Options:
 *   --name, -n        Agent name (required)
 *   --repo, -r        Path to git repository (required)
 *   --budget, -b      Max USD per session
 *   --tags, -t        Comma-separated capability tags
 *   --description, -d Agent description
 *   --help, -h        Show help
 */

import { parseArgs } from 'util'
import path from 'path'
import { AgentWorker } from './worker'
import type { WorkerConfig } from './types'

function showHelp(): void {
  console.log(`
Agent Worker - Claude Code agent with worktree support

Usage:
  bun run index.ts --name <agent-name> --repo <path> [options]

Options:
  --name, -n        Agent name (required)
  --repo, -r        Path to git repository (required)
  --budget, -b      Max USD per session (default: unlimited)
  --tags, -t        Comma-separated capability tags
  --description, -d Agent description
  --help, -h        Show this help

Examples:
  # Start a worker named "coder-1" in the current repo
  bun run index.ts --name coder-1 --repo . --budget 2.0 --tags typescript,react

  # Start with unlimited budget
  bun run index.ts -n researcher -r /path/to/project -t python,ml

Environment:
  NOTIF_API_KEY     API key for notif.sh (required)
`)
}

function parseArguments(): WorkerConfig | null {
  try {
    const { values } = parseArgs({
      options: {
        name: { type: 'string', short: 'n' },
        repo: { type: 'string', short: 'r' },
        budget: { type: 'string', short: 'b' },
        tags: { type: 'string', short: 't' },
        description: { type: 'string', short: 'd' },
        help: { type: 'boolean', short: 'h' },
      },
      allowPositionals: true,
    })

    if (values.help) {
      showHelp()
      return null
    }

    if (!values.name) {
      console.error('Error: --name is required')
      showHelp()
      process.exit(1)
    }

    if (!values.repo) {
      console.error('Error: --repo is required')
      showHelp()
      process.exit(1)
    }

    // Resolve repo path
    const repo = path.resolve(values.repo)

    // Parse budget
    const budget = values.budget ? parseFloat(values.budget) : undefined
    if (budget !== undefined && isNaN(budget)) {
      console.error('Error: --budget must be a number')
      process.exit(1)
    }

    // Parse tags
    const tags = values.tags
      ? values.tags.split(',').map(t => t.trim()).filter(Boolean)
      : []

    // Compute worktree base
    const repoName = path.basename(repo)
    const worktreeBase = path.join(path.dirname(repo), 'worktrees', repoName)

    return {
      name: values.name,
      repo,
      worktreeBase,
      budget,
      tags,
      description: values.description,
    }
  } catch (error) {
    console.error('Error parsing arguments:', error)
    showHelp()
    process.exit(1)
  }
}

async function main(): Promise<void> {
  const config = parseArguments()
  if (!config) {
    return
  }

  // Check for API key
  if (!process.env.NOTIF_API_KEY) {
    console.error('Error: NOTIF_API_KEY environment variable is required')
    process.exit(1)
  }

  const worker = new AgentWorker(config)

  // Handle signals
  const shutdown = () => {
    worker.stop()
    worker.close().then(() => {
      console.log('Worker stopped')
      process.exit(0)
    })
  }

  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)

  try {
    await worker.start()
  } catch (error) {
    console.error('Worker error:', error)
    await worker.close()
    process.exit(1)
  }
}

main()
