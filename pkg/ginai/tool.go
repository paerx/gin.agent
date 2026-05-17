package ginai

// Tool describes one Gin API that is explicitly exposed to the AI layer.
type Tool struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Method       string      `json:"method"`
	Path         string      `json:"path"`
	Params       any         `json:"-"`
	Schema       *JSONSchema `json:"schema,omitempty"`
	ReadOnly     bool        `json:"read_only"`
	NeedConfirm  bool        `json:"need_confirm"`
	Dangerous    bool        `json:"dangerous"`
	Roles        []string    `json:"roles,omitempty"`
	AllowFields  []string    `json:"allow_fields,omitempty"`
	MaxBatchSize int         `json:"max_batch_size,omitempty"`
}

// LLMToolSchema is the tool shape exported to the planner.
type LLMToolSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  *JSONSchema `json:"parameters"`
}
