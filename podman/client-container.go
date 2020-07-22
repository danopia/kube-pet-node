package podman

import (
	"context"
	"io"
	"time"
)

// ContainerAttach(ctx context.Context, nameOrID string, options AttachOptions) error

// ContainerCheckpoint(ctx context.Context, namesOrIds []string, options CheckpointOptions) ([]*CheckpointReport, error)

// ContainerCleanup(ctx context.Context, namesOrIds []string, options ContainerCleanupOptions) ([]*ContainerCleanupReport, error)

// ContainerCommit(ctx context.Context, nameOrID string, options CommitOptions) (*CommitReport, error)

// ContainerCp(ctx context.Context, source, dest string, options ContainerCpOptions) (*ContainerCpReport, error)

// ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*ContainerCreateReport, error)
func (pc *PodmanClient) ContainerCreate(ctx context.Context, spec *SpecGenerator) (*ContainerCreateReport, error) {
	var out ContainerCreateReport
	return &out, pc.performPost(ctx, "/libpod/containers/create", spec, &out)
}

type ContainerCreateReport struct {
	Id       string
	Warnings []string
}

// ContainerDiff(ctx context.Context, nameOrID string, options DiffOptions) (*DiffReport, error)

/// ContainerExec(ctx context.Context, nameOrID string, options ExecOptions, streams define.AttachStreams) (int, error)

/// ContainerExecDetached(ctx context.Context, nameOrID string, options ExecOptions) (string, error)

// ContainerExists(ctx context.Context, nameOrID string) (*BoolReport, error)

// ContainerExport(ctx context.Context, nameOrID string, options ContainerExportOptions) error

// ContainerInit(ctx context.Context, namesOrIds []string, options ContainerInitOptions) ([]*ContainerInitReport, error)

// ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, []error, error)
func (pc *PodmanClient) ContainerInspect(ctx context.Context, nameOrId string, includeSize bool) (*InspectContainerData, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}
	var flags string
	if includeSize {
		flags = "?size=true"
	}

	var out InspectContainerData
	return &out, pc.performGet(ctx, "/libpod/containers/"+encoded+"/json"+flags, &out)
}

// these types are from https://github.com/containers/libpod/blob/957e7a533efe2d314640afac9d3b31cc752edd80/libpod/define/container_inspect.go
type InspectContainerData struct {
	ID              string                 `json:"Id"`
	Created         time.Time              `json:"Created"`
	Path            string                 `json:"Path"`
	Args            []string               `json:"Args"`
	State           *InspectContainerState `json:"State"`
	Image           string                 `json:"Image"`
	ImageName       string                 `json:"ImageName"`
	Rootfs          string                 `json:"Rootfs"`
	Pod             string                 `json:"Pod"`
	ResolvConfPath  string                 `json:"ResolvConfPath"`
	HostnamePath    string                 `json:"HostnamePath"`
	HostsPath       string                 `json:"HostsPath"`
	StaticDir       string                 `json:"StaticDir"`
	OCIConfigPath   string                 `json:"OCIConfigPath,omitempty"`
	OCIRuntime      string                 `json:"OCIRuntime,omitempty"`
	LogPath         string                 `json:"LogPath"`
	LogTag          string                 `json:"LogTag"`
	ConmonPidFile   string                 `json:"ConmonPidFile"`
	Name            string                 `json:"Name"`
	RestartCount    int32                  `json:"RestartCount"`
	Driver          string                 `json:"Driver"`
	MountLabel      string                 `json:"MountLabel"`
	ProcessLabel    string                 `json:"ProcessLabel"`
	AppArmorProfile string                 `json:"AppArmorProfile"`
	EffectiveCaps   []string               `json:"EffectiveCaps"`
	BoundingCaps    []string               `json:"BoundingCaps"`
	ExecIDs         []string               `json:"ExecIDs"`
	// GraphDriver     *driver.Data                `json:"GraphDriver"`
	SizeRw     *int64 `json:"SizeRw,omitempty"`
	SizeRootFs int64  `json:"SizeRootFs,omitempty"`
	// Mounts          []InspectMount              `json:"Mounts"`
	Dependencies    []string                `json:"Dependencies"`
	NetworkSettings *InspectNetworkSettings `json:"NetworkSettings"` //TODO
	ExitCommand     []string                `json:"ExitCommand"`
	Namespace       string                  `json:"Namespace"`
	IsInfra         bool                    `json:"IsInfra"`
	Config          *InspectContainerConfig `json:"Config"`
	// HostConfig      *InspectContainerHostConfig `json:"HostConfig"`
}
type InspectContainerState struct {
	OciVersion  string             `json:"OciVersion"`
	Status      string             `json:"Status"`
	Running     bool               `json:"Running"`
	Paused      bool               `json:"Paused"`
	Restarting  bool               `json:"Restarting"` // TODO
	OOMKilled   bool               `json:"OOMKilled"`
	Dead        bool               `json:"Dead"`
	Pid         int                `json:"Pid"`
	ConmonPid   int                `json:"ConmonPid,omitempty"`
	ExitCode    int32              `json:"ExitCode"`
	Error       string             `json:"Error"` // TODO
	StartedAt   time.Time          `json:"StartedAt"`
	FinishedAt  time.Time          `json:"FinishedAt"`
	Healthcheck HealthCheckResults `json:"Healthcheck,omitempty"`
}
type HealthCheckResults struct {
	Status        string           `json:"Status"`
	FailingStreak int              `json:"FailingStreak"`
	Log           []HealthCheckLog `json:"Log"`
}
type HealthCheckLog struct {
	Start    string `json:"Start"`
	End      string `json:"End"`
	ExitCode int    `json:"ExitCode"`
	Output   string `json:"Output"`
}
type InspectContainerConfig struct {
	Hostname     string              `json:"Hostname"`
	DomainName   string              `json:"Domainname"`
	User         string              `json:"User"`
	AttachStdin  bool                `json:"AttachStdin"`
	AttachStdout bool                `json:"AttachStdout"`
	AttachStderr bool                `json:"AttachStderr"`
	Tty          bool                `json:"Tty"`
	OpenStdin    bool                `json:"OpenStdin"`
	StdinOnce    bool                `json:"StdinOnce"`
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes"`
	WorkingDir   string              `json:"WorkingDir"`
	Entrypoint   string              `json:"Entrypoint"`
	OnBuild      *string             `json:"OnBuild"`
	Labels       map[string]string   `json:"Labels"`
	Annotations  map[string]string   `json:"Annotations"`
	StopSignal   uint                `json:"StopSignal"`
	// Healthcheck *manifest.Schema2HealthConfig `json:"Healthcheck,omitempty"`
	CreateCommand []string `json:"CreateCommand,omitempty"`
}
type InspectBasicNetworkConfig struct {
	EndpointID             string   `json:"EndpointID"`
	Gateway                string   `json:"Gateway"`
	IPAddress              string   `json:"IPAddress"`
	IPPrefixLen            int      `json:"IPPrefixLen"`
	SecondaryIPAddresses   []string `json:"SecondaryIPAddresses,omitempty"`
	IPv6Gateway            string   `json:"IPv6Gateway"`
	GlobalIPv6Address      string   `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen    int      `json:"GlobalIPv6PrefixLen"`
	SecondaryIPv6Addresses []string `json:"SecondaryIPv6Addresses,omitempty"`
	MacAddress             string   `json:"MacAddress"`
	AdditionalMacAddresses []string `json:"AdditionalMACAddresses,omitempty"`
}
type InspectAdditionalNetwork struct {
	InspectBasicNetworkConfig
	NetworkID  string            `json:"NetworkID,omitempty"`
	DriverOpts map[string]string `json:"DriverOpts"`
	IPAMConfig map[string]string `json:"IPAMConfig"`
	Links      []string          `json:"Links"`
}
type InspectNetworkSettings struct {
	InspectBasicNetworkConfig
	Bridge                 string                               `json:"Bridge"`
	SandboxID              string                               `json:"SandboxID"`
	HairpinMode            bool                                 `json:"HairpinMode"`
	LinkLocalIPv6Address   string                               `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int                                  `json:"LinkLocalIPv6PrefixLen"`
	Ports                  map[string][]InspectHostPort         `json:"Ports"`
	SandboxKey             string                               `json:"SandboxKey"`
	Networks               map[string]*InspectAdditionalNetwork `json:"Networks,omitempty"`
}
type InspectHostPort struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)

// ContainerList(ctx context.Context, options ContainerListOptions) ([]ListContainer, error)

// ContainerLogs(ctx context.Context, containers []string, options ContainerLogsOptions) error
type ContainerLogsOptions struct {
	Follow bool
	// Stdout bool
	// Stderr bool
	Since      string
	Until      string
	Timestamps bool
	Tail       string
}

func (pc *PodmanClient) ContainerLogs(ctx context.Context, nameOrId string, options *ContainerLogsOptions) (io.ReadCloser, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}
	flags := "?stdout=true&stderr=true"
	if options.Follow {
		flags += "&follow=true"
	}
	if options.Timestamps {
		flags += "&timestamps=true"
	}
	if options.Since != "" {
		flags += "&since=" + options.Since
	}
	if options.Until != "" {
		flags += "&until=" + options.Until
	}
	if options.Tail != "" {
		flags += "&tail=" + options.Tail
	}

	resp, err := pc.performRawRequest(ctx, "GET", "/libpod/containers/"+encoded+"/logs"+flags)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ContainerMount(ctx context.Context, nameOrIDs []string, options ContainerMountOptions) ([]*ContainerMountReport, error)

// ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)

// ContainerPort(ctx context.Context, nameOrID string, options ContainerPortOptions) ([]*ContainerPortReport, error)

// ContainerPrune(ctx context.Context, options ContainerPruneOptions) (*ContainerPruneReport, error)

// ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)

// ContainerRestore(ctx context.Context, namesOrIds []string, options RestoreOptions) ([]*RestoreReport, error)

// ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)

// ContainerRun(ctx context.Context, opts ContainerRunOptions) (*ContainerRunReport, error)

// ContainerRunlabel(ctx context.Context, label string, image string, args []string, opts ContainerRunlabelOptions) error

// ContainerStart(ctx context.Context, namesOrIds []string, options ContainerStartOptions) ([]*ContainerStartReport, error)

// ContainerStats(ctx context.Context, namesOrIds []string, options ContainerStatsOptions) error

// ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)

// ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)

// ContainerUnmount(ctx context.Context, nameOrIDs []string, options ContainerUnmountOptions) ([]*ContainerUnmountReport, error)

// ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)

// ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
