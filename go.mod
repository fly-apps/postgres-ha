module github.com/fly-examples/postgres-ha

go 1.16

require (
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/jackc/pgtype v1.6.2
	github.com/jackc/pgx/v4 v4.10.1
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.1.0
	github.com/shirou/gopsutil/v3 v3.21.3
	github.com/sorintlab/stolon v0.17.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
)

replace github.com/sorintlab/stolon => github.com/superfly/stolon v0.16.1-0.20220327002213-e9961aaae1da
