import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type { Project } from './types'

const PROJECT_STORAGE_KEY = 'notif_selected_project'

// Read from localStorage (client-side only)
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
  isHydrated: boolean // True once client-side hydration is complete
}

const ProjectContext = createContext<ProjectContextType | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  // Start with null, then hydrate from localStorage on client
  const [selectedProject, setSelectedProjectState] = useState<Project | null>(null)
  const [isHydrated, setIsHydrated] = useState(false)

  // Hydrate from localStorage on client mount
  useEffect(() => {
    const stored = getStoredProject()
    if (stored) {
      setSelectedProjectState(stored)
    }
    setIsHydrated(true)
  }, [])

  const setSelectedProject = useCallback((project: Project | null) => {
    const currentId = getStoredProject()?.id

    if (project) {
      localStorage.setItem(PROJECT_STORAGE_KEY, JSON.stringify(project))
    } else {
      localStorage.removeItem(PROJECT_STORAGE_KEY)
    }

    // If project actually changed, do a full page refresh to reset all state
    if (project?.id !== currentId) {
      window.location.reload()
    } else {
      setSelectedProjectState(project)
    }
  }, [])

  return (
    <ProjectContext.Provider
      value={{
        selectedProject,
        setSelectedProject,
        projectId: selectedProject?.id ?? null,
        isHydrated,
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
