import { useState, useEffect, useRef, useCallback } from 'react'
import { Check, AlertCircle, Wand2 } from 'lucide-react'

type SchemaEditorProps = {
  value: string
  onChange: (value: string) => void
  onValidChange?: (isValid: boolean) => void
  className?: string
  readOnly?: boolean
}

export function SchemaEditor({
  value,
  onChange,
  onValidChange,
  className = '',
  readOnly = false,
}: SchemaEditorProps) {
  const [error, setError] = useState<string | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const lineNumbersRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    try {
      if (value.trim()) {
        JSON.parse(value)
        setError(null)
        onValidChange?.(true)
      } else {
        setError(null)
        onValidChange?.(false)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Invalid JSON')
      onValidChange?.(false)
    }
  }, [value, onValidChange])

  const handleScroll = useCallback(() => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop
    }
  }, [])

  const handleFormat = useCallback(() => {
    try {
      const parsed = JSON.parse(value)
      onChange(JSON.stringify(parsed, null, 2))
    } catch {
      // Can't format invalid JSON
    }
  }, [value, onChange])

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault()
      const textarea = e.currentTarget
      const { selectionStart, selectionEnd } = textarea
      const newValue = value.substring(0, selectionStart) + '  ' + value.substring(selectionEnd)
      onChange(newValue)
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = selectionStart + 2
      }, 0)
    }
  }, [value, onChange])

  const lineCount = value.split('\n').length

  return (
    <div className={`border border-neutral-200 bg-white text-xs ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-2 py-1 border-b border-neutral-100 bg-neutral-50">
        <div className="flex items-center gap-1">
          {error ? (
            <span className="text-error flex items-center gap-1">
              <AlertCircle className="w-3 h-3" />
              Invalid
            </span>
          ) : value.trim() ? (
            <span className="text-green-600 flex items-center gap-1">
              <Check className="w-3 h-3" />
              Valid
            </span>
          ) : null}
        </div>
        <button
          type="button"
          onClick={handleFormat}
          disabled={!!error || readOnly}
          className="p-1 text-neutral-400 hover:text-neutral-600 disabled:opacity-30"
          title="Format"
        >
          <Wand2 className="w-3 h-3" />
        </button>
      </div>

      {/* Editor */}
      <div className="relative flex h-64 overflow-hidden">
        {/* Line numbers */}
        <div
          ref={lineNumbersRef}
          className="w-8 bg-neutral-50 border-r border-neutral-100 overflow-hidden select-none"
        >
          <div className="py-2 px-1 text-right">
            {Array.from({ length: lineCount }, (_, i) => (
              <div key={i} className="text-neutral-300 leading-4 h-4">{i + 1}</div>
            ))}
          </div>
        </div>

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onScroll={handleScroll}
          onKeyDown={handleKeyDown}
          readOnly={readOnly}
          spellCheck={false}
          className={`flex-1 p-2 font-mono text-xs leading-4 resize-none outline-none ${
            readOnly ? 'bg-neutral-50 text-neutral-500' : 'bg-white text-neutral-800'
          }`}
          style={{ tabSize: 2 }}
        />
      </div>

      {/* Error */}
      {error && (
        <div className="px-2 py-1 border-t border-red-100 bg-red-50 text-error truncate">
          {error}
        </div>
      )}
    </div>
  )
}
