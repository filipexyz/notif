import type { Notif } from 'notif.sh'
import type { AgentEvent, AgentMessage, Session } from './types'
import type { WorktreeManager } from './worktree'
import { executeClaude } from './executor'

/**
 * Manages parallel Claude sessions
 */
export class SessionManager {
  private sessions = new Map<string, Session>()

  constructor(
    private client: Notif,
    private worktreeManager: WorktreeManager,
    private agentName: string,
    private defaultBudget?: number
  ) {}

  /**
   * Get a session by ID
   */
  get(sessionId: string): Session | undefined {
    return this.sessions.get(sessionId)
  }

  /**
   * Get all active sessions
   */
  getActive(): Session[] {
    return Array.from(this.sessions.values()).filter(
      s => s.status === 'starting' || s.status === 'running'
    )
  }

  /**
   * Get first active session ID (for status reporting)
   */
  getActiveSessionId(): string | undefined {
    const active = this.getActive()
    return active.length > 0 ? active[0].id : undefined
  }

  /**
   * Check if any session is running
   */
  hasActiveSessions(): boolean {
    return this.getActive().length > 0
  }

  /**
   * Create and run a new session
   */
  async create(msg: AgentMessage): Promise<void> {
    const sessionId = msg.sessionId
    const prompt = msg.prompt || ''
    const budget = msg.options?.budgetUsd ?? this.defaultBudget

    // Generate branch name from session ID
    const branch = `agent/${sessionId.slice(0, 12)}`

    // Create session entry
    const session: Session = {
      id: sessionId,
      branch,
      status: 'starting',
      costUsd: 0,
      startedAt: new Date(),
    }
    this.sessions.set(sessionId, session)

    try {
      // Create worktree
      const worktreePath = await this.worktreeManager.create(branch)
      session.worktreePath = worktreePath

      // Emit started event
      await this.emitEvent({
        sessionId,
        agent: this.agentName,
        kind: 'started',
        message: `Worktree created at ${worktreePath}`,
        timestamp: new Date().toISOString(),
      })

      session.status = 'running'

      // Run Claude with streaming
      await this.runClaude(session, prompt, budget)
    } catch (error) {
      session.status = 'failed'
      await this.emitEvent({
        sessionId,
        agent: this.agentName,
        kind: 'failed',
        error: {
          message: error instanceof Error ? error.message : String(error),
        },
        timestamp: new Date().toISOString(),
      })
    }
  }

  /**
   * Resume a session with follow-up prompt
   */
  async resume(sessionId: string, prompt: string): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error(`Session not found: ${sessionId}`)
    }

    if (!session.claudeSessionId) {
      throw new Error(`Session has no Claude session ID: ${sessionId}`)
    }

    session.status = 'running'
    const budget = this.defaultBudget

    try {
      await this.runClaude(session, prompt, budget)
    } catch (error) {
      session.status = 'failed'
      await this.emitEvent({
        sessionId,
        agent: this.agentName,
        kind: 'failed',
        error: {
          message: error instanceof Error ? error.message : String(error),
        },
        timestamp: new Date().toISOString(),
      })
    }
  }

  /**
   * Cancel a running session
   */
  async cancel(sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error(`Session not found: ${sessionId}`)
    }

    // Kill the process if running
    if (session.process) {
      session.process.kill()
    }

    session.status = 'cancelled'

    await this.emitEvent({
      sessionId,
      agent: this.agentName,
      kind: 'failed',
      error: {
        message: 'Session cancelled by user',
        code: 'CANCELLED',
      },
      costUsd: session.costUsd,
      timestamp: new Date().toISOString(),
    })
  }

  /**
   * Run Claude CLI (batch mode)
   */
  private async runClaude(session: Session, prompt: string, budget?: number): Promise<void> {
    const cwd = session.worktreePath || process.cwd()

    const result = await executeClaude({
      prompt,
      cwd,
      budget,
      resumeSession: session.claudeSessionId,
    })

    // Store Claude's internal session ID for resumption
    if (result.claudeSessionId) {
      session.claudeSessionId = result.claudeSessionId
    }
    session.costUsd = result.costUsd

    if (result.isError) {
      session.status = 'failed'
      await this.emitEvent({
        sessionId: session.id,
        agent: this.agentName,
        kind: 'failed',
        error: {
          message: result.result,
        },
        costUsd: session.costUsd,
        timestamp: new Date().toISOString(),
      })
    } else {
      session.status = 'completed'
      await this.emitEvent({
        sessionId: session.id,
        agent: this.agentName,
        kind: 'completed',
        result: result.result,
        costUsd: session.costUsd,
        timestamp: new Date().toISOString(),
      })
    }
  }

  /**
   * Emit an agent event
   */
  private async emitEvent(event: AgentEvent): Promise<void> {
    const topic = `agents.${this.agentName}.session.${event.kind}`
    await this.client.emit(topic, event)
  }
}
