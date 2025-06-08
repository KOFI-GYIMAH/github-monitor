package handler

type AddRepositoryRequest struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
