import { createContext, useContext, useState, useCallback, ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { Project } from './types'

const PROJECT_STORAGE_KEY = 'notif_selected_project'

// Sync read from localStorage to avoid race condition on page load
function getStoredProject(): Project | null {
  if (typeof window === 'undefined') return null
  const stored = localStorage.getItem(PROJECT_STORAGE_KEY)
  if (stored) {
    try {
      return JSON.parse(stored)
    } catch {
      localStorage.removeItem(PROJECT_STORAGE_KEY)
    }
  }
  return null
}

type ProjectContextType = {
  selectedProject: Project | null
  setSelectedProject: (project: Project | null) => void
  projectId: string | null
}

const ProjectContext = createContext<ProjectContextType | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  // Initialize synchronously from localStorage to avoid race condition
  const [selectedProject, setSelectedProjectState] = useState<Project | null>(getStoredProject)
  const queryClient = useQueryClient()

  const setSelectedProject = useCallback((project: Project | null) => {
    setSelectedProjectState(project)
    if (project) {
      localStorage.setItem(PROJECT_STORAGE_KEY, JSON.stringify(project))
    } else {
      localStorage.removeItem(PROJECT_STORAGE_KEY)
    }
    // Invalidate all queries to refetch with new project context
    // Exclude 'projects' query since that's org-level, not project-level
    queryClient.invalidateQueries({
      predicate: (query) => query.queryKey[0] !== 'projects',
    })
  }, [queryClient])

  return (
    <ProjectContext.Provider
      value={{
        selectedProject,
        setSelectedProject,
        projectId: selectedProject?.id ?? null,
      }}
    >
      {children}
    </ProjectContext.Provider>
  )
}

export function useProject() {
  const context = useContext(ProjectContext)
  if (!context) {
    throw new Error('useProject must be used within a ProjectProvider')
  }
  return context
}

// Hook to get just the project ID (for convenience)
export function useProjectId() {
  const { projectId } = useProject()
  return projectId
}
