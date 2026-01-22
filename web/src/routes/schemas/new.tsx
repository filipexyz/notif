import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Check } from 'lucide-react'
import { useState } from 'react'
import { Button, Input } from '../../components/ui'
import { SchemaEditor } from '../../components/schema'
import { useApi } from '../../lib/api'
import type { CreateSchemaRequest, Schema } from '../../lib/types'

export const Route = createFileRoute('/schemas/new')({
  component: NewSchemaPage,
})

function NewSchemaPage() {
  const navigate = useNavigate()
  const api = useApi()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [topicPattern, setTopicPattern] = useState('')
  const [description, setDescription] = useState('')
  const [tags, setTags] = useState('')

  // Also create first version
  const [createVersion, setCreateVersion] = useState(true)
  const [version, setVersion] = useState('1.0.0')
  const [schemaJson, setSchemaJson] = useState('{\n  "type": "object",\n  "required": [],\n  "properties": {\n    \n  }\n}')
  const [validationMode, setValidationMode] = useState<'strict' | 'warn' | 'disabled'>('strict')
  const [onInvalid, setOnInvalid] = useState<'reject' | 'log' | 'dlq'>('reject')
  const [isValidJson, setIsValidJson] = useState(true)

  const createSchemaMutation = useMutation({
    mutationFn: async (data: CreateSchemaRequest) => {
      // Create schema first
      const schema = await api<Schema>('/api/v1/schemas', {
        method: 'POST',
        body: JSON.stringify(data),
      })

      // Create version if requested
      if (createVersion && version && schemaJson) {
        const parsedSchema = JSON.parse(schemaJson)
        await api(`/api/v1/schemas/${schema.name}/versions`, {
          method: 'POST',
          body: JSON.stringify({
            version,
            schema: parsedSchema,
            validation_mode: validationMode,
            on_invalid: onInvalid,
          }),
        })
      }

      return schema
    },
    onSuccess: (schema) => {
      queryClient.invalidateQueries({ queryKey: ['schemas'] })
      navigate({ to: '/schemas/$name', params: { name: schema.name } })
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    // Validate JSON if creating version
    if (createVersion && !isValidJson) {
      return
    }

    const tagsArray = tags
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0)

    createSchemaMutation.mutate({
      name,
      topic_pattern: topicPattern,
      description: description || undefined,
      tags: tagsArray.length > 0 ? tagsArray : undefined,
    })
  }

  return (
    <div className="p-4">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <Link to="/schemas" className="text-neutral-400 hover:text-neutral-600">
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <h1 className="text-xl font-semibold text-neutral-900">New Schema</h1>
      </div>

      <form onSubmit={handleSubmit} className="max-w-3xl">
        {/* Schema basics */}
        <div className="border border-neutral-200 bg-white p-4 mb-4">
          <h2 className="text-sm font-medium text-neutral-700 mb-4">Schema Details</h2>

          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">
                  Name <span className="text-error">*</span>
                </label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="order-placed"
                  required
                />
                <p className="text-xs text-neutral-500 mt-1">
                  Unique identifier (lowercase, hyphens allowed)
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">
                  Topic Pattern <span className="text-error">*</span>
                </label>
                <Input
                  value={topicPattern}
                  onChange={(e) => setTopicPattern(e.target.value)}
                  placeholder="orders.placed or orders.*"
                  required
                />
                <p className="text-xs text-neutral-500 mt-1">
                  Use * for single level, {'>'} for multi-level wildcard
                </p>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-neutral-700 mb-1">Description</label>
              <Input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Schema for order placement events"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-neutral-700 mb-1">Tags</label>
              <Input
                value={tags}
                onChange={(e) => setTags(e.target.value)}
                placeholder="orders, commerce, critical"
              />
              <p className="text-xs text-neutral-500 mt-1">Comma-separated tags for organization</p>
            </div>
          </div>
        </div>

        {/* Version section */}
        <div className="border border-neutral-200 bg-white p-4 mb-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-medium text-neutral-700">Initial Version</h2>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={createVersion}
                onChange={(e) => setCreateVersion(e.target.checked)}
                className="w-4 h-4"
              />
              Create version now
            </label>
          </div>

          {createVersion && (
            <div className="space-y-4">
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="block text-sm font-medium text-neutral-700 mb-1">Version</label>
                  <Input
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    placeholder="1.0.0"
                    required={createVersion}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-neutral-700 mb-1">Validation Mode</label>
                  <select
                    value={validationMode}
                    onChange={(e) => setValidationMode(e.target.value as typeof validationMode)}
                    className="w-full px-3 py-2 border border-neutral-200 text-sm"
                  >
                    <option value="strict">Strict - Enforce</option>
                    <option value="warn">Warn - Log only</option>
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

              <div>
                <label className="block text-sm font-medium text-neutral-700 mb-1">JSON Schema</label>
                <SchemaEditor
                  value={schemaJson}
                  onChange={setSchemaJson}
                  onValidChange={setIsValidJson}
                />
              </div>
            </div>
          )}
        </div>

        {/* Error */}
        {createSchemaMutation.error && (
          <div className="p-3 mb-4 bg-red-50 border border-error text-sm text-error">
            {(createSchemaMutation.error as Error).message}
          </div>
        )}

        {/* Submit */}
        <div className="flex gap-2">
          <Button
            type="submit"
            disabled={createSchemaMutation.isPending || (createVersion && !isValidJson)}
          >
            <Check className="w-4 h-4 mr-1.5" />
            {createSchemaMutation.isPending ? 'Creating...' : 'Create Schema'}
          </Button>
          <Link to="/schemas">
            <Button variant="outline" type="button">Cancel</Button>
          </Link>
        </div>
      </form>
    </div>
  )
}
