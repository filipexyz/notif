import { createFileRoute, useSearch } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Copy, Eye, EyeOff, Pencil, FolderOpen } from 'lucide-react'
import { Button } from '../components/ui'
import { useApi } from '../lib/api'
import { useProject } from '../lib/project-context'
import type { APIKey, CreateAPIKeyResponse, Project, ProjectsResponse } from '../lib/types'

type SearchParams = {
  tab?: 'api-keys' | 'projects'
}

export const Route = createFileRoute('/settings')({
  component: SettingsPage,
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    tab: (search.tab as SearchParams['tab']) || 'api-keys',
  }),
})

function SettingsPage() {
  const search = useSearch({ from: '/settings' })
  const [activeTab, setActiveTab] = useState<'api-keys' | 'projects'>(search.tab || 'api-keys')

  // Sync tab with URL search params
  useEffect(() => {
    if (search.tab) {
      setActiveTab(search.tab)
    }
  }, [search.tab])

  return (
    <div className="p-4">
      <h1 className="text-xl font-semibold text-neutral-900 mb-6">Settings</h1>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 border-b border-neutral-200">
        <button
          onClick={() => setActiveTab('api-keys')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'api-keys'
              ? 'text-primary-600 border-primary-500'
              : 'text-neutral-600 border-transparent hover:text-neutral-900'
          }`}
        >
          API Keys
        </button>
        <button
          onClick={() => setActiveTab('projects')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'projects'
              ? 'text-primary-600 border-primary-500'
              : 'text-neutral-600 border-transparent hover:text-neutral-900'
          }`}
        >
          Projects
        </button>
      </div>

      {activeTab === 'api-keys' && <APIKeysSection />}
      {activeTab === 'projects' && <ProjectsSection />}
    </div>
  )
}

function APIKeysSection() {
  const api = useApi()
  const queryClient = useQueryClient()

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [showKey, setShowKey] = useState(false)

  const { data: apiKeysResponse, isLoading, error } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => api<{ api_keys: APIKey[]; count: number }>('/api/v1/api-keys'),
  })

  const { data: projectsData } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api<ProjectsResponse>('/api/v1/projects'),
  })

  const apiKeys = apiKeysResponse?.api_keys ?? []
  const projects = projectsData?.projects ?? []

  // Set default project
  useEffect(() => {
    if (!selectedProjectId && projects.length > 0) {
      setSelectedProjectId(projects[0].id)
    }
  }, [projects, selectedProjectId])

  const createKeyMutation = useMutation({
    mutationFn: ({ name, project_id }: { name: string; project_id: string }) =>
      api<CreateAPIKeyResponse>('/api/v1/api-keys', {
        method: 'POST',
        body: JSON.stringify({ name, project_id }),
      }),
    onSuccess: (data) => {
      setCreatedKey(data.full_key)
      setNewKeyName('')
      setShowCreateModal(false)
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const deleteKeyMutation = useMutation({
    mutationFn: (id: string) =>
      api(`/api/v1/api-keys/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const handleCreateKey = () => {
    if (!selectedProjectId) {
      alert('Please select a project')
      return
    }
    createKeyMutation.mutate({
      name: newKeyName || 'Untitled Key',
      project_id: selectedProjectId,
    })
  }

  const handleCopyKey = () => {
    if (createdKey) {
      navigator.clipboard.writeText(createdKey)
    }
  }

  const handleDeleteKey = (id: string) => {
    if (confirm('Are you sure you want to revoke this API key?')) {
      deleteKeyMutation.mutate(id)
    }
  }

  return (
    <section>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-medium text-neutral-900">API Keys</h2>
        <Button size="sm" onClick={() => setShowCreateModal(true)}>
          <Plus className="w-4 h-4 mr-1.5" />
          Create Key
        </Button>
      </div>

      {/* Created key display */}
      {createdKey && (
        <div className="mb-4 p-4 bg-success/10 border border-success">
          <p className="text-sm text-neutral-700 mb-2">
            Your new API key has been created. Copy it now - you won't be able to see it again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 px-3 py-2 bg-white border border-neutral-200 font-mono text-sm">
              {showKey ? createdKey : '•'.repeat(24)}
            </code>
            <button
              onClick={() => setShowKey(!showKey)}
              className="p-2 text-neutral-500 hover:text-neutral-700 hover:bg-neutral-100"
            >
              {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
            <button
              onClick={handleCopyKey}
              className="p-2 text-neutral-500 hover:text-neutral-700 hover:bg-neutral-100"
            >
              <Copy className="w-4 h-4" />
            </button>
          </div>
          <button
            onClick={() => setCreatedKey(null)}
            className="mt-2 text-sm text-neutral-500 hover:text-neutral-700"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Loading state */}
      {isLoading && (
        <div className="py-8 text-center text-neutral-500">Loading API keys...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="py-8 text-center text-error">
          Failed to load API keys: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && apiKeys.length === 0 && (
        <div className="py-8 text-center text-neutral-500">
          No API keys created yet.
        </div>
      )}

      {/* Keys table */}
      {apiKeys.length > 0 && (
        <div className="border border-neutral-200 bg-white">
          <table className="w-full">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Name</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Key</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Project</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Created</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Last Used</th>
                <th className="px-4 py-2 text-right text-sm font-medium text-neutral-700">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {apiKeys.map((key) => (
                <tr key={key.id} className="hover:bg-neutral-50">
                  <td className="px-4 py-3 text-sm font-medium text-neutral-700">
                    {key.name || 'Unnamed'}
                  </td>
                  <td className="px-4 py-3 text-sm font-mono text-neutral-500">
                    {key.key_prefix}
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-500">
                    {key.project_name || key.project_id}
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-500">
                    {new Date(key.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-sm text-neutral-500">
                    {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => handleDeleteKey(key.id)}
                      disabled={deleteKeyMutation.isPending}
                      className="p-1.5 text-neutral-400 hover:text-error hover:bg-neutral-100 disabled:opacity-50"
                      title="Revoke"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Key Modal */}
      {showCreateModal && (
        <>
          <div className="fixed inset-0 bg-neutral-900/20 z-40" onClick={() => setShowCreateModal(false)} />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-sm bg-white border border-neutral-200 p-6 z-50">
            <h3 className="text-lg font-medium text-neutral-900 mb-4">Create API Key</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Name</label>
                <input
                  type="text"
                  placeholder="Key name (e.g., Production)"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Project</label>
                <select
                  value={selectedProjectId}
                  onChange={(e) => setSelectedProjectId(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200 bg-white"
                >
                  {projects.length === 0 ? (
                    <option value="">No projects available</option>
                  ) : (
                    projects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.name}
                      </option>
                    ))
                  )}
                </select>
                <p className="mt-1 text-xs text-neutral-500">
                  The API key will only have access to this project's data.
                </p>
              </div>
            </div>
            {createKeyMutation.error && (
              <div className="mt-4 text-sm text-error">
                {createKeyMutation.error.message}
              </div>
            )}
            <div className="flex gap-2 mt-6">
              <Button onClick={handleCreateKey} disabled={createKeyMutation.isPending || !selectedProjectId}>
                {createKeyMutation.isPending ? 'Creating...' : 'Create'}
              </Button>
              <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
                Cancel
              </Button>
            </div>
          </div>
        </>
      )}
    </section>
  )
}

function ProjectsSection() {
  const api = useApi()
  const queryClient = useQueryClient()
  const { selectedProject, setSelectedProject } = useProject()

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editingProject, setEditingProject] = useState<Project | null>(null)
  const [projectName, setProjectName] = useState('')
  const [projectSlug, setProjectSlug] = useState('')

  const { data: projectsData, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api<ProjectsResponse>('/api/v1/projects'),
  })

  const projects = projectsData?.projects ?? []

  const createProjectMutation = useMutation({
    mutationFn: ({ name, slug }: { name: string; slug?: string }) =>
      api<Project>('/api/v1/projects', {
        method: 'POST',
        body: JSON.stringify({ name, slug: slug || undefined }),
      }),
    onSuccess: (data) => {
      setProjectName('')
      setProjectSlug('')
      setShowCreateModal(false)
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      // Select the newly created project
      setSelectedProject(data)
    },
  })

  const updateProjectMutation = useMutation({
    mutationFn: ({ id, name, slug }: { id: string; name?: string; slug?: string }) =>
      api<Project>(`/api/v1/projects/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ name, slug }),
      }),
    onSuccess: (data) => {
      setShowEditModal(false)
      setEditingProject(null)
      setProjectName('')
      setProjectSlug('')
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      // Update selected project if we just edited it
      if (selectedProject?.id === data.id) {
        setSelectedProject(data)
      }
    },
  })

  const deleteProjectMutation = useMutation({
    mutationFn: (id: string) =>
      api(`/api/v1/projects/${id}`, { method: 'DELETE' }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      // Deselect if we deleted the selected project
      if (selectedProject?.id === id) {
        setSelectedProject(null)
      }
    },
  })

  const handleCreateProject = () => {
    if (!projectName.trim()) {
      alert('Please enter a project name')
      return
    }
    createProjectMutation.mutate({
      name: projectName.trim(),
      slug: projectSlug.trim() || undefined,
    })
  }

  const handleEditProject = () => {
    if (!editingProject || !projectName.trim()) return
    updateProjectMutation.mutate({
      id: editingProject.id,
      name: projectName.trim(),
      slug: projectSlug.trim() || undefined,
    })
  }

  const handleDeleteProject = (project: Project) => {
    if (project.slug === 'default') {
      alert('Cannot delete the default project')
      return
    }
    if (confirm(`Are you sure you want to delete "${project.name}"?`)) {
      deleteProjectMutation.mutate(project.id)
    }
  }

  const openEditModal = (project: Project) => {
    setEditingProject(project)
    setProjectName(project.name)
    setProjectSlug(project.slug)
    setShowEditModal(true)
  }

  return (
    <section>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-medium text-neutral-900">Projects</h2>
        <Button size="sm" onClick={() => setShowCreateModal(true)}>
          <Plus className="w-4 h-4 mr-1.5" />
          Create Project
        </Button>
      </div>

      <p className="text-sm text-neutral-500 mb-4">
        Projects let you organize your events and API keys. Each API key is scoped to a single project.
      </p>

      {/* Loading state */}
      {isLoading && (
        <div className="py-8 text-center text-neutral-500">Loading projects...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="py-8 text-center text-error">
          Failed to load projects: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && projects.length === 0 && (
        <div className="py-8 text-center text-neutral-500">
          No projects yet.
        </div>
      )}

      {/* Projects grid */}
      {projects.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((project) => (
            <div
              key={project.id}
              className={`p-4 border bg-white ${
                selectedProject?.id === project.id
                  ? 'border-primary-500'
                  : 'border-neutral-200'
              }`}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-2">
                  <FolderOpen className="w-5 h-5 text-neutral-400" />
                  <div>
                    <h3 className="font-medium text-neutral-900">{project.name}</h3>
                    <p className="text-xs text-neutral-500">{project.slug}</p>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => openEditModal(project)}
                    className="p-1.5 text-neutral-400 hover:text-neutral-700 hover:bg-neutral-100"
                    title="Edit"
                  >
                    <Pencil className="w-4 h-4" />
                  </button>
                  {project.slug !== 'default' && (
                    <button
                      onClick={() => handleDeleteProject(project)}
                      disabled={deleteProjectMutation.isPending}
                      className="p-1.5 text-neutral-400 hover:text-error hover:bg-neutral-100 disabled:opacity-50"
                      title="Delete"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </div>
              <div className="mt-3 text-xs text-neutral-500">
                Created {new Date(project.created_at).toLocaleDateString()}
              </div>
              {selectedProject?.id === project.id && (
                <div className="mt-2 text-xs text-primary-600 font-medium">
                  Currently selected
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Create Project Modal */}
      {showCreateModal && (
        <>
          <div className="fixed inset-0 bg-neutral-900/20 z-40" onClick={() => setShowCreateModal(false)} />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-sm bg-white border border-neutral-200 p-6 z-50">
            <h3 className="text-lg font-medium text-neutral-900 mb-4">Create Project</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Name</label>
                <input
                  type="text"
                  placeholder="Project name (e.g., Production)"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Slug (optional)</label>
                <input
                  type="text"
                  placeholder="Auto-generated from name"
                  value={projectSlug}
                  onChange={(e) => setProjectSlug(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200"
                />
                <p className="mt-1 text-xs text-neutral-500">
                  URL-safe identifier. Leave blank to auto-generate.
                </p>
              </div>
            </div>
            {createProjectMutation.error && (
              <div className="mt-4 text-sm text-error">
                {createProjectMutation.error.message}
              </div>
            )}
            <div className="flex gap-2 mt-6">
              <Button onClick={handleCreateProject} disabled={createProjectMutation.isPending}>
                {createProjectMutation.isPending ? 'Creating...' : 'Create'}
              </Button>
              <Button variant="secondary" onClick={() => { setShowCreateModal(false); setProjectName(''); setProjectSlug(''); }}>
                Cancel
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Edit Project Modal */}
      {showEditModal && editingProject && (
        <>
          <div className="fixed inset-0 bg-neutral-900/20 z-40" onClick={() => setShowEditModal(false)} />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-sm bg-white border border-neutral-200 p-6 z-50">
            <h3 className="text-lg font-medium text-neutral-900 mb-4">Edit Project</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Name</label>
                <input
                  type="text"
                  placeholder="Project name"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">Slug</label>
                <input
                  type="text"
                  placeholder="Slug"
                  value={projectSlug}
                  onChange={(e) => setProjectSlug(e.target.value)}
                  className="w-full px-3 py-2 text-sm border border-neutral-200"
                  disabled={editingProject.slug === 'default'}
                />
                {editingProject.slug === 'default' && (
                  <p className="mt-1 text-xs text-neutral-500">
                    Default project slug cannot be changed.
                  </p>
                )}
              </div>
            </div>
            {updateProjectMutation.error && (
              <div className="mt-4 text-sm text-error">
                {updateProjectMutation.error.message}
              </div>
            )}
            <div className="flex gap-2 mt-6">
              <Button onClick={handleEditProject} disabled={updateProjectMutation.isPending}>
                {updateProjectMutation.isPending ? 'Saving...' : 'Save'}
              </Button>
              <Button variant="secondary" onClick={() => { setShowEditModal(false); setEditingProject(null); setProjectName(''); setProjectSlug(''); }}>
                Cancel
              </Button>
            </div>
          </div>
        </>
      )}
    </section>
  )
}
