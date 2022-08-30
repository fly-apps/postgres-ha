package commands

import (
	"encoding/json"
	"net/http"

	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/go-chi/chi/v5"
)

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	conn, close, err := proxyConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	users, err := admin.ListUsers(r.Context(), conn)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{
		Result: users,
	}

	render.JSON(w, res, http.StatusOK)

}

func handleFindUser(w http.ResponseWriter, r *http.Request) {
	conn, close, err := proxyConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	user, err := admin.FindUser(r.Context(), conn, name)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{
		Result: user,
	}
	render.JSON(w, res, http.StatusOK)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	conn, close, err := proxyConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	var input createUserRequest

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.Err(w, err)
		return
	}
	defer r.Body.Close()

	err = admin.CreateUser(r.Context(), conn, input.Username, input.Password)
	if err != nil {
		render.Err(w, err)
		return
	}

	if input.Database != "" {
		err = admin.GrantAccess(r.Context(), conn, input.Username, input.Database)
		if err != nil {
			render.Err(w, err)
			return
		}
	}

	if input.Superuser {
		err = admin.GrantSuperuser(r.Context(), conn, input.Username)
		if err != nil {
			render.Err(w, err)
			return
		}
	}
	res := &Response{
		Result: true,
	}

	render.JSON(w, res, http.StatusOK)
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	conn, close, err := proxyConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	err = admin.DeleteUser(r.Context(), conn, name)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{Result: true}

	render.JSON(w, res, http.StatusOK)
}
