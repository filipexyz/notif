import type { ClaudeEvent, ClaudeResultEvent, ClaudeSystemEvent } from './types'

export type PermissionMode = 'default' | 'acceptEdits' | 'bypassPermissions' | 'plan'

export interface ExecuteOptions {
  prompt: string
  cwd: string
  budget?: number
  resumeSession?: string
  permissionMode?: PermissionMode
}

export interface ExecuteResult {
  claudeSessionId: string
  result: string
  isError: boolean
  costUsd: number
}

/**
 * Execute Claude CLI and wait for completion (batch mode)
 * This matches the Python worker approach - simpler and more reliable
 */
export async function executeClaude(opts: ExecuteOptions): Promise<ExecuteResult> {
  const args = ['-p', '--output-format', 'json']

  // Default to bypassPermissions for full agent autonomy
  const permMode = opts.permissionMode || 'bypassPermissions'
  args.push('--permission-mode', permMode)

  if (opts.resumeSession) {
    args.push('--resume', opts.resumeSession)
  }

  console.log(`  [claude] Running: claude ${args.join(' ')}`)
  console.log(`  [claude] CWD: ${opts.cwd}`)
  console.log(`  [claude] Prompt: ${opts.prompt.slice(0, 60)}...`)

  // Use Bun.spawn with subprocess pattern matching Python worker
  const proc = Bun.spawn(['claude', ...args], {
    cwd: opts.cwd,
    stdin: 'pipe',
    stdout: 'pipe',
    stderr: 'pipe',
  })

  console.log(`  [claude] Process spawned, PID: ${proc.pid}`)

  // Write prompt to stdin and close immediately using FileSink
  proc.stdin.write(opts.prompt)
  proc.stdin.flush()
  proc.stdin.end()
  console.log(`  [claude] Stdin written and closed`)

  // Read stdout and stderr
  console.log(`  [claude] Waiting for output...`)
  const stdout = await new Response(proc.stdout).text()
  const stderr = await new Response(proc.stderr).text()

  console.log(`  [claude] stdout length: ${stdout.length}`)
  console.log(`  [claude] stderr length: ${stderr.length}`)
  if (stderr) {
    console.log(`  [claude] stderr: ${stderr.slice(0, 200)}`)
  }

  const exitCode = await proc.exited
  console.log(`  [claude] Exit code: ${exitCode}`)

  if (exitCode !== 0) {
    console.log(`  [claude] Error output: ${stderr || stdout}`)
    return {
      claudeSessionId: '',
      result: stderr || stdout || `Claude CLI failed with exit code ${exitCode}`,
      isError: true,
      costUsd: 0,
    }
  }

  try {
    // Claude outputs array of messages, last one has result
    console.log(`  [claude] Parsing JSON output...`)
    const output = JSON.parse(stdout)
    const messages = Array.isArray(output) ? output : [output]
    console.log(`  [claude] Messages count: ${messages.length}`)

    const last = messages[messages.length - 1]
    const first = messages[0]

    const result: ExecuteResult = {
      claudeSessionId: first?.session_id || '',
      result: last?.result || '',
      isError: last?.is_error || false,
      costUsd: last?.total_cost_usd || 0,
    }

    console.log(`  [claude] Session: ${result.claudeSessionId}`)
    console.log(`  [claude] Cost: $${result.costUsd.toFixed(4)}`)
    console.log(`  [claude] Result: ${result.result.slice(0, 200)}...`)

    return result
  } catch (e) {
    console.log(`  [claude] Parse error: ${e}`)
    console.log(`  [claude] Raw stdout: ${stdout.slice(0, 500)}`)
    return {
      claudeSessionId: '',
      result: stdout || String(e),
      isError: false,
      costUsd: 0,
    }
  }
}

/**
 * Get Claude CLI version
 */
export async function getClaudeVersion(): Promise<string> {
  try {
    const proc = Bun.spawn(['claude', '--version'], {
      stdout: 'pipe',
      stderr: 'pipe',
    })
    const output = await new Response(proc.stdout).text()
    return output.trim()
  } catch {
    return 'unknown'
  }
}
