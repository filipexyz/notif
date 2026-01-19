import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import type { Project } from './types'

const PROJECT_STORAGE_KEY = 'notif_selected_project'

type ProjectContextType = {
  selectedProject: Project | null
  setSelectedProject: (project: Project | null) => void
  projectId: string | null
}

const ProjectContext = createContext<ProjectContextType | null>(null)

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [selectedProject, setSelectedProjectState] = useState<Project | null>(null)

  // Load from localStorage on mount
  useEffect(() => {
    const stored = localStorage.getItem(PROJECT_STORAGE_KEY)
    if (stored) {
      try {
        setSelectedProjectState(JSON.parse(stored))
      } catch {
        localStorage.removeItem(PROJECT_STORAGE_KEY)
      }
    }
  }, [])

  const setSelectedProject = (project: Project | null) => {
    setSelectedProjectState(project)
    if (project) {
      localStorage.setItem(PROJECT_STORAGE_KEY, JSON.stringify(project))
    } else {
      localStorage.removeItem(PROJECT_STORAGE_KEY)
    }
  }

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
