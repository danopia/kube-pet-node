package podman

import (
	"context"
)

// AutoUpdate(ctx context.Context, options AutoUpdateOptions) (*AutoUpdateReport, []error)

// Config(ctx context.Context) (*config.Config, error)

// Events(ctx context.Context, opts EventsOptions) error

// GenerateSystemd(ctx context.Context, nameOrID string, opts GenerateSystemdOptions) (*GenerateSystemdReport, error)

// GenerateKube(ctx context.Context, nameOrID string, opts GenerateKubeOptions) (*GenerateKubeReport, error)

// SystemPrune(ctx context.Context, options SystemPruneOptions) (*SystemPruneReport, error)

// HealthCheckRun(ctx context.Context, nameOrID string, options HealthCheckOptions) (*define.HealthCheckResults, error)

// Info(ctx context.Context) (*define.Info, error)

// PlayKube(ctx context.Context, path string, opts PlayKubeOptions) (*PlayKubeReport, error)

// SetupRootless(ctx context.Context, cmd *cobra.Command) error

// Shutdown(ctx context.Context)

// SystemDf(ctx context.Context, options SystemDfOptions) (*SystemDfReport, error)

// Unshare(ctx context.Context, args []string) error

// VarlinkService(ctx context.Context, opts ServiceOptions) error

// Version(ctx context.Context) (*SystemVersionReport, error)
func (pc *PodmanClient) Version(ctx context.Context) (*DockerVersionReport, error) {
	var out DockerVersionReport
	return &out, pc.performGet(ctx, "/libpod/version", &out)
}

type DockerVersionReport struct {
	Platform   DockerVersionPlatform
	Components []DockerVersionComponent

	APIVersion    string
	Arch          string
	BuildTime     string
	Experimental  bool
	GitCommit     string
	GoVersion     string
	KernelVersion string
	MinAPIVersion string
	Os            string
	Version       string
}
type DockerVersionPlatform struct {
	Name string
}
type DockerVersionComponent struct {
	Name    string
	Version string
	Details map[string]string
}

// Renumber(ctx context.Context, flags *pflag.FlagSet, config *PodmanConfig) error

// Migrate(ctx context.Context, flags *pflag.FlagSet, config *PodmanConfig, options SystemMigrateOptions) error

// Reset(ctx context.Context) error

// Shutdown(ctx context.Context)
