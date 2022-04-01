package commands

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fly-examples/postgres-ha/pkg/flypg"
	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4"
)

func Handler() http.Handler {
	r := chi.NewRouter()

	r.Route("/users", func(r chi.Router) {
		r.Get("/{name}", handleFindUser)
		r.Get("/list", handleListUsers)
		r.Post("/create", handleCreateUser)
		r.Delete("/delete/{name}", handleDeleteUser)
	})

	r.Route("/databases", func(r chi.Router) {
		r.Get("/list", handleListDatabases)
		r.Get("/{name}", handleFindDatabase)
		r.Post("/create", handleCreateDatabase)
		r.Delete("/delete/{name}", handleDeleteDatabase)
	})

	return r
}

type Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func handleListDatabases(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	res, err := admin.ListDatabases(r.Context(), pg)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	res, err := admin.ListUsers(r.Context(), pg)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)

}

func handleFindUser(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	res, err := admin.FindUser(r.Context(), pg, name)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleFindDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	res, err := admin.FindDatabase(r.Context(), pg, name)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: res}, http.StatusOK)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	var input createUserRequest

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	err = admin.CreateUser(r.Context(), pg, input.Username, input.Password)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}

	if input.Database != "" {
		err = admin.GrantAccess(r.Context(), pg, input.Username, input.Database)
		if err != nil {
			render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	if input.Superuser {
		err = admin.GrantSuperuser(r.Context(), pg, input.Username)
		if err != nil {
			render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	render.JSON(w, &Response{Result: true}, http.StatusOK)
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	err = admin.DeleteUser(r.Context(), pg, name)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: true}, http.StatusOK)
}

func handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	input := createDatabaseRequest{}

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	err = admin.CreateDatabase(r.Context(), pg, input.Name)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: true}, http.StatusOK)
}

func handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	err = admin.DeleteDatabase(r.Context(), pg, name)
	if err != nil {
		render.JSON(w, &Response{Error: err.Error()}, http.StatusInternalServerError)
		return
	}
	render.JSON(w, &Response{Result: true}, http.StatusOK)
}

func getConnection(ctx context.Context) (*pgx.Conn, func() error, error) {
	node, err := flypg.NewNode()
	if err != nil {
		return nil, nil, err
	}

	pg, err := node.NewProxyConnection(ctx)
	if err != nil {
		return nil, nil, err
	}
	close := func() error {
		return pg.Close(ctx)
	}

	return pg, close, nil
}
