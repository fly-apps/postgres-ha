package commands

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4"
)

const Port = 5600

func Handler() http.Handler {
	r := chi.NewRouter()

	r.Route("/users", func(r chi.Router) {
		r.Get("/", handleListUsers)
		r.Post("/create", handleCreateUser)
		r.Delete("/delete", handleDeleteUser)
	})

	r.Route("/databases", func(r chi.Router) {
		r.Get("/", handleListDatabases)
		r.Post("/create", handleCreateDatabase)
		r.Delete("/delete", handleDeleteDatabase)
	})

	return r
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
