package data

import "time"

// Column defines a table column in DDL operations.
type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	IsNullable   bool    `json:"is_nullable"`
	DefaultValue *string `json:"default_value,omitempty"`
	IsPrimaryKey bool    `json:"is_primary_key"`
	IsUnique     bool    `json:"is_unique"`
}

// ForeignKey describes a foreign key constraint.
type ForeignKey struct {
	ConstraintName string `json:"constraint_name,omitempty"`
	Column         string `json:"column"`
	ForeignSchema  string `json:"foreign_schema"`
	ForeignTable   string `json:"foreign_table"`
	ForeignColumn  string `json:"foreign_column"`
	OnDelete       string `json:"on_delete,omitempty"` // CASCADE, SET NULL, RESTRICT
	OnUpdate       string `json:"on_update,omitempty"`
}

// Index describes an index on a table.
type Index struct {
	IndexName string   `json:"index_name"`
	Columns   []string `json:"columns"`
	Type      string   `json:"type"` // btree, gin, gist, brin
	IsUnique  bool     `json:"is_unique"`
}

// Policy describes a Row-Level Security policy on a table.
type Policy struct {
	Schema          string   `json:"schema"`
	Table           string   `json:"table"`
	Name            string   `json:"name"`
	Command         string   `json:"command"` // ALL, SELECT, INSERT, UPDATE, DELETE
	Roles           []string `json:"roles"`
	Permissive      string   `json:"permissive"` // PERMISSIVE, RESTRICTIVE
	UsingExpression string   `json:"using_expression,omitempty"`
	CheckExpression string   `json:"with_check_expression,omitempty"`
}

// Table describes a table's schema, constraints, indexes, and statistics.
type Table struct {
	Schema            string       `json:"schema"`
	Name              string       `json:"name"`
	PrimaryKey        string       `json:"primary_key"`
	Columns           []Column     `json:"columns"`
	ForeignKeys       []ForeignKey `json:"foreign_keys"`
	Indexes           []Index      `json:"indexes"`
	RLSEnabled        bool         `json:"rls_enabled"`
	RLSForced         bool         `json:"rls_forced"`
	EstimatedRowCount int64        `json:"estimated_row_count"`
	SizeBytes         int64        `json:"size_bytes"`
}

// CreateTableInput describes the schema for creating a new table.
type CreateTableInput struct {
	Schema         string       `json:"schema"`
	Name           string       `json:"name"`
	PrimaryKeyName string       `json:"primary_key_name,omitempty"` // defaults to "id" if empty
	Columns        []Column     `json:"columns"`
	ForeignKeys    []ForeignKey `json:"foreign_keys,omitempty"`
}

// UpdateColumnInput describes changes to apply to an existing column.
type UpdateColumnInput struct {
	NewName      *string `json:"new_name,omitempty"`
	NewType      *string `json:"new_type,omitempty"`
	IsNullable   *bool   `json:"is_nullable,omitempty"`
	DefaultValue *string `json:"default_value,omitempty"`
}

// CreateIndexInput describes parameters for creating an index.
type CreateIndexInput struct {
	IndexName string   `json:"index_name,omitempty"`
	Columns   []string `json:"columns"`
	Type      string   `json:"type"` // btree, gin, gist, brin
	IsUnique  bool     `json:"is_unique"`
}

// CreatePolicyInput describes parameters for creating an RLS policy.
type CreatePolicyInput struct {
	Name            string   `json:"name"`
	Command         string   `json:"command,omitempty"`    // Default: ALL
	Roles           []string `json:"roles,omitempty"`      // Default: [PUBLIC]
	Permissive      string   `json:"permissive,omitempty"` // Default: PERMISSIVE
	UsingExpression string   `json:"using_expression,omitempty"`
	CheckExpression string   `json:"with_check_expression,omitempty"`
}

// ToggleRLSInput describes the action to change RLS on a table.
type ToggleRLSInput struct {
	Action string `json:"action,omitempty"`
	Mode   string `json:"mode,omitempty"` // ENABLE, DISABLE, FORCE, NO FORCE
}

// InvalidateCacheInput represents targeted cache invalidation options.
type InvalidateCacheInput struct {
	All     bool   `json:"all,omitempty"`
	Catalog bool   `json:"catalog,omitempty"`
	Schema  string `json:"schema,omitempty"`
	Table   string `json:"table,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// ExecuteSQLInput represents an arbitrary SQL script execution payload.
type ExecuteSQLInput struct {
	SQL   string `json:"sql,omitempty"`
	Query string `json:"query,omitempty"`
}

// CreateTableResponse represents the result of table creation.
type CreateTableResponse struct {
	Status  string `json:"status"`
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Message string `json:"message"`
}

// DeleteTableResponse represents the result of dropping a table.
type DeleteTableResponse struct {
	Status  string `json:"status"`
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Message string `json:"message"`
}

// CreateColumnResponse represents the result of adding a column.
type CreateColumnResponse struct {
	Status string `json:"status"`
	Column string `json:"column"`
}

// UpdateColumnResponse represents the result of altering a column.
type UpdateColumnResponse struct {
	Status string `json:"status"`
	Column string `json:"column"`
}

// DeleteColumnResponse represents the result of dropping a column.
type DeleteColumnResponse struct {
	Status string `json:"status"`
	Column string `json:"column"`
}

// CreateIndexResponse represents the result of creating an index.
type CreateIndexResponse struct {
	Status string `json:"status"`
	Index  string `json:"index"`
}

// ListIndexesResponse represents a list of index definitions on a table.
type ListIndexesResponse struct {
	Indexes []Index `json:"indexes"`
	Count   int     `json:"count"`
}

// DeleteIndexResponse represents the result of dropping an index.
type DeleteIndexResponse struct {
	Status string `json:"status"`
	Index  string `json:"index"`
}

// CreatePolicyResponse represents the result of creating an RLS policy.
type CreatePolicyResponse struct {
	Status string `json:"status"`
	Policy string `json:"policy"`
}

// ListPoliciesResponse represents a list of RLS policies on a table.
type ListPoliciesResponse struct {
	Policies []Policy `json:"policies"`
	Count    int      `json:"count"`
}

// DeletePolicyResponse represents the result of dropping an RLS policy.
type DeletePolicyResponse struct {
	Status string `json:"status"`
	Policy string `json:"policy"`
}

// ToggleRLSResponse represents the result of toggling RLS on a table.
type ToggleRLSResponse struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
}

// FlushCacheResponse represents the confirmation of global cache flush.
type FlushCacheResponse struct {
	Status    string    `json:"status"`
	FlushedAt time.Time `json:"flushed_at"`
}

// InvalidateCacheResponse represents the confirmation of targeted cache invalidation.
type InvalidateCacheResponse struct {
	Status  string `json:"status"`
	Pattern string `json:"pattern"`
}

// ListTablesResponse represents a list of table summaries.
type ListTablesResponse struct {
	Tables []Table `json:"tables"`
	Count  int     `json:"count"`
}

// ExecuteSQLResponse represents execution timing and results from raw SQL queries.
type ExecuteSQLResponse struct {
	Columns         []string         `json:"columns"`
	Rows            []map[string]any `json:"rows"`
	RowsAffected    int64            `json:"rows_affected"`
	ExecutionTimeMs float64          `json:"execution_time_ms"`
}
