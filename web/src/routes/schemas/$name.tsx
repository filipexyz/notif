import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Trash2, Plus, Check, X } from 'lucide-react'
import { useState } from 'react'
import { Button, Badge, Input } from '../../components/ui'
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

      {/* Schema info */}
      <div className="grid grid-cols-2 gap-4 mb-6">
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-sm font-medium text-neutral-500 mb-2">Topic Pattern</h3>
          <code className="text-sm font-mono text-neutral-900 bg-neutral-100 px-2 py-1">
            {schema.topic_pattern}
          </code>
        </div>
        <div className="border border-neutral-200 bg-white p-4">
          <h3 className="text-sm font-medium text-neutral-500 mb-2">Tags</h3>
          <div className="flex gap-1 flex-wrap">
            {schema.tags?.map((tag) => (
              <Badge key={tag} variant="default">{tag}</Badge>
            ))}
            {(!schema.tags || schema.tags.length === 0) && (
              <span className="text-sm text-neutral-400">No tags</span>
            )}
          </div>
        </div>
      </div>

      {/* Latest version info */}
      {schema.latest_version && (
        <div className="border border-neutral-200 bg-white p-4 mb-6">
          <h3 className="text-sm font-medium text-neutral-700 mb-3">Latest Version: {schema.latest_version.version}</h3>
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div>
              <span className="text-neutral-500">Validation Mode:</span>{' '}
              <Badge variant={schema.latest_version.validation_mode === 'strict' ? 'error' : 'default'}>
                {schema.latest_version.validation_mode}
              </Badge>
            </div>
            <div>
              <span className="text-neutral-500">On Invalid:</span>{' '}
              <Badge variant="default">{schema.latest_version.on_invalid}</Badge>
            </div>
            <div>
              <span className="text-neutral-500">Fingerprint:</span>{' '}
              <code className="text-xs font-mono text-neutral-600">
                {schema.latest_version.fingerprint.slice(0, 16)}...
              </code>
            </div>
          </div>

          {/* JSON Schema preview */}
          <div className="mt-4">
            <h4 className="text-sm font-medium text-neutral-500 mb-2">JSON Schema</h4>
            <pre className="text-xs font-mono bg-neutral-50 p-3 overflow-x-auto border border-neutral-200 max-h-64 overflow-y-auto">
              {JSON.stringify(schema.latest_version.schema, null, 2)}
            </pre>
          </div>
        </div>
      )}

      {/* Versions section */}
      <div className="border border-neutral-200 bg-white">
        <div className="flex items-center justify-between px-4 py-3 border-b border-neutral-200">
          <h3 className="text-sm font-medium text-neutral-700">Versions</h3>
          <Button size="sm" variant="outline" onClick={() => setShowNewVersion(true)}>
            <Plus className="w-4 h-4 mr-1.5" />
            New Version
          </Button>
        </div>

        {/* New version form */}
        {showNewVersion && (
          <NewVersionForm
            schemaName={name}
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
        <pre className="mt-3 text-xs font-mono bg-neutral-50 p-3 overflow-x-auto border border-neutral-200 max-h-48 overflow-y-auto">
          {JSON.stringify(version.schema, null, 2)}
        </pre>
      )}
    </div>
  )
}

function NewVersionForm({ schemaName, onClose }: { schemaName: string; onClose: () => void }) {
  const api = useApi()
  const queryClient = useQueryClient()

  const [version, setVersion] = useState('')
  const [schemaJson, setSchemaJson] = useState('{\n  "type": "object",\n  "properties": {}\n}')
  const [validationMode, setValidationMode] = useState<'strict' | 'warn' | 'disabled'>('strict')
  const [onInvalid, setOnInvalid] = useState<'reject' | 'log' | 'dlq'>('reject')
  const [jsonError, setJsonError] = useState<string | null>(null)

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
    setJsonError(null)

    try {
      const parsedSchema = JSON.parse(schemaJson)
      createMutation.mutate({
        version,
        schema: parsedSchema,
        validation_mode: validationMode,
        on_invalid: onInvalid,
      })
    } catch {
      setJsonError('Invalid JSON schema')
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
        <textarea
          value={schemaJson}
          onChange={(e) => setSchemaJson(e.target.value)}
          className="w-full px-3 py-2 border border-neutral-200 font-mono text-sm h-48"
          placeholder='{"type": "object", "properties": {}}'
          required
        />
        {jsonError && <p className="text-sm text-error mt-1">{jsonError}</p>}
        {createMutation.error && (
          <p className="text-sm text-error mt-1">{(createMutation.error as Error).message}</p>
        )}
      </div>

      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={createMutation.isPending}>
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
