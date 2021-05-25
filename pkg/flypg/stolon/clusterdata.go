package stolon

// copied from stolon because the cluster package is internal

import (
	"encoding/json"
	"strings"
	"time"
)

type ClusterData struct {
	// ClusterData format version. Used to detect incompatible
	// version and do upgrade. Needs to be bumped when a non
	// backward compatible change is done to the other struct
	// members.
	FormatVersion uint64    `json:"formatVersion"`
	ChangeTime    time.Time `json:"changeTime"`
	Cluster       *Cluster  `json:"cluster"`
	Keepers       Keepers   `json:"keepers"`
	DBs           DBs       `json:"dbs"`
	Proxy         *Proxy    `json:"proxy"`
}

func (cd *ClusterData) FindDB(keeperUID string) *DB {
	for _, db := range cd.DBs {
		if db.Spec.KeeperUID == keeperUID {
			return db
		}
	}
	return nil
}

type Cluster struct {
	UID        string    `json:"uid,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	ChangeTime time.Time `json:"changeTime,omitempty"`

	Spec *ClusterSpec `json:"spec,omitempty"`

	Status ClusterStatus `json:"status,omitempty"`
}

type ClusterSpec struct {
	// Interval to wait before next check
	SleepInterval *Duration `json:"sleepInterval,omitempty"`
	// Time after which any request (keepers checks from sentinel etc...) will fail.
	RequestTimeout *Duration `json:"requestTimeout,omitempty"`
	// Interval to wait for a db to be converged to the required state when
	// no long operation are expected.
	ConvergenceTimeout *Duration `json:"convergenceTimeout,omitempty"`
	// Interval to wait for a db to be initialized (doing a initdb)
	InitTimeout *Duration `json:"initTimeout,omitempty"`
	// Interval to wait for a db to be synced with a master
	SyncTimeout *Duration `json:"syncTimeout,omitempty"`
	// Interval to wait for a db to boot and become ready
	DBWaitReadyTimeout *Duration `json:"dbWaitReadyTimeout,omitempty"`
	// Interval after the first fail to declare a keeper or a db as not healthy.
	FailInterval *Duration `json:"failInterval,omitempty"`
	// Interval after which a dead keeper will be removed from the cluster data
	DeadKeeperRemovalInterval *Duration `json:"deadKeeperRemovalInterval,omitempty"`
	// Interval to wait before next proxy check
	ProxyCheckInterval *Duration `json:"proxyCheckInterval,omitempty"`
	// Interval where the proxy must successfully complete a check
	ProxyTimeout *Duration `json:"proxyTimeout,omitempty"`
	// Max number of standbys. This needs to be greater enough to cover both
	// standby managed by stolon and additional standbys configured by the
	// user. Its value affect different postgres parameters like
	// max_replication_slots and max_wal_senders. Setting this to a number
	// lower than the sum of stolon managed standbys and user managed
	// standbys will have unpredicatable effects due to problems creating
	// replication slots or replication problems due to exhausted wal
	// senders.
	MaxStandbys *uint16 `json:"maxStandbys,omitempty"`
	// Max number of standbys for every sender. A sender can be a master or
	// another standby (if/when implementing cascading replication).
	MaxStandbysPerSender *uint16 `json:"maxStandbysPerSender,omitempty"`
	// Max lag in bytes that an asynchronous standy can have to be elected in
	// place of a failed master
	MaxStandbyLag *uint32 `json:"maxStandbyLag,omitempty"`
	// Use Synchronous replication between master and its standbys
	SynchronousReplication *bool `json:"synchronousReplication,omitempty"`
	// MinSynchronousStandbys is the mininum number if synchronous standbys
	// to be configured when SynchronousReplication is true
	MinSynchronousStandbys *uint16 `json:"minSynchronousStandbys,omitempty"`
	// MaxSynchronousStandbys is the maximum number if synchronous standbys
	// to be configured when SynchronousReplication is true
	MaxSynchronousStandbys *uint16 `json:"maxSynchronousStandbys,omitempty"`
	// AdditionalWalSenders defines the number of additional wal_senders in
	// addition to the ones internally defined by stolon
	AdditionalWalSenders *uint16 `json:"additionalWalSenders"`
	// AdditionalMasterReplicationSlots defines additional replication slots to
	// be created on the master postgres instance. Replication slots not defined
	// here will be dropped from the master instance (i.e. manually created
	// replication slots will be removed).
	AdditionalMasterReplicationSlots []string `json:"additionalMasterReplicationSlots"`
	// Whether to use pg_rewind
	UsePgrewind *bool `json:"usePgrewind,omitempty"`
	// InitMode defines the cluster initialization mode. Current modes are: new, existing, pitr
	InitMode *ClusterInitMode `json:"initMode,omitempty"`
	// Whether to merge pgParameters of the initialized db cluster, useful
	// the retain initdb generated parameters when InitMode is new, retain
	// current parameters when initMode is existing or pitr.
	MergePgParameters *bool `json:"mergePgParameters,omitempty"`
	// Role defines the cluster operating role (master or standby of an external database)
	Role *ClusterRole `json:"role,omitempty"`
	// Init configuration used when InitMode is "new"
	NewConfig *NewConfig `json:"newConfig,omitempty"`
	// Point in time recovery init configuration used when InitMode is "pitr"
	PITRConfig *PITRConfig `json:"pitrConfig,omitempty"`
	// Existing init configuration used when InitMode is "existing"
	ExistingConfig *ExistingConfig `json:"existingConfig,omitempty"`
	// Standby config when role is standby
	StandbyConfig *StandbyConfig `json:"standbyConfig,omitempty"`
	// Define the mode of the default hba rules needed for replication by standby keepers (the su and repl auth methods will be the one provided in the keeper command line options)
	// Values can be "all" or "strict", "all" allow access from all ips, "strict" restrict master access to standby servers ips.
	// Default is "all"
	DefaultSUReplAccessMode *SUReplAccessMode `json:"defaultSUReplAccessMode,omitempty"`
	// Map of postgres parameters
	PGParameters PGParameters `json:"pgParameters,omitempty"`
	// Additional pg_hba.conf entries
	// we don't set omitempty since we want to distinguish between null or empty slice
	PGHBA []string `json:"pgHBA"`
	// Enable automatic pg restart when pg parameters that requires restart changes
	AutomaticPgRestart *bool `json:"automaticPgRestart"`
}
type NewConfig struct {
	Locale        string `json:"locale,omitempty"`
	Encoding      string `json:"encoding,omitempty"`
	DataChecksums bool   `json:"dataChecksums,omitempty"`
}

type PITRConfig struct {
	// DataRestoreCommand defines the command to execute for restoring the db
	// cluster data). %d is replaced with the full path to the db cluster
	// datadir. Use %% to embed an actual % character.
	DataRestoreCommand      string                   `json:"dataRestoreCommand,omitempty"`
	ArchiveRecoverySettings *ArchiveRecoverySettings `json:"archiveRecoverySettings,omitempty"`
	RecoveryTargetSettings  *RecoveryTargetSettings  `json:"recoveryTargetSettings,omitempty"`
}

type ExistingConfig struct {
	KeeperUID string `json:"keeperUID,omitempty"`
}

// Standby config when role is standby
type StandbyConfig struct {
	StandbySettings         *StandbySettings         `json:"standbySettings,omitempty"`
	ArchiveRecoverySettings *ArchiveRecoverySettings `json:"archiveRecoverySettings,omitempty"`
}

type ClusterInitMode string

// ArchiveRecoverySettings defines the archive recovery settings in the recovery.conf file (https://www.postgresql.org/docs/9.6/static/archive-recovery-settings.html )
type ArchiveRecoverySettings struct {
	// value for restore_command
	RestoreCommand string `json:"restoreCommand,omitempty"`
}

// RecoveryTargetSettings defines the recovery target settings in the recovery.conf file (https://www.postgresql.org/docs/9.6/static/recovery-target-settings.html )
type RecoveryTargetSettings struct {
	RecoveryTarget         string `json:"recoveryTarget,omitempty"`
	RecoveryTargetLsn      string `json:"recoveryTargetLsn,omitempty"`
	RecoveryTargetName     string `json:"recoveryTargetName,omitempty"`
	RecoveryTargetTime     string `json:"recoveryTargetTime,omitempty"`
	RecoveryTargetXid      string `json:"recoveryTargetXid,omitempty"`
	RecoveryTargetTimeline string `json:"recoveryTargetTimeline,omitempty"`
}

// StandbySettings defines the standby settings in the recovery.conf file (https://www.postgresql.org/docs/9.6/static/standby-settings.html )
type StandbySettings struct {
	PrimaryConninfo       string `json:"primaryConninfo,omitempty"`
	PrimarySlotName       string `json:"primarySlotName,omitempty"`
	RecoveryMinApplyDelay string `json:"recoveryMinApplyDelay,omitempty"`
}

type SUReplAccessMode string

const (
	// Allow access from every host
	SUReplAccessAll SUReplAccessMode = "all"
	// Allow access from standby server IPs only
	SUReplAccessStrict SUReplAccessMode = "strict"
)

func SUReplAccessModeP(s SUReplAccessMode) *SUReplAccessMode {
	return &s
}

const (
	// Initialize a cluster starting from a freshly initialized database cluster. Valid only when cluster role is master.
	ClusterInitModeNew ClusterInitMode = "new"
	// Initialize a cluster doing a point in time recovery on a keeper.
	ClusterInitModePITR ClusterInitMode = "pitr"
	// Initialize a cluster with an user specified already populated db cluster.
	ClusterInitModeExisting ClusterInitMode = "existing"
)

// Duration is needed to be able to marshal/unmarshal json strings with time
// unit (eg. 3s, 100ms) instead of ugly times in nanoseconds.
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	du, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = du
	return nil
}

type ClusterRole string

const (
	ClusterRoleMaster  ClusterRole = "master"
	ClusterRoleStandby ClusterRole = "standby"
)

type PGParameters map[string]string

type ClusterStatus struct {
	CurrentGeneration int64        `json:"currentGeneration,omitempty"`
	Phase             ClusterPhase `json:"phase,omitempty"`
	// Master DB UID
	Master string `json:"master,omitempty"`
}

type ClusterPhase string

const (
	ClusterPhaseInitializing ClusterPhase = "initializing"
	ClusterPhaseNormal       ClusterPhase = "normal"
)

type Keepers map[string]*Keeper
type DBs map[string]*DB

type KeeperSpec struct{}

type KeeperStatus struct {
	Healthy         bool      `json:"healthy,omitempty"`
	LastHealthyTime time.Time `json:"lastHealthyTime,omitempty"`

	BootUUID string `json:"bootUUID,omitempty"`

	PostgresBinaryVersion PostgresBinaryVersion `json:"postgresBinaryVersion,omitempty"`

	ForceFail bool `json:"forceFail,omitempty"`

	CanBeMaster             *bool `json:"canBeMaster,omitempty"`
	CanBeSynchronousReplica *bool `json:"canBeSynchronousReplica,omitempty"`
}

type Keeper struct {
	// Keeper ID
	UID        string    `json:"uid,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	ChangeTime time.Time `json:"changeTime,omitempty"`

	Spec *KeeperSpec `json:"spec,omitempty"`

	Status KeeperStatus `json:"status,omitempty"`
}

type PostgresBinaryVersion struct {
	Maj int
	Min int
}

type DB struct {
	UID        string    `json:"uid,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	ChangeTime time.Time `json:"changeTime,omitempty"`

	Spec *DBSpec `json:"spec,omitempty"`

	Status DBStatus `json:"status,omitempty"`
}
type DBSpec struct {
	// The KeeperUID this db is assigned to
	KeeperUID string `json:"keeperUID,omitempty"`
	// Time after which any request (keepers checks from sentinel etc...) will fail.
	RequestTimeout Duration `json:"requestTimeout,omitempty"`
	// See ClusterSpec MaxStandbys description
	MaxStandbys uint16 `json:"maxStandbys,omitempty"`
	// Use Synchronous replication between master and its standbys
	SynchronousReplication bool `json:"synchronousReplication,omitempty"`
	// Whether to use pg_rewind
	UsePgrewind bool `json:"usePgrewind,omitempty"`
	// AdditionalWalSenders defines the number of additional wal_senders in
	// addition to the ones internally defined by stolon
	AdditionalWalSenders uint16 `json:"additionalWalSenders"`
	// AdditionalReplicationSlots is a list of additional replication slots.
	// Replication slots not defined here will be dropped from the instance
	// (i.e. manually created replication slots will be removed).
	AdditionalReplicationSlots []string `json:"additionalReplicationSlots"`
	// InitMode defines the db initialization mode. Current modes are: none, new
	InitMode DBInitMode `json:"initMode,omitempty"`
	// Init configuration used when InitMode is "new"
	NewConfig *NewConfig `json:"newConfig,omitempty"`
	// Point in time recovery init configuration used when InitMode is "pitr"
	PITRConfig *PITRConfig `json:"pitrConfig,omitempty"`
	// Map of postgres parameters
	PGParameters PGParameters `json:"pgParameters,omitempty"`
	// Additional pg_hba.conf entries
	// We don't set omitempty since we want to distinguish between null or empty slice
	PGHBA []string `json:"pgHBA"`
	// DB Role (master or standby)
	Role string `json:"role,omitempty"`
	// FollowConfig when Role is "standby"
	FollowConfig *FollowConfig `json:"followConfig,omitempty"`
	// Followers DB UIDs
	Followers []string `json:"followers"`
	// Whether to include previous postgresql.conf
	IncludeConfig bool `json:"includePreviousConfig,omitempty"`
	// SynchronousStandbys are the standbys to be configured as synchronous
	SynchronousStandbys []string `json:"synchronousStandbys"`
	// External SynchronousStandbys are external standbys names to be configured as synchronous
	ExternalSynchronousStandbys []string `json:"externalSynchronousStandbys"`
}

type DBStatus struct {
	Healthy bool `json:"healthy,omitempty"`

	CurrentGeneration int64 `json:"currentGeneration,omitempty"`

	ListenAddress string `json:"listenAddress,omitempty"`
	Port          string `json:"port,omitempty"`

	SystemID         string                   `json:"systemdID,omitempty"`
	TimelineID       uint64                   `json:"timelineID,omitempty"`
	XLogPos          uint64                   `json:"xLogPos,omitempty"`
	TimelinesHistory PostgresTimelinesHistory `json:"timelinesHistory,omitempty"`

	PGParameters PGParameters `json:"pgParameters,omitempty"`

	// DBUIDs of the internal standbys currently reported as in sync by the instance
	CurSynchronousStandbys []string `json:"-"`

	// DBUIDs of the internal standbys that we know are in sync.
	// They could be currently down but we know that they were reported as in
	// sync in the past and they are defined inside synchronous_standby_names
	// so the instance will wait for acknowledge from them.
	SynchronousStandbys []string `json:"synchronousStandbys"`

	// NOTE(sgotti) we currently don't report the external synchronous standbys.
	// If/when needed lets add a new ExternalSynchronousStandbys field

	OlderWalFile string `json:"olderWalFile,omitempty"`
}

type DBInitMode string

const (
	DBInitModeNone DBInitMode = "none"
	// Use existing db cluster data
	DBInitModeExisting DBInitMode = "existing"
	// Initialize a db starting from a freshly initialized database cluster
	DBInitModeNew DBInitMode = "new"
	// Initialize a db doing a point in time recovery
	DBInitModePITR DBInitMode = "pitr"
	// Initialize a db doing a resync to a target database cluster
	DBInitModeResync DBInitMode = "resync"
)

type PostgresTimelinesHistory []*PostgresTimelineHistory

type PostgresTimelineHistory struct {
	TimelineID  uint64 `json:"timelineID,omitempty"`
	SwitchPoint uint64 `json:"switchPoint,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type FollowType string

const (
	// Follow an db managed by a keeper in our cluster
	FollowTypeInternal FollowType = "internal"
	// Follow an external db
	FollowTypeExternal FollowType = "external"
)

type FollowConfig struct {
	Type FollowType `json:"type,omitempty"`
	// Keeper ID to follow when Type is "internal"
	DBUID string `json:"dbuid,omitempty"`
	// Standby settings when Type is "external"
	StandbySettings         *StandbySettings         `json:"standbySettings,omitempty"`
	ArchiveRecoverySettings *ArchiveRecoverySettings `json:"archiveRecoverySettings,omitempty"`
}

type Proxy struct {
	UID        string    `json:"uid,omitempty"`
	Generation int64     `json:"generation,omitempty"`
	ChangeTime time.Time `json:"changeTime,omitempty"`

	Spec ProxySpec `json:"spec,omitempty"`

	Status ProxyStatus `json:"status,omitempty"`
}

type ProxySpec struct {
	MasterDBUID    string   `json:"masterDbUid,omitempty"`
	EnabledProxies []string `json:"enabledProxies,omitempty"`
}

type ProxyStatus struct {
}
