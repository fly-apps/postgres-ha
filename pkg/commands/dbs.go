package commands

import (
	"encoding/json"
	"net/http"

	"github.com/fly-examples/postgres-ha/pkg/flypg/admin"
	"github.com/fly-examples/postgres-ha/pkg/render"
	"github.com/go-chi/chi/v5"
)

func handleListDatabases(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	dbs, err := admin.ListDatabases(r.Context(), pg)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{
		Result: dbs,
	}

	render.JSON(w, res, http.StatusOK)
}

func handleFindDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	db, err := admin.FindDatabase(r.Context(), pg, name)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{
		Result: db,
	}

	render.JSON(w, res, http.StatusOK)
}

func handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	input := createDatabaseRequest{}

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		render.Err(w, err)
		return
	}
	defer r.Body.Close()

	err = admin.CreateDatabase(r.Context(), pg, input.Name)
	if err != nil {
		render.Err(w, err)
		return
	}

	res := &Response{Result: true}

	render.JSON(w, res, http.StatusOK)
}

func handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	pg, close, err := getConnection(r.Context())
	if err != nil {
		render.Err(w, err)
		return
	}
	defer close()

	name := chi.URLParam(r, "name")

	err = admin.DeleteDatabase(r.Context(), pg, name)
	if err != nil {
		render.Err(w, err)
		return
	}
	res := &Response{Result: true}

	render.JSON(w, res, http.StatusOK)
}
