import { useState, useRef, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, Plus, FolderOpen, Check } from 'lucide-react'
import { Link } from '@tanstack/react-router'

import { useApi } from '../../lib/api'
import { useProject } from '../../lib/project-context'
import type { Project, ProjectsResponse } from '../../lib/types'

export function ProjectSelector() {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const api = useApi()
  const { selectedProject, setSelectedProject } = useProject()

  // Fetch projects
  const { data, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api<ProjectsResponse>('/api/v1/projects'),
  })

  const projects = data?.projects ?? []

  // Auto-select first project if none selected
  useEffect(() => {
    if (!selectedProject && projects.length > 0) {
      setSelectedProject(projects[0])
    }
  }, [projects, selectedProject, setSelectedProject])

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Close on Escape
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setIsOpen(false)
    }
    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [])

  const handleSelect = (project: Project) => {
    setSelectedProject(project)
    setIsOpen(false)
  }

  if (isLoading) {
    return (
      <div className="px-3 py-1.5 text-sm text-neutral-400">
        Loading...
      </div>
    )
  }

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
      >
        <FolderOpen className="w-4 h-4 text-neutral-400" />
        <span className="max-w-[140px] truncate">
          {selectedProject?.name ?? 'Select project'}
        </span>
        <ChevronDown className={`w-4 h-4 text-neutral-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 w-56 bg-white border border-neutral-200 shadow-lg z-50">
          <div className="py-1">
            {projects.length === 0 ? (
              <div className="px-3 py-2 text-sm text-neutral-500">
                No projects yet
              </div>
            ) : (
              projects.map((project) => (
                <button
                  key={project.id}
                  onClick={() => handleSelect(project)}
                  className="w-full flex items-center gap-2 px-3 py-2 text-sm text-left hover:bg-neutral-50 transition-colors"
                >
                  <span className="flex-1 truncate">{project.name}</span>
                  {selectedProject?.id === project.id && (
                    <Check className="w-4 h-4 text-primary-500" />
                  )}
                </button>
              ))
            )}
          </div>

          <div className="border-t border-neutral-200">
            <Link
              to="/settings"
              search={{ tab: 'projects' }}
              onClick={() => setIsOpen(false)}
              className="flex items-center gap-2 px-3 py-2 text-sm text-neutral-600 hover:bg-neutral-50 transition-colors"
            >
              <Plus className="w-4 h-4" />
              Manage projects
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
