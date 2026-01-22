import { useState } from 'react'
import { ChevronRight, ChevronDown, Hash, Type, ToggleLeft, List, Braces, CircleDot } from 'lucide-react'
import { Badge } from '../ui'

type JSONSchemaType = {
  type?: string | string[]
  properties?: Record<string, JSONSchemaType>
  items?: JSONSchemaType
  required?: string[]
  enum?: (string | number)[]
  description?: string
  format?: string
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  pattern?: string
  default?: unknown
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
    <div className={`font-mono text-sm ${className}`}>
      <SchemaNode schema={schema} name="root" isRoot depth={0} />
    </div>
  )
}

type SchemaNodeProps = {
  schema: JSONSchemaType
  name: string
  isRoot?: boolean
  isRequired?: boolean
  depth: number
}

function SchemaNode({ schema, name, isRoot, isRequired, depth }: SchemaNodeProps) {
  const [expanded, setExpanded] = useState(depth < 2)
  const type = getSchemaType(schema)
  const hasChildren = type === 'object' && schema.properties && Object.keys(schema.properties).length > 0
  const hasItems = type === 'array' && schema.items
  const isExpandable = hasChildren || hasItems

  const TypeIcon = getTypeIcon(type)

  return (
    <div className="select-none">
      <div
        className={`flex items-center gap-2 py-1.5 px-2 hover:bg-neutral-100 cursor-pointer group ${isRoot ? 'bg-neutral-50' : ''}`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={() => isExpandable && setExpanded(!expanded)}
      >
        {/* Expand/collapse icon */}
        <span className="w-4 h-4 flex items-center justify-center text-neutral-400">
          {isExpandable ? (
            expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />
          ) : (
            <span className="w-4" />
          )}
        </span>

        {/* Type icon */}
        <TypeIcon className={`w-4 h-4 ${getTypeColor(type)}`} />

        {/* Property name */}
        {!isRoot && (
          <span className="text-neutral-900 font-medium">
            {name}
            {isRequired && <span className="text-error ml-0.5">*</span>}
          </span>
        )}

        {/* Type badge */}
        <Badge variant={getTypeBadgeVariant(type)} className="text-xs py-0 px-1.5">
          {type}
          {schema.format && <span className="text-neutral-400 ml-1">({schema.format})</span>}
        </Badge>

        {/* Enum values */}
        {schema.enum && (
          <span className="text-xs text-neutral-500">
            [{schema.enum.slice(0, 3).map(v => JSON.stringify(v)).join(', ')}
            {schema.enum.length > 3 && `, +${schema.enum.length - 3}`}]
          </span>
        )}

        {/* Constraints */}
        <span className="text-xs text-neutral-400 hidden group-hover:inline">
          {getConstraints(schema)}
        </span>
      </div>

      {/* Description */}
      {schema.description && (
        <div
          className="text-xs text-neutral-500 py-1 border-l-2 border-neutral-200"
          style={{ paddingLeft: `${depth * 16 + 36}px` }}
        >
          {schema.description}
        </div>
      )}

      {/* Children (object properties) */}
      {expanded && hasChildren && (
        <div>
          {Object.entries(schema.properties!).map(([propName, propSchema]) => (
            <SchemaNode
              key={propName}
              schema={propSchema}
              name={propName}
              isRequired={schema.required?.includes(propName)}
              depth={depth + 1}
            />
          ))}
        </div>
      )}

      {/* Array items */}
      {expanded && hasItems && (
        <SchemaNode
          schema={schema.items!}
          name="items"
          depth={depth + 1}
        />
      )}
    </div>
  )
}

function getSchemaType(schema: JSONSchemaType): string {
  if (schema.$ref) return 'ref'
  if (schema.enum) return 'enum'
  if (schema.oneOf) return 'oneOf'
  if (schema.anyOf) return 'anyOf'
  if (schema.allOf) return 'allOf'
  if (Array.isArray(schema.type)) return schema.type.join(' | ')
  return schema.type || 'any'
}

function getTypeIcon(type: string) {
  switch (type) {
    case 'string': return Type
    case 'number':
    case 'integer': return Hash
    case 'boolean': return ToggleLeft
    case 'array': return List
    case 'object': return Braces
    case 'enum': return CircleDot
    default: return Braces
  }
}

function getTypeColor(type: string): string {
  switch (type) {
    case 'string': return 'text-green-600'
    case 'number':
    case 'integer': return 'text-blue-600'
    case 'boolean': return 'text-amber-600'
    case 'array': return 'text-purple-600'
    case 'object': return 'text-neutral-600'
    case 'enum': return 'text-pink-600'
    default: return 'text-neutral-400'
  }
}

function getTypeBadgeVariant(type: string): 'default' | 'success' | 'warning' | 'error' | 'info' {
  switch (type) {
    case 'string': return 'success'
    case 'number':
    case 'integer': return 'info'
    case 'boolean': return 'warning'
    default: return 'default'
  }
}

function getConstraints(schema: JSONSchemaType): string {
  const constraints: string[] = []
  if (schema.minimum !== undefined) constraints.push(`min: ${schema.minimum}`)
  if (schema.maximum !== undefined) constraints.push(`max: ${schema.maximum}`)
  if (schema.minLength !== undefined) constraints.push(`minLen: ${schema.minLength}`)
  if (schema.maxLength !== undefined) constraints.push(`maxLen: ${schema.maxLength}`)
  if (schema.pattern) constraints.push(`pattern: /${schema.pattern}/`)
  return constraints.join(', ')
}
