import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Trash2, Plus, Check, X, Code, Eye, Pencil, Save } from 'lucide-react'
import { useState } from 'react'
import { Button, Badge, Input } from '../../components/ui'
import { SchemaVisualizer, SchemaEditor } from '../../components/schema'
import { useApi, useProjectReady } from '../../lib/api'
import type { Schema, SchemaVersion, SchemaVersionsResponse, CreateSchemaVersionRequest } from '../../lib/types'

export const Route = createFileRoute('/schemas/$name')({
  component: SchemaDetailPage,
})

function SchemaDetailPage() {
  const { name } = Route.useParams()
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()
  const projectReady = useProjectReady()

  const [showNewVersion, setShowNewVersion] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [viewMode, setViewMode] = useState<'visual' | 'json'>('visual')
  const [editMode, setEditMode] = useState(false)
  const [editedSchema, setEditedSchema] = useState('')
  const [isValidJson, setIsValidJson] = useState(true)

  // Fetch schema
  const { data: schema, isLoading, error } = useQuery({
    queryKey: ['schema', name],
    queryFn: () => api<Schema>(`/api/v1/schemas/${name}`),
    enabled: projectReady,
  })

  // Fetch versions
  const { data: versionsResponse } = useQuery({
    queryKey: ['schema-versions', name],
    queryFn: () => api<SchemaVersionsResponse>(`/api/v1/schemas/${name}/versions`),
    enabled: projectReady && !!schema,
  })
  const versions = versionsResponse?.versions

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: () => api(`/api/v1/schemas/${name}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schemas'] })
      navigate({ to: '/schemas' })
    },
  })

  // Update schema mutation (creates new version)
  const updateMutation = useMutation({
    mutationFn: (data: CreateSchemaVersionRequest) =>
      api(`/api/v1/schemas/${name}/versions`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schema', name] })
      queryClient.invalidateQueries({ queryKey: ['schema-versions', name] })
      queryClient.invalidateQueries({ queryKey: ['schemas'] })
      setEditMode(false)
    },
  })

  const handleStartEdit = () => {
    if (schema?.latest_version) {
      setEditedSchema(JSON.stringify(schema.latest_version.schema, null, 2))
      setEditMode(true)
    }
  }

  const handleSaveEdit = () => {
    if (!isValidJson || !schema?.latest_version) return

    try {
      const parsedSchema = JSON.parse(editedSchema)
      const currentVersion = schema.latest_version.version
      const newVersion = bumpPatchVersion(currentVersion)

      updateMutation.mutate({
        version: newVersion,
        schema: parsedSchema,
        validation_mode: schema.latest_version.validation_mode,
        on_invalid: schema.latest_version.on_invalid,
      })
    } catch {
      // Invalid JSON, shouldn't happen if isValidJson is true
    }
  }

  const handleCancelEdit = () => {
    setEditMode(false)
    setEditedSchema('')
  }

  if (isLoading) {
    return (
      <div className="p-4">
        <div className="p-8 text-center text-neutral-500">Loading schema...</div>
      </div>
    )
  }

  if (error || !schema) {
    return (
      <div className="p-4">
        <div className="p-8 text-center text-error">
          Failed to load schema: {error?.message || 'Not found'}
        </div>
      </div>
    )
  }

  const currentSchema = schema.latest_version?.schema

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Link to="/schemas" className="text-neutral-400 hover:text-neutral-600">
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-xl font-semibold text-neutral-900">{schema.name}</h1>
            {schema.description && (
              <p className="text-sm text-neutral-500 mt-0.5">{schema.description}</p>
            )}
          </div>
        </div>
        <div className="flex gap-2">
          {!editMode && schema.latest_version && (
            <Button variant="outline" size="sm" onClick={handleStartEdit}>
              <Pencil className="w-4 h-4 mr-1.5" />
              Edit Schema
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowDeleteConfirm(true)}
            className="text-error hover:bg-error hover:text-white"
          >
            <Trash2 className="w-4 h-4 mr-1.5" />
            Delete
          </Button>
        </div>
      </div>

      {/* Delete confirmation */}
      {showDeleteConfirm && (
        <div className="mb-4 p-4 border border-error bg-red-50">
          <p className="text-sm text-error mb-3">
            Are you sure you want to delete this schema? This will also delete all versions.
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowDeleteConfirm(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
              className="bg-error text-white hover:bg-red-700"
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Yes, Delete'}
            </Button>
          </div>
        </div>
      )}

      {/* Schema info cards */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-xs font-medium text-neutral-500 mb-1">Topic Pattern</h3>
          <code className="text-sm font-mono text-neutral-900 bg-neutral-100 px-2 py-1">
            {schema.topic_pattern}
          </code>
        </div>
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-xs font-medium text-neutral-500 mb-1">Version</h3>
          <Badge variant="info" className="text-sm">
            {schema.latest_version?.version || 'No version'}
          </Badge>
        </div>
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-xs font-medium text-neutral-500 mb-1">Validation</h3>
          <Badge variant={schema.latest_version?.validation_mode === 'strict' ? 'error' : 'default'}>
            {schema.latest_version?.validation_mode || 'none'}
          </Badge>
        </div>
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-xs font-medium text-neutral-500 mb-1">Tags</h3>
          <div className="flex gap-1 flex-wrap">
            {schema.tags?.map((tag) => (
              <Badge key={tag} variant="default" className="text-xs">{tag}</Badge>
            ))}
            {(!schema.tags || schema.tags.length === 0) && (
              <span className="text-sm text-neutral-400">-</span>
            )}
          </div>
        </div>
      </div>

      {/* Schema viewer/editor */}
      {currentSchema && (
        <div className="border border-neutral-200 bg-white mb-6">
          {/* Toolbar */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-neutral-200 bg-neutral-50">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-medium text-neutral-700">JSON Schema</h3>
              {editMode && (
                <Badge variant="warning" className="text-xs">Editing</Badge>
              )}
            </div>
            <div className="flex items-center gap-2">
              {editMode ? (
                <>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleCancelEdit}
                    disabled={updateMutation.isPending}
                  >
                    <X className="w-4 h-4 mr-1.5" />
                    Cancel
                  </Button>
                  <Button
                    size="sm"
                    onClick={handleSaveEdit}
                    disabled={!isValidJson || updateMutation.isPending}
                  >
                    <Save className="w-4 h-4 mr-1.5" />
                    {updateMutation.isPending ? 'Saving...' : 'Save as New Version'}
                  </Button>
                </>
              ) : (
                <div className="flex border border-neutral-200 bg-white">
                  <button
                    onClick={() => setViewMode('visual')}
                    className={`px-3 py-1.5 text-sm flex items-center gap-1.5 ${
                      viewMode === 'visual'
                        ? 'bg-primary-500 text-white'
                        : 'text-neutral-600 hover:bg-neutral-100'
                    }`}
                  >
                    <Eye className="w-4 h-4" />
                    Visual
                  </button>
                  <button
                    onClick={() => setViewMode('json')}
                    className={`px-3 py-1.5 text-sm flex items-center gap-1.5 ${
                      viewMode === 'json'
                        ? 'bg-primary-500 text-white'
                        : 'text-neutral-600 hover:bg-neutral-100'
                    }`}
                  >
                    <Code className="w-4 h-4" />
                    JSON
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Content */}
          {editMode ? (
            <SchemaEditor
              value={editedSchema}
              onChange={setEditedSchema}
              onValidChange={setIsValidJson}
            />
          ) : viewMode === 'visual' ? (
            <div className="p-4 max-h-96 overflow-auto">
              <SchemaVisualizer schema={currentSchema} />
            </div>
          ) : (
            <pre className="p-4 text-sm font-mono bg-neutral-50 overflow-auto max-h-96">
              {JSON.stringify(currentSchema, null, 2)}
            </pre>
          )}

          {updateMutation.error && (
            <div className="px-4 py-2 border-t border-error bg-red-50 text-sm text-error">
              {(updateMutation.error as Error).message}
            </div>
          )}
        </div>
      )}

      {/* Versions section */}
      <div className="border border-neutral-200 bg-white">
        <div className="flex items-center justify-between px-4 py-3 border-b border-neutral-200">
          <h3 className="text-sm font-medium text-neutral-700">
            Version History
            {versions && <span className="text-neutral-400 ml-2">({versions.length})</span>}
          </h3>
          <Button size="sm" variant="outline" onClick={() => setShowNewVersion(true)}>
            <Plus className="w-4 h-4 mr-1.5" />
            New Version
          </Button>
        </div>

        {/* New version form */}
        {showNewVersion && (
          <NewVersionForm
            schemaName={name}
            currentSchema={currentSchema}
            onClose={() => setShowNewVersion(false)}
          />
        )}

        {/* Versions list */}
        {versions && versions.length > 0 ? (
          <div className="divide-y divide-neutral-100">
            {versions.map((version) => (
              <VersionRow key={version.id} version={version} />
            ))}
          </div>
        ) : (
          <div className="p-4 text-center text-sm text-neutral-500">
            No versions yet. Create your first version to start validating events.
          </div>
        )}
      </div>
    </div>
  )
}

function VersionRow({ version }: { version: SchemaVersion }) {
  const [expanded, setExpanded] = useState(false)
  const [viewMode, setViewMode] = useState<'visual' | 'json'>('visual')

  return (
    <div className="px-4 py-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="font-medium text-sm text-neutral-900">{version.version}</span>
          {version.is_latest && <Badge variant="success">latest</Badge>}
          <Badge variant={version.validation_mode === 'strict' ? 'error' : 'default'}>
            {version.validation_mode}
          </Badge>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-neutral-500">
            {new Date(version.created_at).toLocaleDateString()}
          </span>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-xs text-primary-600 hover:text-primary-700"
          >
            {expanded ? 'Hide' : 'Show'} schema
          </button>
        </div>
      </div>
      {expanded && (
        <div className="mt-3 border border-neutral-200">
          {/* View toggle */}
          <div className="flex border-b border-neutral-200 bg-neutral-50">
            <button
              onClick={() => setViewMode('visual')}
              className={`px-3 py-1.5 text-xs flex items-center gap-1 ${
                viewMode === 'visual'
                  ? 'bg-white border-b-2 border-primary-500 text-primary-600'
                  : 'text-neutral-500 hover:text-neutral-700'
              }`}
            >
              <Eye className="w-3 h-3" />
              Visual
            </button>
            <button
              onClick={() => setViewMode('json')}
              className={`px-3 py-1.5 text-xs flex items-center gap-1 ${
                viewMode === 'json'
                  ? 'bg-white border-b-2 border-primary-500 text-primary-600'
                  : 'text-neutral-500 hover:text-neutral-700'
              }`}
            >
              <Code className="w-3 h-3" />
              JSON
            </button>
          </div>
          {viewMode === 'visual' ? (
            <div className="p-3 max-h-64 overflow-auto bg-white">
              <SchemaVisualizer schema={version.schema} />
            </div>
          ) : (
            <pre className="p-3 text-xs font-mono bg-neutral-50 overflow-auto max-h-64">
              {JSON.stringify(version.schema, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  )
}

function NewVersionForm({
  schemaName,
  currentSchema,
  onClose,
}: {
  schemaName: string
  currentSchema?: Record<string, unknown>
  onClose: () => void
}) {
  const api = useApi()
  const queryClient = useQueryClient()

  const [version, setVersion] = useState('')
  const [schemaJson, setSchemaJson] = useState(
    currentSchema
      ? JSON.stringify(currentSchema, null, 2)
      : '{\n  "type": "object",\n  "properties": {}\n}'
  )
  const [validationMode, setValidationMode] = useState<'strict' | 'warn' | 'disabled'>('strict')
  const [onInvalid, setOnInvalid] = useState<'reject' | 'log' | 'dlq'>('reject')
  const [isValidJson, setIsValidJson] = useState(true)

  const createMutation = useMutation({
    mutationFn: (data: CreateSchemaVersionRequest) =>
      api(`/api/v1/schemas/${schemaName}/versions`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schema', schemaName] })
      queryClient.invalidateQueries({ queryKey: ['schema-versions', schemaName] })
      queryClient.invalidateQueries({ queryKey: ['schemas'] })
      onClose()
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!isValidJson) return

    try {
      const parsedSchema = JSON.parse(schemaJson)
      createMutation.mutate({
        version,
        schema: parsedSchema,
        validation_mode: validationMode,
        on_invalid: onInvalid,
      })
    } catch {
      // Invalid JSON
    }
  }

  return (
    <form onSubmit={handleSubmit} className="p-4 border-b border-neutral-200 bg-neutral-50">
      <div className="grid grid-cols-3 gap-4 mb-4">
        <div>
          <label className="block text-sm font-medium text-neutral-700 mb-1">Version</label>
          <Input
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder="1.0.0"
            required
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-700 mb-1">Validation Mode</label>
          <select
            value={validationMode}
            onChange={(e) => setValidationMode(e.target.value as typeof validationMode)}
            className="w-full px-3 py-2 border border-neutral-200 text-sm"
          >
            <option value="strict">Strict</option>
            <option value="warn">Warn</option>
            <option value="disabled">Disabled</option>
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-neutral-700 mb-1">On Invalid</label>
          <select
            value={onInvalid}
            onChange={(e) => setOnInvalid(e.target.value as typeof onInvalid)}
            className="w-full px-3 py-2 border border-neutral-200 text-sm"
          >
            <option value="reject">Reject</option>
            <option value="log">Log</option>
            <option value="dlq">DLQ</option>
          </select>
        </div>
      </div>

      <div className="mb-4">
        <label className="block text-sm font-medium text-neutral-700 mb-1">JSON Schema</label>
        <SchemaEditor
          value={schemaJson}
          onChange={setSchemaJson}
          onValidChange={setIsValidJson}
        />
        {createMutation.error && (
          <p className="text-sm text-error mt-1">{(createMutation.error as Error).message}</p>
        )}
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={!isValidJson || createMutation.isPending}>
          <Check className="w-4 h-4 mr-1.5" />
          {createMutation.isPending ? 'Creating...' : 'Create Version'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onClose}>
          <X className="w-4 h-4 mr-1.5" />
          Cancel
        </Button>
      </div>
    </form>
  )
}

function bumpPatchVersion(v: string): string {
  const parts = v.split('.')
  if (parts.length !== 3) {
    return v + '.1'
  }
  const patch = parseInt(parts[2], 10) || 0
  return `${parts[0]}.${parts[1]}.${patch + 1}`
}
