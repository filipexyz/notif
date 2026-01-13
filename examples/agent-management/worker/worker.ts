import { Notif } from 'notif.sh'
import { hostname } from 'os'
import path from 'path'
import { $ } from 'bun'
import type { Agent, AgentMessage, WorkerConfig } from './types'
import { WorktreeManager } from './worktree'
import { SessionManager } from './session'
import { getClaudeVersion } from './executor'

// Topics
const TOPIC_DISCOVER = 'agents.discover'
const TOPIC_AVAILABLE = 'agents.available'

/**
 * Agent worker that handles discovery and session management
 */
export class AgentWorker {
  private client: Notif
  private worktreeManager: WorktreeManager
  private sessionManager: SessionManager
  private shutdown = false

  constructor(private config: WorkerConfig) {
    this.client = new Notif()

    // Set up worktree manager
    const repoName = path.basename(path.resolve(config.repo))
    const worktreeBase = config.worktreeBase || path.join(path.dirname(path.resolve(config.repo)), 'worktrees', repoName)
    this.worktreeManager = new WorktreeManager(path.resolve(config.repo), worktreeBase)

    // Set up session manager
    this.sessionManager = new SessionManager(
      this.client,
      this.worktreeManager,
      config.name,
      config.budget
    )
  }

  /**
   * Start the worker
   */
  async start(): Promise<void> {
    console.log(`Agent worker started`)
    console.log(`  Name: ${this.config.name}`)
    console.log(`  Repo: ${this.config.repo}`)
    console.log(`  Budget: ${this.config.budget ? `$${this.config.budget.toFixed(2)}` : 'unlimited'}`)
    console.log(`  Tags: ${this.config.tags.join(', ') || 'none'}`)
    console.log()

    // Run all handlers concurrently
    await Promise.all([
      this.handleDiscovery(),
      this.handleSessionCreate(),
      this.handleSessionMessage(),
      this.handleSessionCancel(),
    ])
  }

  /**
   * Handle discovery requests
   */
  private async handleDiscovery(): Promise<void> {
    console.log(`  [discovery] Subscribing to ${TOPIC_DISCOVER}...`)

    try {
      for await (const event of this.client.subscribe(TOPIC_DISCOVER, {
        group: `worker-${this.config.name}`,
      })) {
        if (this.shutdown) break
        await this.announce()
      }
    } catch (error) {
      if (!this.shutdown) {
        console.error(`  [discovery] Error:`, error)
      }
    }
  }

  /**
   * Handle session.create messages
   */
  private async handleSessionCreate(): Promise<void> {
    const topic = `agents.${this.config.name}.session.create`
    console.log(`  [session] Subscribing to ${topic}...`)

    try {
      for await (const event of this.client.subscribe(topic, {
        group: `worker-${this.config.name}`,
      })) {
        if (this.shutdown) break

        // Handle both snake_case (from Rust) and camelCase field names
        const data = event.data as Record<string, unknown>
        const msg: AgentMessage = {
          sessionId: (data.sessionId || data.session_id) as string,
          agent: (data.agent) as string,
          kind: (data.kind) as 'prompt' | 'resume' | 'cancel',
          prompt: data.prompt as string | undefined,
          options: data.options as AgentMessage['options'],
        }
        console.log(`>>> [${msg.sessionId?.slice(0, 12)}] New session: ${msg.prompt?.slice(0, 60)}...`)

        // Run session in background (parallel)
        this.sessionManager.create(msg).catch((error) => {
          console.error(`  [session] Error creating session:`, error)
        })
      }
    } catch (error) {
      if (!this.shutdown) {
        console.error(`  [session] Error:`, error)
      }
    }
  }

  /**
   * Handle session.message (resume) messages
   */
  private async handleSessionMessage(): Promise<void> {
    const topic = `agents.${this.config.name}.session.message`
    console.log(`  [resume] Subscribing to ${topic}...`)

    try {
      for await (const event of this.client.subscribe(topic, {
        group: `worker-${this.config.name}`,
      })) {
        if (this.shutdown) break

        // Handle both snake_case (from Rust) and camelCase field names
        const data = event.data as Record<string, unknown>
        const msg: AgentMessage = {
          sessionId: (data.sessionId || data.session_id) as string,
          agent: (data.agent) as string,
          kind: (data.kind) as 'prompt' | 'resume' | 'cancel',
          prompt: data.prompt as string | undefined,
        }
        console.log(`>>> [${msg.sessionId?.slice(0, 12)}] Resume: ${msg.prompt?.slice(0, 60)}...`)

        // Resume session in background
        this.sessionManager.resume(msg.sessionId, msg.prompt || '').catch((error) => {
          console.error(`  [resume] Error resuming session:`, error)
        })
      }
    } catch (error) {
      if (!this.shutdown) {
        console.error(`  [resume] Error:`, error)
      }
    }
  }

  /**
   * Handle session.cancel messages
   */
  private async handleSessionCancel(): Promise<void> {
    const topic = `agents.${this.config.name}.session.cancel`
    console.log(`  [cancel] Subscribing to ${topic}...`)

    try {
      for await (const event of this.client.subscribe(topic, {
        group: `worker-${this.config.name}`,
      })) {
        if (this.shutdown) break

        // Handle both snake_case (from Rust) and camelCase field names
        const data = event.data as Record<string, unknown>
        const msg: AgentMessage = {
          sessionId: (data.sessionId || data.session_id) as string,
          agent: (data.agent) as string,
          kind: (data.kind) as 'prompt' | 'resume' | 'cancel',
        }
        console.log(`>>> [${msg.sessionId?.slice(0, 12)}] Cancel requested`)

        await this.sessionManager.cancel(msg.sessionId).catch((error) => {
          console.error(`  [cancel] Error cancelling session:`, error)
        })
      }
    } catch (error) {
      if (!this.shutdown) {
        console.error(`  [cancel] Error:`, error)
      }
    }
  }

  /**
   * Announce availability
   */
  private async announce(): Promise<void> {
    const agent: Agent = {
      name: this.config.name,
      description: this.config.description,
      hostname: hostname(),
      tags: this.config.tags,
      executor: {
        kind: 'claude',
        version: await getClaudeVersion(),
        cli: 'claude',
      },
      project: {
        name: path.basename(path.resolve(this.config.repo)),
        path: path.resolve(this.config.repo),
        repo: await this.getGitRemote(),
      },
      status: this.sessionManager.hasActiveSessions() ? 'busy' : 'idle',
      activeSessionId: this.sessionManager.getActiveSessionId(),
    }

    await this.client.emit(TOPIC_AVAILABLE, agent)
    console.log(`  [discovery] Announced availability`)
  }

  /**
   * Get git remote URL
   */
  private async getGitRemote(): Promise<string> {
    try {
      const result = await $`git -C ${this.config.repo} remote get-url origin`.quiet()
      const url = result.stdout.toString().trim()
      // Convert SSH URL to repo format: git@github.com:user/repo.git -> user/repo
      const match = url.match(/github\.com[:/]([^/]+\/[^/]+?)(?:\.git)?$/)
      return match ? match[1] : url
    } catch {
      return 'unknown'
    }
  }

  /**
   * Shutdown the worker
   */
  stop(): void {
    this.shutdown = true
    console.log('\nShutting down...')
  }

  /**
   * Cleanup and close connections
   */
  async close(): Promise<void> {
    await this.client.close()
  }
}
