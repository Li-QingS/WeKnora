package types

import (
	"encoding/json"
	"time"
)

// ModelCallStatus is the terminal status of one model call.
type ModelCallStatus string

const (
	ModelCallStatusSuccess ModelCallStatus = "success"
	ModelCallStatusFailed  ModelCallStatus = "failed"
)

// ModelCallInfo is the non-persistent payload collected by cost wrappers.
type ModelCallInfo struct {
	TenantID         uint64
	ModelID          string
	ModelName        string
	ModelType        string
	Purpose          string
	Status           ModelCallStatus
	StartedAt        time.Time
	FinishedAt       time.Time
	DurationMS       int64
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CacheReadTokens  int
	CacheWriteTokens int
	CacheMissTokens  int
	UnitType         string
	UnitCount        int64
	ErrorType        string
	ErrorMessage     string
	SessionID        string
	UserID           string
	PrincipalType    string
	PrincipalID      string
	RequestGroupID   string
	TraceID          string
}

// ModelCallRecord is the persisted model call ledger row.
type ModelCallRecord struct {
	ID               string          `gorm:"primaryKey;type:varchar(64)" json:"id"`
	TenantID         uint64          `gorm:"index" json:"tenant_id"`
	ModelID          string          `gorm:"type:varchar(128);index" json:"model_id"`
	ModelName        string          `gorm:"type:varchar(255)" json:"model_name"`
	ModelType        string          `gorm:"type:varchar(32)" json:"model_type"`
	Purpose          string          `gorm:"type:varchar(128)" json:"purpose"`
	Status           string          `gorm:"type:varchar(16)" json:"status"`
	StartedAt        time.Time       `json:"started_at"`
	FinishedAt       time.Time       `json:"finished_at"`
	DurationMS       int64           `json:"duration_ms"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	CacheReadTokens  int             `json:"cache_read_tokens"`
	CacheWriteTokens int             `json:"cache_write_tokens"`
	CacheMissTokens  int             `json:"cache_miss_tokens"`
	UnitType         string          `gorm:"type:varchar(32)" json:"unit_type"`
	UnitCount        int64           `json:"unit_count"`
	ErrorType        string          `gorm:"type:varchar(128)" json:"error_type"`
	ErrorMessage     string          `gorm:"type:text" json:"error_message"`
	SessionID        string          `gorm:"type:varchar(128)" json:"session_id"`
	UserID           string          `gorm:"type:varchar(255)" json:"user_id"`
	PrincipalType    string          `gorm:"type:varchar(32)" json:"principal_type"`
	PrincipalID      string          `gorm:"type:varchar(255)" json:"principal_id"`
	RequestGroupID   string          `gorm:"type:varchar(128)" json:"request_group_id"`
	TraceID          string          `gorm:"type:varchar(64)" json:"trace_id"`
	EstimatedCostUSD *float64        `gorm:"type:decimal(20,8)" json:"estimated_cost_usd"`
	PriceSnapshot    json.RawMessage `gorm:"type:jsonb" json:"price_snapshot"`
	CreatedAt        time.Time       `json:"created_at"`
}

// TableName returns the database table name for ModelCallRecord.
func (ModelCallRecord) TableName() string {
	return "model_call_records"
}

// ModelPrice configures the per-tenant price for one model.
type ModelPrice struct {
	ID                        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	TenantID                  uint64    `gorm:"index" json:"tenant_id"`
	ModelID                   string    `gorm:"type:varchar(128);index" json:"model_id"`
	InputPricePerMillion      *float64  `gorm:"type:decimal(20,8)" json:"input_price_per_million"`
	OutputPricePerMillion     *float64  `gorm:"type:decimal(20,8)" json:"output_price_per_million"`
	CacheReadPricePerMillion  *float64  `gorm:"type:decimal(20,8)" json:"cache_read_price_per_million"`
	CacheWritePricePerMillion *float64  `gorm:"type:decimal(20,8)" json:"cache_write_price_per_million"`
	UnitType                  string    `gorm:"type:varchar(32)" json:"unit_type"`
	UnitPrice                 *float64  `gorm:"type:decimal(20,8)" json:"unit_price"`
	Currency                  string    `gorm:"type:varchar(8)" json:"currency"`
	UpdatedBy                 string    `gorm:"type:varchar(128)" json:"updated_by"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// TableName returns the database table name for ModelPrice.
func (ModelPrice) TableName() string {
	return "model_prices"
}

// PriceSnapshot is stored on every call record so later price changes do not
// rewrite history.
type PriceSnapshot struct {
	Currency                  string   `json:"currency"`
	InputPricePerMillion      *float64 `json:"input_price_per_million,omitempty"`
	OutputPricePerMillion     *float64 `json:"output_price_per_million,omitempty"`
	CacheReadPricePerMillion  *float64 `json:"cache_read_price_per_million,omitempty"`
	CacheWritePricePerMillion *float64 `json:"cache_write_price_per_million,omitempty"`
	UnitType                  string   `json:"unit_type,omitempty"`
	UnitPrice                 *float64 `json:"unit_price,omitempty"`
}

// ModelCallFilter carries optional list/summary filters.
type ModelCallFilter struct {
	ModelID        string
	ModelType      string
	Status         string
	RequestGroupID string
	From           *time.Time
	To             *time.Time
}

// ModelCallSummaryItem is one aggregate row by model.
type ModelCallSummaryItem struct {
	ModelID          string   `json:"model_id"`
	ModelName        string   `json:"model_name"`
	ModelType        string   `json:"model_type"`
	Calls            int64    `json:"calls"`
	SuccessCount     int64    `json:"success_count"`
	FailedCount      int64    `json:"failed_count"`
	PromptTokens     int64    `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
	TotalTokens      int64    `json:"total_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`
	CacheMissTokens  int64    `json:"cache_miss_tokens"`
	EstimatedCostUSD *float64 `json:"estimated_cost_usd"`
}
