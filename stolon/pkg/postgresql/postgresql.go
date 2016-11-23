// Copyright 2015 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package postgresql

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/sorintlab/stolon/common"

	_ "github.com/lib/pq"
	"github.com/uber-go/zap"
	"golang.org/x/net/context"
)

const (
	postgresConf    = "postgresql.conf"
	tmpPostgresConf = "stolon-temp-postgresql.conf"
)

var log = zap.New(zap.NullEncoder())

func SetLogger(l zap.Logger) {
	log = l
}

type Manager struct {
	pgBinPath       string
	dataDir         string
	parameters      common.Parameters
	localConnParams ConnParams
	replConnParams  ConnParams
	suUsername      string
	suPassword      string
	replUsername    string
	replPassword    string
	requestTimeout  time.Duration
}

type SystemData struct {
	SystemID   string
	TimelineID uint64
	XLogPos    uint64
}

type TimelineHistory struct {
	TimelineID  uint64
	SwitchPoint uint64
	Reason      string
}

func NewManager(pgBinPath string, dataDir string, parameters common.Parameters, localConnParams, replConnParams ConnParams, suUsername, suPassword, replUsername, replPassword string, requestTimeout time.Duration) *Manager {
	return &Manager{
		pgBinPath:       pgBinPath,
		dataDir:         filepath.Join(dataDir, "postgres"),
		parameters:      parameters,
		replConnParams:  replConnParams,
		localConnParams: localConnParams,
		suUsername:      suUsername,
		suPassword:      suPassword,
		replUsername:    replUsername,
		replPassword:    replPassword,
		requestTimeout:  requestTimeout,
	}
}

func (p *Manager) SetParameters(parameters common.Parameters) {
	p.parameters = parameters
}

func (p *Manager) GetParameters() common.Parameters {
	return p.parameters
}

func (p *Manager) Init() error {
	// ioutil.Tempfile already creates files with 0600 permissions
	pwfile, err := ioutil.TempFile("", "pwfile")
	if err != nil {
		return err
	}
	defer os.Remove(pwfile.Name())
	defer pwfile.Close()

	pwfile.WriteString(p.suPassword)

	name := filepath.Join(p.pgBinPath, "initdb")
	out, err := exec.Command(name, "-D", p.dataDir, "-U", p.suUsername, "--pwfile", pwfile.Name()).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("error: %v, output: %s", err, out)
		goto out
	}
	// On every error remove the dataDir, so we don't end with an half initialized database
out:
	if err != nil {
		os.RemoveAll(p.dataDir)
		return err
	}
	return nil
}

func (p *Manager) Restore(command string) error {
	var err error
	var output []byte

	command = expand(command, p.dataDir)

	if err = os.MkdirAll(p.dataDir, 0700); err != nil {
		err = fmt.Errorf("cannot create data dir: %v", err)
		goto out
	}
	output, err = exec.Command("/bin/sh", "-c", command).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("error: %v, output: %s", err, output)
		goto out
	}
	// On every error remove the dataDir, so we don't end with an half initialized database
out:
	if err != nil {
		os.RemoveAll(p.dataDir)
		return err
	}
	return nil
}

func (p *Manager) StartTmpMerged(args ...string) error {
	// start postgres with a conf file different then postgresql.conf so we don't have to touch it
	f, err := os.Create(filepath.Join(p.dataDir, tmpPostgresConf))
	if err != nil {
		return err
	}
	defer f.Close()

	// include postgresql.conf if it exists
	_, err = os.Stat(filepath.Join(p.dataDir, postgresConf))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !os.IsNotExist(err) {
		f.WriteString(fmt.Sprintf("include '%s'\n", postgresConf))
	}
	for k, v := range p.parameters {
		// Single quotes needs to be doubled
		ev := strings.Replace(v, `'`, `''`, -1)
		_, err = f.WriteString(fmt.Sprintf("%s = '%s'\n", k, ev))
		if err != nil {
			os.Remove(f.Name())
			return err
		}
	}
	if err = f.Sync(); err != nil {
		return err
	}

	if err := p.writePgHba(); err != nil {
		return fmt.Errorf("error writing conf file: %v", err)
	}
	return p.start("-o", fmt.Sprintf("-c config_file=%s", f.Name()))
}

func (p *Manager) Start() error {
	if err := p.WriteConf(); err != nil {
		return fmt.Errorf("error writing conf file: %v", err)
	}
	if err := p.writePgHba(); err != nil {
		return fmt.Errorf("error writing conf file: %v", err)
	}
	return p.start()
}

func (p *Manager) start(args ...string) error {
	log.Info("starting database")
	name := filepath.Join(p.pgBinPath, "pg_ctl")
	args = append([]string{"start", "-w", "-D", p.dataDir, "-o", "-c unix_socket_directories=/tmp"}, args...)
	cmd := exec.Command(name, args...)
	log.Debug("execing cmd", zap.Object("cmd", cmd))
	// TODO(sgotti) attaching a pipe to sdtout/stderr makes the postgres
	// process executed by pg_ctl inheriting it's file descriptors. So
	// cmd.Wait() will block and waiting on them to be closed (will happend
	// only when postgres is stopped). So this functions will never return.
	// To avoid this no output is captured. If needed there's the need to
	// find a way to get the output whitout blocking.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error: %v", err)
	}
	return nil
}

func (p *Manager) Stop(fast bool) error {
	log.Info("stopping database")
	name := filepath.Join(p.pgBinPath, "pg_ctl")
	cmd := exec.Command(name, "stop", "-w", "-D", p.dataDir, "-o", "-c unix_socket_directories=/tmp")
	if fast {
		cmd.Args = append(cmd.Args, "-m", "fast")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	return nil
}

func (p *Manager) IsStarted() (bool, error) {
	name := filepath.Join(p.pgBinPath, "pg_ctl")
	cmd := exec.Command(name, "status", "-w", "-D", p.dataDir, "-o", "-c unix_socket_directories=/tmp")
	_, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			status := cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
			if status == 3 {
				return false, nil
			}
		}
		return false, fmt.Errorf("cannot get instance state: %v", err)
	}
	return true, nil
}

func (p *Manager) Reload() error {
	log.Info("reloading database configuration")
	if err := p.WriteConf(); err != nil {
		return fmt.Errorf("error writing conf file: %v", err)
	}
	if err := p.writePgHba(); err != nil {
		return fmt.Errorf("error writing conf file: %v", err)
	}
	name := filepath.Join(p.pgBinPath, "pg_ctl")
	cmd := exec.Command(name, "reload", "-D", p.dataDir, "-o", "-c unix_socket_directories=/tmp")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	return nil
}

func (p *Manager) Restart(fast bool) error {
	log.Info("restarting database")
	if err := p.Stop(fast); err != nil {
		return err
	}
	if err := p.Start(); err != nil {
		return err
	}
	return nil
}

func (p *Manager) Promote() error {
	log.Info("promoting database")
	name := filepath.Join(p.pgBinPath, "pg_ctl")
	cmd := exec.Command(name, "promote", "-w", "-D", p.dataDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	return nil
}

func (p *Manager) SetupRoles() error {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()

	if p.suUsername == p.replUsername {
		log.Info("adding replication role to superuser")
		if err := alterRole(ctx, p.localConnParams, []string{"replication"}, p.suUsername, p.suPassword); err != nil {
			return fmt.Errorf("error adding replication role to superuser: %v", err)
		}
		log.Info("replication role added to superuser")
	} else {
		// Configure superuser role password
		if p.suPassword != "" {
			log.Info("setting superuser password")
			if err := setPassword(ctx, p.localConnParams, p.suUsername, p.suPassword); err != nil {
				return fmt.Errorf("error setting superuser password: %v", err)
			}
			log.Info("superuser password set")
		}
		roles := []string{"login", "replication"}
		log.Info("creating replication role")
		if err := createRole(ctx, p.localConnParams, roles, p.replUsername, p.replPassword); err != nil {
			return fmt.Errorf("error creating replication role: %v", err)
		}
		log.Info("replication role created", zap.String("role", p.replUsername))
	}
	return nil
}

func (p *Manager) GetReplicatinSlots() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return getReplicatinSlots(ctx, p.localConnParams)
}

func (p *Manager) CreateReplicationSlot(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return createReplicationSlot(ctx, p.localConnParams, name)
}

func (p *Manager) DropReplicationSlot(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return dropReplicationSlot(ctx, p.localConnParams, name)
}

func (p *Manager) IsInitialized() (bool, error) {
	// List of required files or directories relative to postgres data dir
	// From https://www.postgresql.org/docs/9.4/static/storage-file-layout.html
	// with some additions.
	// TODO(sgotti) when the code to get the current db version is in place
	// also add additinal files introduced by releases after 9.4.
	requiredFiles := []string{
		"PG_VERSION",
		"base",
		"global",
		"pg_clog",
		"pg_dynshmem",
		"pg_logical",
		"pg_multixact",
		"pg_notify",
		"pg_replslot",
		"pg_serial",
		"pg_snapshots",
		"pg_stat",
		"pg_stat_tmp",
		"pg_subtrans",
		"pg_tblspc",
		"pg_twophase",
		"pg_xlog",
		"global/pg_control",
	}
	for _, f := range requiredFiles {
		exists, err := fileExists(filepath.Join(p.dataDir, f))
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func (p *Manager) GetRoleFromDB() (common.Role, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return getRole(ctx, p.localConnParams)
}

func (p *Manager) GetRole() (common.Role, error) {
	curConnParams, err := p.GetPrimaryConninfo()
	if err != nil {
		return "", fmt.Errorf("error retrieving primary conn info: %v", err)
	}
	if curConnParams == nil {
		return common.RoleMaster, nil
	}
	return common.RoleStandby, nil
}

func (p *Manager) GetPrimaryConninfo() (ConnParams, error) {
	regex, err := regexp.Compile(`\s*primary_conninfo\s*=\s*'(.*)'$`)
	if err != nil {
		return nil, err
	}
	fh, err := os.Open(filepath.Join(p.dataDir, "recovery.conf"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		m := regex.FindStringSubmatch(scanner.Text())
		if len(m) == 2 {
			return ParseConnString(m[1])
		}
	}
	return nil, nil
}

func (p *Manager) HasConnParams() (bool, error) {
	regex, err := regexp.Compile(`primary_conninfo`)
	if err != nil {
		return false, err
	}
	fh, err := os.Open(filepath.Join(p.dataDir, "recovery.conf"))
	if os.IsNotExist(err) {
		return false, nil
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if regex.MatchString(scanner.Text()) {
			return true, nil
		}
	}
	return false, nil
}

func (p *Manager) WriteConf() error {
	f, err := ioutil.TempFile(p.dataDir, postgresConf)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range p.parameters {
		// Single quotes needs to be doubled
		ev := strings.Replace(v, `'`, `''`, -1)
		_, err = f.WriteString(fmt.Sprintf("%s = '%s'\n", k, ev))
		if err != nil {
			os.Remove(f.Name())
			return err
		}
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(p.dataDir, postgresConf)); err != nil {
		os.Remove(f.Name())
		return err
	}

	return nil
}

func (p *Manager) WriteRecoveryConf(recoveryParameters common.Parameters) error {
	f, err := ioutil.TempFile(p.dataDir, "recovery.conf")
	if err != nil {
		return err
	}
	defer f.Close()

	for n, v := range recoveryParameters {
		f.WriteString(fmt.Sprintf("%s = '%s'\n", n, v))
	}

	if err = f.Sync(); err != nil {
		return err
	}

	if err = os.Rename(f.Name(), filepath.Join(p.dataDir, "recovery.conf")); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

func (p *Manager) writePgHba() error {
	f, err := ioutil.TempFile(p.dataDir, "pg_hba.conf")
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("local all all md5\n")
	// TODO(sgotti) Do not set this but let the user provide its pg_hba.conf file/entries
	f.WriteString("host all all 0.0.0.0/0 md5\n")
	f.WriteString("host all all ::0/0 md5\n")
	// TODO(sgotti) Configure this dynamically based on our followers provided by the clusterview
	f.WriteString(fmt.Sprintf("host replication %s %s md5\n", p.replUsername, "0.0.0.0/0"))
	f.WriteString(fmt.Sprintf("host replication %s %s md5\n", p.replUsername, "::0/0"))

	if err = os.Rename(f.Name(), filepath.Join(p.dataDir, "pg_hba.conf")); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

func (p *Manager) SyncFromFollowedPGRewind(followedConnParams ConnParams, password string) error {
	// ioutil.Tempfile already creates files with 0600 permissions
	pgpass, err := ioutil.TempFile("", "pgpass")
	if err != nil {
		return err
	}
	defer os.Remove(pgpass.Name())
	defer pgpass.Close()

	host := followedConnParams.Get("host")
	port := followedConnParams.Get("port")
	user := followedConnParams.Get("user")
	pgpass.WriteString(fmt.Sprintf("%s:%s:*:%s:%s\n", host, port, user, password))

	// Disable syncronous replication. pg_rewind needs to create a
	// temporary table on the master but if synchronous replication is
	// enabled and there're no active standbys it will hang.
	followedConnParams.Set("options", "-c synchronous_commit=off")
	followedConnString := followedConnParams.ConnString()

	log.Info("running pg_rewind")
	name := filepath.Join(p.pgBinPath, "pg_rewind")
	cmd := exec.Command(name, "--debug", "-D", p.dataDir, "--source-server="+followedConnString)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSFILE=%s", pgpass.Name()))
	log.Debug("execing cmd", zap.Object("cmd", cmd))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	log.Debug("cmd out", zap.String("out", string(out)))
	return nil
}

func (p *Manager) SyncFromFollowed(followedConnParams ConnParams) error {
	// ioutil.Tempfile already creates files with 0600 permissions
	pgpass, err := ioutil.TempFile("", "pgpass")
	if err != nil {
		return err
	}
	defer os.Remove(pgpass.Name())
	defer pgpass.Close()

	host := followedConnParams.Get("host")
	port := followedConnParams.Get("port")
	user := followedConnParams.Get("user")
	password := followedConnParams.Get("password")
	pgpass.WriteString(fmt.Sprintf("%s:%s:*:%s:%s\n", host, port, user, password))

	log.Info("running pg_basebackup")
	name := filepath.Join(p.pgBinPath, "pg_basebackup")
	cmd := exec.Command(name, "-R", "-D", p.dataDir, "--host="+host, "--port="+port, "-U", user)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSFILE=%s", pgpass.Name()))
	log.Debug("execing cmd", zap.Object("cmd", cmd))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error: %v, output: %s", err, string(out))
	}
	return nil
}

func (p *Manager) RemoveAll() error {
	initialized, err := p.IsInitialized()
	if err != nil {
		return fmt.Errorf("failed to retrieve instance state: %v", err)
	}
	started := false
	if initialized {
		var err error
		started, err = p.IsStarted()
		if err != nil {
			return fmt.Errorf("failed to retrieve instance state: %v", err)
		}
	}
	if started {
		return fmt.Errorf("cannot remove postregsql database. Instance is active")
	}
	return os.RemoveAll(p.dataDir)
}

func (p *Manager) GetSystemData() (*SystemData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return getSystemData(ctx, p.replConnParams)
}

func (p *Manager) GetTimelinesHistory(timeline uint64) ([]*TimelineHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return getTimelinesHistory(ctx, timeline, p.replConnParams)
}

func (p *Manager) GetConfigFilePGParameters() (common.Parameters, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
	defer cancel()
	return getConfigFilePGParameters(ctx, p.localConnParams)
}
