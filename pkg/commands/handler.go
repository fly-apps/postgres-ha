package commands

import (
	"context"
	"net/http"

	"github.com/fly-apps/postgres-ha/pkg/flypg"
	"github.com/fly-apps/postgres-ha/pkg/flypg/stolon"
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

	r.Route("/admin", func(r chi.Router) {
		r.Get("/role", handleRole)
		r.Get("/failover/trigger", handleFailoverTrigger)
		r.Get("/restart", handleRestart)
		r.Get("/settings/view", handleViewSettings)
		r.Get("/replicationstats", handleReplicationStats)
		r.Post("/readonly/enable", handleEnableReadonly)
		r.Post("/readonly/disable", handleDisableReadonly)
		r.Get("/dbuid", handleStolonDBUid)
		r.Post("/haproxy/restart", handleRestartHaproxy)
		r.Post("/settings/update", handleUpdateSettings)
	})

	return r
}

func proxyConnection(ctx context.Context) (*pgx.Conn, func() error, error) {
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

func localConnection(ctx context.Context) (*pgx.Conn, func() error, error) {
	node, err := flypg.NewNode()
	if err != nil {
		return nil, nil, err
	}

	pg, err := node.NewLocalConnection(ctx)
	if err != nil {
		return nil, nil, err
	}
	close := func() error {
		return pg.Close(ctx)
	}

	return pg, close, nil
}

func masterKeeperUID(data *stolon.ClusterData) string {
	db := data.DBs[data.Cluster.Status.Master]
	return db.Spec.KeeperUID
}
