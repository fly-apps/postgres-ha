package commands

type createUserRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Superuser bool   `json:"superuser"`
}

type createDatabaseRequest struct {
	Name string `json:"name"`
}

type grantAccessRequest struct {
	Username string `json:"username"`
	Database string `json:"database"`
}
