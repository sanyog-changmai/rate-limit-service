package dto

type CheckRequestDto struct {
	APIKey  string `json:"api_key" binding:"required"`
	Service string `json:"service" binding:"required"`
	Route   string `json:"route" binding:"required"`
	Method  string `json:"method" binding:"required"`
}

type CheckResponseDto struct {
	Allowed    bool    `json:"allowed"`
	Remaining  int     `json:"remaining"`
	Limit      int     `json:"limit"`
	ResetAt    int64   `json:"reset_at"`
	RetryAfter float64 `json:"retry_after,omitempty"`
}
