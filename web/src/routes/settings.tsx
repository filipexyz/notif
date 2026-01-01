import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Copy, Eye, EyeOff } from 'lucide-react'
import { Button } from '../components/ui'
import { useApi } from '../lib/api'
import type { APIKey, CreateAPIKeyResponse } from '../lib/types'

export const Route = createFileRoute('/settings')({
  component: SettingsPage,
})

function SettingsPage() {
  const api = useApi()
  const queryClient = useQueryClient()

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [showKey, setShowKey] = useState(false)

  const { data: apiKeysResponse, isLoading, error } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => api<{ api_keys: APIKey[]; count: number }>('/api/v1/api-keys'),
  })
  const apiKeys = apiKeysResponse?.api_keys

  const createKeyMutation = useMutation({
    mutationFn: (name: string) =>
      api<CreateAPIKeyResponse>('/api/v1/api-keys', {
        method: 'POST',
        body: JSON.stringify({ name }),
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
    createKeyMutation.mutate(newKeyName || 'Untitled Key')
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

  const keys = apiKeys ?? []

  return (
    <div className="p-4">
      <h1 className="text-xl font-semibold text-neutral-900 mb-6">Settings</h1>

      {/* API Keys Section */}
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
        {!isLoading && !error && keys.length === 0 && (
          <div className="py-8 text-center text-neutral-500">
            No API keys created yet.
          </div>
        )}

        {/* Keys table */}
        {keys.length > 0 && (
          <div className="border border-neutral-200 bg-white">
            <table className="w-full">
              <thead>
                <tr className="border-b border-neutral-200 bg-neutral-50">
                  <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Name</th>
                  <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Key</th>
                  <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Created</th>
                  <th className="px-4 py-2 text-left text-sm font-medium text-neutral-700">Last Used</th>
                  <th className="px-4 py-2 text-right text-sm font-medium text-neutral-700">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100">
                {keys.map((key) => (
                  <tr key={key.id} className="hover:bg-neutral-50">
                    <td className="px-4 py-3 text-sm font-medium text-neutral-700">
                      {key.name || 'Unnamed'}
                    </td>
                    <td className="px-4 py-3 text-sm font-mono text-neutral-500">
                      {key.key_prefix}
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
      </section>

      {/* Create Key Modal */}
      {showCreateModal && (
        <>
          <div className="fixed inset-0 bg-neutral-900/20 z-40" onClick={() => setShowCreateModal(false)} />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-sm bg-white border border-neutral-200 p-6 z-50">
            <h3 className="text-lg font-medium text-neutral-900 mb-4">Create API Key</h3>
            <input
              type="text"
              placeholder="Key name (e.g., Production)"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              className="w-full px-3 py-2 text-sm border border-neutral-200 mb-4"
            />
            {createKeyMutation.error && (
              <div className="mb-4 text-sm text-error">
                {createKeyMutation.error.message}
              </div>
            )}
            <div className="flex gap-2">
              <Button onClick={handleCreateKey} disabled={createKeyMutation.isPending}>
                {createKeyMutation.isPending ? 'Creating...' : 'Create'}
              </Button>
              <Button variant="secondary" onClick={() => setShowCreateModal(false)}>
                Cancel
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
