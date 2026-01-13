/**
 * Re-export generated types for worker use
 */
export type {
  Agent,
  AgentExecutor,
  AgentProject,
} from '../generated/typescript/agent'

export type {
  AgentMessage,
  AgentMessageOptions,
} from '../generated/typescript/agentmessage'

export type {
  AgentEvent,
  AgentEventError,
  AgentEventPr,
} from '../generated/typescript/agentevent'

/**
 * Internal session state (not part of protocol)
 */
export interface Session {
  /** Our session ID (from AgentMessage.sessionId) */
  id: string
  /** Claude's internal UUID (from first response) */
  claudeSessionId?: string
  /** Path to worktree directory */
  worktreePath?: string
  /** Branch name */
  branch?: string
  /** Session status */
  status: 'starting' | 'running' | 'completed' | 'failed' | 'cancelled'
  /** Cost incurred so far */
  costUsd: number
  /** When session started */
  startedAt: Date
  /** Subprocess handle for cancellation */
  process?: ReturnType<typeof Bun.spawn>
}

/**
 * Worktree info from git
 */
export interface Worktree {
  path: string
  branch: string
  head: string
}

/**
 * Worker configuration from CLI
 */
export interface WorkerConfig {
  name: string
  repo: string
  worktreeBase: string
  budget?: number
  tags: string[]
  description?: string
}

/**
 * Claude CLI streaming event types
 */
export interface ClaudeSystemEvent {
  type: 'system'
  session_id: string
  tools: string[]
  model: string
}

export interface ClaudeAssistantEvent {
  type: 'assistant'
  message: {
    content: Array<{ type: string; text?: string }>
  }
}

export interface ClaudeResultEvent {
  type: 'result'
  result: string
  is_error: boolean
  total_cost_usd: number
  session_id: string
}

export type ClaudeEvent = ClaudeSystemEvent | ClaudeAssistantEvent | ClaudeResultEvent | Record<string, unknown>
