package codegen

// TypeKind represents the kind of a type in the intermediate representation.
type TypeKind int

const (
	KindObject TypeKind = iota
	KindArray
	KindString
	KindNumber
	KindInteger
	KindBoolean
	KindEnum
	KindAny
	KindRef // Reference to another type
)

// String returns the string representation of the TypeKind.
func (k TypeKind) String() string {
	switch k {
	case KindObject:
		return "object"
	case KindArray:
		return "array"
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	case KindInteger:
		return "integer"
	case KindBoolean:
		return "boolean"
	case KindEnum:
		return "enum"
	case KindAny:
		return "any"
	case KindRef:
		return "ref"
	default:
		return "unknown"
	}
}

// Schema represents a parsed schema ready for code generation.
type Schema struct {
	Name        string
	Topic       string
	Version     string
	Description string
	Root        *Type
	Definitions map[string]*Type // Named definitions/nested types
}

// Type represents a type in the intermediate representation.
type Type struct {
	Kind        TypeKind
	Name        string     // For named nested types (e.g., OrderPlacedItem)
	Description string     // Description from JSON Schema
	Properties  []Property // For objects
	Items       *Type      // For arrays
	Enum        []string   // For enums (string enums)
	Nullable    bool       // Whether the type can be null
	Format      string     // date-time, email, uuid, uri, etc.
	Ref         string     // For KindRef: the reference name

	// Validation constraints
	MinLength *int     // For strings
	MaxLength *int     // For strings
	Pattern   string   // For strings (regex pattern)
	Minimum   *float64 // For numbers
	Maximum   *float64 // For numbers
	MinItems  *int     // For arrays
	MaxItems  *int     // For arrays
}

// Property represents a property of an object type.
type Property struct {
	Name        string
	JSONName    string // Original JSON field name
	Type        *Type
	Required    bool
	Description string
}

// NewObjectType creates a new object type.
func NewObjectType(name string) *Type {
	return &Type{
		Kind: KindObject,
		Name: name,
	}
}

// NewArrayType creates a new array type.
func NewArrayType(items *Type) *Type {
	return &Type{
		Kind:  KindArray,
		Items: items,
	}
}

// NewStringType creates a new string type.
func NewStringType() *Type {
	return &Type{Kind: KindString}
}

// NewNumberType creates a new number type.
func NewNumberType() *Type {
	return &Type{Kind: KindNumber}
}

// NewIntegerType creates a new integer type.
func NewIntegerType() *Type {
	return &Type{Kind: KindInteger}
}

// NewBooleanType creates a new boolean type.
func NewBooleanType() *Type {
	return &Type{Kind: KindBoolean}
}

// NewEnumType creates a new enum type.
func NewEnumType(values []string) *Type {
	return &Type{
		Kind: KindEnum,
		Enum: values,
	}
}

// NewAnyType creates a new any type.
func NewAnyType() *Type {
	return &Type{Kind: KindAny}
}

// NewRefType creates a reference to another type.
func NewRefType(ref string) *Type {
	return &Type{
		Kind: KindRef,
		Ref:  ref,
	}
}

// IsRequired returns true if the property is required.
func (p Property) IsRequired() bool {
	return p.Required
}

// IsOptional returns true if the property is optional.
func (p Property) IsOptional() bool {
	return !p.Required || p.Type.Nullable
}

// HasConstraints returns true if the type has validation constraints.
func (t *Type) HasConstraints() bool {
	return t.MinLength != nil || t.MaxLength != nil || t.Pattern != "" ||
		t.Minimum != nil || t.Maximum != nil ||
		t.MinItems != nil || t.MaxItems != nil
}
