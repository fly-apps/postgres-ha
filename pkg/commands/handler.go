package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/jackc/pgx/v4"
)

const Port = 5600

func StartCommandsHandler() {

	http.HandleFunc("/commands/users", handleListUsers)
	http.HandleFunc("/commands/users/create", handleCreateUser)
	http.HandleFunc("/commands/users/delete", handleDeleteUser)

	http.HandleFunc("/commands/users/grant", handleGrantAccess)
	http.HandleFunc("/commands/users/revoke", handleRevokeAccess)
	http.HandleFunc("/commands/users/superuser/grant", handleGrantSuperuser)
	http.HandleFunc("/commands/users/superuser/revoke", handleRevokeSuperuser)

	http.HandleFunc("/commands/databases", handleListDatabases)
	http.HandleFunc("/commands/databases/create", handleCreateDatabase)
	http.HandleFunc("/commands/databases/delete", handleDeleteDatabase)

	fmt.Printf("Listening on port %d", Port)
	http.ListenAndServe(fmt.Sprintf(":%d", Port), nil)
}

type Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func handleListDatabases(w http.ResponseWriter, r *http.Request) {
	res, err := ListDatabases(r.Context(), nil)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	res, err := ListUsers(r.Context(), nil)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)

}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := CreateUser(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := DeleteUser(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := CreateDatabase(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := DeleteDatabase(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleGrantAccess(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := GrantAccess(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleRevokeAccess(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := RevokeAccess(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleGrantSuperuser(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := GrantSuperuser(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleRevokeSuperuser(w http.ResponseWriter, r *http.Request) {
	input := map[string]interface{}{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	res, err := RevokeSuperuser(r.Context(), input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func getConnection(ctx context.Context) (*pgx.Conn, error) {
	node, err := flypg.NewNode()
	if err != nil {
		return nil, err
	}

	pg, err := node.NewLocalConnection(ctx)
	if err != nil {
		return nil, err
	}
	return pg, nil
}
