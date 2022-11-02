package commands

type createUserRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Superuser bool   `json:"superuser"`
	Login     bool   `json:"login"`
	Database  string `json:"databases"`
}

type createDatabaseRequest struct {
	Name string `json:"name"`
}

type failOverResponse struct {
	Message string `json:"message"`
}

type Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}
