import { $ } from 'bun'
import path from 'path'
import type { Worktree } from './types'

/**
 * Manages git worktrees for isolated parallel work
 */
export class WorktreeManager {
  constructor(
    private repoPath: string,
    private basePath: string
  ) {}

  /**
   * Slugify branch name for filesystem
   * e.g., "feat/auth-system" -> "feat-auth-system"
   */
  private slugify(branch: string): string {
    return branch.replace(/\//g, '-').replace(/[^a-zA-Z0-9-_]/g, '')
  }

  /**
   * Get path for a branch
   */
  getPath(branch: string): string {
    return path.join(this.basePath, this.slugify(branch))
  }

  /**
   * Create a new worktree
   * @param branch - Branch name to create
   * @param baseBranch - Base branch to start from (default: main)
   * @returns Path to the created worktree
   */
  async create(branch: string, baseBranch = 'main'): Promise<string> {
    const worktreePath = this.getPath(branch)

    // Ensure base directory exists
    await $`mkdir -p ${this.basePath}`.quiet()

    // Fetch latest from remote
    await $`git -C ${this.repoPath} fetch origin ${baseBranch}`.quiet().nothrow()

    // Create worktree with new branch
    const result = await $`git -C ${this.repoPath} worktree add -b ${branch} ${worktreePath} origin/${baseBranch}`
      .quiet()
      .nothrow()

    if (result.exitCode !== 0) {
      // Branch might already exist, try without -b
      const retryResult = await $`git -C ${this.repoPath} worktree add ${worktreePath} ${branch}`
        .quiet()
        .nothrow()

      if (retryResult.exitCode !== 0) {
        throw new Error(`Failed to create worktree: ${retryResult.stderr.toString()}`)
      }
    }

    return worktreePath
  }

  /**
   * Remove a worktree
   * @param branch - Branch name to remove
   */
  async remove(branch: string): Promise<void> {
    const worktreePath = this.getPath(branch)

    // Remove worktree
    const result = await $`git -C ${this.repoPath} worktree remove ${worktreePath} --force`
      .quiet()
      .nothrow()

    if (result.exitCode !== 0) {
      // Worktree might already be gone, just log warning
      console.warn(`Warning: Could not remove worktree ${worktreePath}: ${result.stderr.toString()}`)
    }

    // Optionally delete the branch
    await $`git -C ${this.repoPath} branch -D ${branch}`.quiet().nothrow()
  }

  /**
   * List all active worktrees
   */
  async list(): Promise<Worktree[]> {
    const result = await $`git -C ${this.repoPath} worktree list --porcelain`.quiet()
    const output = result.stdout.toString()

    const worktrees: Worktree[] = []
    let current: Partial<Worktree> = {}

    for (const line of output.split('\n')) {
      if (line.startsWith('worktree ')) {
        current.path = line.slice(9)
      } else if (line.startsWith('HEAD ')) {
        current.head = line.slice(5)
      } else if (line.startsWith('branch ')) {
        current.branch = line.slice(7).replace('refs/heads/', '')
      } else if (line === '') {
        if (current.path && current.branch && current.head) {
          worktrees.push(current as Worktree)
        }
        current = {}
      }
    }

    // Filter to only show worktrees in our base path
    return worktrees.filter(wt => wt.path.startsWith(this.basePath))
  }

  /**
   * Check if a worktree exists for a branch
   */
  async exists(branch: string): Promise<boolean> {
    const worktrees = await this.list()
    return worktrees.some(wt => wt.branch === branch)
  }
}
