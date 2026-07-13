package model

type Provider struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Protocol string            `json:"protocol"`
	BaseURL  string            `json:"base_url"`
	Enabled  bool              `json:"enabled"`
	Headers  map[string]string `json:"headers"`
}

type APIKey struct {
	ID         string `json:"id"`
	ProviderID string `json:"provider_id"`
	Label      string `json:"label"`
	Value      string `json:"key_value"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
}

type Alias struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Routing string        `json:"routing"`
	Enabled bool          `json:"enabled"`
	Targets []AliasTarget `json:"targets"`
}

type AliasTarget struct {
	ProviderID string `json:"provider_id"`
	ModelName  string `json:"model_name"`
	Position   int    `json:"position"`
}

type GatewayKey struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Hash    string `json:"-"`
	Prefix  string `json:"prefix"`
	Enabled bool   `json:"enabled"`
}

type AdminUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}
