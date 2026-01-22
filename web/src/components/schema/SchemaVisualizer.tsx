import { useState } from 'react'
import { ChevronRight, ChevronDown } from 'lucide-react'

type JSONSchemaType = {
  type?: string | string[]
  properties?: Record<string, JSONSchemaType>
  items?: JSONSchemaType
  required?: string[]
  enum?: (string | number)[]
  description?: string
  format?: string
  $ref?: string
  oneOf?: JSONSchemaType[]
  anyOf?: JSONSchemaType[]
  allOf?: JSONSchemaType[]
}

type SchemaVisualizerProps = {
  schema: JSONSchemaType
  className?: string
}

export function SchemaVisualizer({ schema, className = '' }: SchemaVisualizerProps) {
  return (
    <div className={`font-mono text-xs ${className}`}>
      <PropertyList schema={schema} required={schema.required} depth={0} />
    </div>
  )
}

function PropertyList({
  schema,
  required = [],
  depth
}: {
  schema: JSONSchemaType
  required?: string[]
  depth: number
}) {
  const properties = schema.properties || {}
  const entries = Object.entries(properties)

  if (entries.length === 0) {
    return <span className="text-neutral-400">{'{ }'}</span>
  }

  return (
    <div>
      {entries.map(([name, prop], i) => (
        <PropertyRow
          key={name}
          name={name}
          schema={prop}
          isRequired={required.includes(name)}
          isLast={i === entries.length - 1}
          depth={depth}
        />
      ))}
    </div>
  )
}

function PropertyRow({
  name,
  schema,
  isRequired,
  isLast,
  depth
}: {
  name: string
  schema: JSONSchemaType
  isRequired: boolean
  isLast: boolean
  depth: number
}) {
  const [expanded, setExpanded] = useState(depth < 2)
  const type = getType(schema)
  const isExpandable = type === 'object' || type === 'array'
  const hasChildren = type === 'object' && schema.properties && Object.keys(schema.properties).length > 0
  const hasItems = type === 'array' && schema.items

  return (
    <div style={{ marginLeft: depth > 0 ? 16 : 0 }}>
      <div
        className={`flex items-center gap-1 py-0.5 hover:bg-neutral-50 ${isExpandable ? 'cursor-pointer' : ''}`}
        onClick={() => isExpandable && setExpanded(!expanded)}
      >
        {/* Expand icon */}
        <span className="w-3 h-3 flex items-center justify-center text-neutral-300">
          {isExpandable ? (
            expanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />
          ) : null}
        </span>

        {/* Name */}
        <span className="text-neutral-700">{name}</span>
        {isRequired && <span className="text-error">*</span>}
        <span className="text-neutral-300">:</span>

        {/* Type */}
        <span className={getTypeColor(type)}>{type}</span>
        {schema.format && <span className="text-neutral-400">({schema.format})</span>}

        {/* Enum preview */}
        {schema.enum && (
          <span className="text-neutral-400 truncate max-w-48">
            [{schema.enum.slice(0, 3).map(v => JSON.stringify(v)).join(', ')}
            {schema.enum.length > 3 && '...'}]
          </span>
        )}

        {/* Description */}
        {schema.description && (
          <span className="text-neutral-400 truncate max-w-64 ml-2">// {schema.description}</span>
        )}
      </div>

      {/* Children */}
      {expanded && hasChildren && (
        <PropertyList schema={schema} required={schema.required} depth={depth + 1} />
      )}

      {/* Array items */}
      {expanded && hasItems && (
        <div style={{ marginLeft: 16 }}>
          <div className="flex items-center gap-1 py-0.5 text-neutral-400">
            <span className="w-3" />
            <span>items:</span>
            <span className={getTypeColor(getType(schema.items!))}>{getType(schema.items!)}</span>
          </div>
          {schema.items?.properties && (
            <PropertyList schema={schema.items} required={schema.items.required} depth={depth + 2} />
          )}
        </div>
      )}
    </div>
  )
}

function getType(schema: JSONSchemaType): string {
  if (schema.$ref) return 'ref'
  if (schema.enum) return 'enum'
  if (schema.oneOf) return 'oneOf'
  if (schema.anyOf) return 'anyOf'
  if (Array.isArray(schema.type)) return schema.type.filter(t => t !== 'null').join('|')
  return schema.type || 'any'
}

function getTypeColor(type: string): string {
  switch (type) {
    case 'string': return 'text-green-600'
    case 'number':
    case 'integer': return 'text-blue-600'
    case 'boolean': return 'text-amber-600'
    case 'array': return 'text-purple-600'
    case 'object': return 'text-neutral-600'
    default: return 'text-neutral-500'
  }
}
