import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Plus, FileJson } from 'lucide-react'
import { Button, Badge } from '../../components/ui'
import { useApi, useProjectReady } from '../../lib/api'
import type { Schema, SchemasResponse } from '../../lib/types'

export const Route = createFileRoute('/schemas/')({
  component: SchemasPage,
})

function SchemasPage() {
  const api = useApi()
  const projectReady = useProjectReady()

  const { data: schemasResponse, isLoading, error } = useQuery({
    queryKey: ['schemas'],
    queryFn: () => api<SchemasResponse>('/api/v1/schemas'),
    enabled: projectReady,
  })
  const schemas = schemasResponse?.schemas

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-semibold text-neutral-900">Schemas</h1>
          <p className="text-sm text-neutral-500 mt-1">
            Define JSON schemas to validate events on specific topics
          </p>
        </div>
        <Link to="/schemas/new">
          <Button size="sm">
            <Plus className="w-4 h-4 mr-1.5" />
            New Schema
          </Button>
        </Link>
      </div>

      {/* Loading state */}
      {isLoading && (
        <div className="p-8 text-center text-neutral-500">Loading schemas...</div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-8 text-center text-error">
          Failed to load schemas: {error.message}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && schemas?.length === 0 && (
        <div className="p-8 text-center border border-neutral-200 bg-white">
          <FileJson className="w-12 h-12 mx-auto text-neutral-300 mb-3" />
          <p className="text-neutral-500 mb-4">
            No schemas configured. Create your first schema to start validating events.
          </p>
          <Link to="/schemas/new">
            <Button size="sm">
              <Plus className="w-4 h-4 mr-1.5" />
              Create Schema
            </Button>
          </Link>
        </div>
      )}

      {/* Table */}
      {schemas && schemas.length > 0 && (
        <div className="border border-neutral-200 bg-white">
          <table className="w-full">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50">
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Name</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Topic Pattern</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Version</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Validation</th>
                <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Tags</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {schemas.map((schema) => (
                <SchemaRow key={schema.id} schema={schema} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function SchemaRow({ schema }: { schema: Schema }) {
  const validationMode = schema.latest_version?.validation_mode || 'none'
  const validationVariant = validationMode === 'strict' ? 'error' : validationMode === 'warn' ? 'warning' : 'default'

  return (
    <tr className="hover:bg-neutral-50">
      <td className="px-4 py-3">
        <Link
          to="/schemas/$name"
          params={{ name: schema.name }}
          className="text-sm font-medium text-neutral-900 hover:text-primary-600"
        >
          {schema.name}
        </Link>
        {schema.description && (
          <p className="text-xs text-neutral-500 mt-0.5 truncate max-w-xs">
            {schema.description}
          </p>
        )}
      </td>
      <td className="px-4 py-3">
        <code className="text-sm font-mono text-neutral-600 bg-neutral-100 px-1.5 py-0.5">
          {schema.topic_pattern}
        </code>
      </td>
      <td className="px-4 py-3">
        {schema.latest_version ? (
          <Badge variant="default">{schema.latest_version.version}</Badge>
        ) : (
          <span className="text-sm text-neutral-400">no version</span>
        )}
      </td>
      <td className="px-4 py-3">
        <Badge variant={validationVariant}>
          {validationMode}
        </Badge>
      </td>
      <td className="px-4 py-3">
        <div className="flex gap-1 flex-wrap">
          {schema.tags?.map((tag) => (
            <Badge key={tag} variant="default" className="text-xs">{tag}</Badge>
          ))}
          {(!schema.tags || schema.tags.length === 0) && (
            <span className="text-sm text-neutral-400">-</span>
          )}
        </div>
      </td>
    </tr>
  )
}
