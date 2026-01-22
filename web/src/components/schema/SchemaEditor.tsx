import { useState, useEffect, useRef, useCallback } from 'react'
import { Check, X, AlertCircle, Copy, Wand2 } from 'lucide-react'
import { Button, Badge } from '../ui'

type SchemaEditorProps = {
  value: string
  onChange: (value: string) => void
  onValidChange?: (isValid: boolean) => void
  placeholder?: string
  className?: string
  readOnly?: boolean
}

export function SchemaEditor({
  value,
  onChange,
  onValidChange,
  placeholder = '{\n  "type": "object",\n  "properties": {}\n}',
  className = '',
  readOnly = false,
}: SchemaEditorProps) {
  const [error, setError] = useState<string | null>(null)
  const [cursorPosition, setCursorPosition] = useState({ line: 1, col: 1 })
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const lineNumbersRef = useRef<HTMLDivElement>(null)

  // Validate JSON on change
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
      const msg = e instanceof Error ? e.message : 'Invalid JSON'
      setError(msg)
      onValidChange?.(false)
    }
  }, [value, onValidChange])

  // Sync scroll between textarea and line numbers
  const handleScroll = useCallback(() => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop
    }
  }, [])

  // Track cursor position
  const handleSelect = useCallback(() => {
    if (textareaRef.current) {
      const { selectionStart } = textareaRef.current
      const lines = value.substring(0, selectionStart).split('\n')
      setCursorPosition({
        line: lines.length,
        col: lines[lines.length - 1].length + 1,
      })
    }
  }, [value])

  // Format JSON
  const handleFormat = useCallback(() => {
    try {
      const parsed = JSON.parse(value)
      onChange(JSON.stringify(parsed, null, 2))
    } catch {
      // Can't format invalid JSON
    }
  }, [value, onChange])

  // Copy to clipboard
  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(value)
  }, [value])

  // Handle tab key for indentation
  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault()
      const textarea = e.currentTarget
      const { selectionStart, selectionEnd } = textarea
      const newValue = value.substring(0, selectionStart) + '  ' + value.substring(selectionEnd)
      onChange(newValue)
      // Restore cursor position after React updates
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = selectionStart + 2
      }, 0)
    }
  }, [value, onChange])

  const lineCount = value.split('\n').length

  return (
    <div className={`border border-neutral-200 bg-white ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-neutral-200 bg-neutral-50">
        <div className="flex items-center gap-2">
          {error ? (
            <Badge variant="error" className="text-xs">
              <AlertCircle className="w-3 h-3 mr-1" />
              Invalid JSON
            </Badge>
          ) : value.trim() ? (
            <Badge variant="success" className="text-xs">
              <Check className="w-3 h-3 mr-1" />
              Valid JSON
            </Badge>
          ) : null}
        </div>
        <div className="flex items-center gap-1">
          <span className="text-xs text-neutral-400 mr-2">
            Ln {cursorPosition.line}, Col {cursorPosition.col}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleFormat}
            disabled={!!error || readOnly}
            title="Format JSON"
          >
            <Wand2 className="w-4 h-4" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleCopy}
            title="Copy to clipboard"
          >
            <Copy className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Editor */}
      <div className="relative flex h-80 overflow-hidden">
        {/* Line numbers */}
        <div
          ref={lineNumbersRef}
          className="w-12 bg-neutral-50 border-r border-neutral-200 overflow-hidden select-none"
        >
          <div className="py-3 px-2 text-right">
            {Array.from({ length: lineCount }, (_, i) => (
              <div
                key={i}
                className="text-xs text-neutral-400 font-mono leading-5 h-5"
              >
                {i + 1}
              </div>
            ))}
          </div>
        </div>

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onScroll={handleScroll}
          onSelect={handleSelect}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          readOnly={readOnly}
          spellCheck={false}
          className={`flex-1 p-3 font-mono text-sm leading-5 resize-none outline-none ${
            readOnly ? 'bg-neutral-50 text-neutral-600' : 'bg-white text-neutral-900'
          } ${error ? 'text-error' : ''}`}
          style={{ tabSize: 2 }}
        />
      </div>

      {/* Error message */}
      {error && (
        <div className="px-3 py-2 border-t border-error bg-red-50 text-sm text-error flex items-start gap-2">
          <X className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span className="font-mono text-xs">{error}</span>
        </div>
      )}
    </div>
  )
}
