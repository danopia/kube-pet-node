package podman

import (
	"context"
	"encoding/json"
)

func (pc *PodmanClient) ContainerCreate(ctx context.Context, spec *SpecGenerator) (*ContainerCreateReport, error) {
	response, err := pc.performPost(ctx, "/libpod/containers/create", spec)
	if err != nil {
		return nil, err
	}

	var out ContainerCreateReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
}
type ContainerCreateReport struct {
	Id string
	Warnings []string
}

// ContainerAttach(ctx context.Context, nameOrID string, options AttachOptions) error
// ContainerCheckpoint(ctx context.Context, namesOrIds []string, options CheckpointOptions) ([]*CheckpointReport, error)
// ContainerCleanup(ctx context.Context, namesOrIds []string, options ContainerCleanupOptions) ([]*ContainerCleanupReport, error)
// ContainerCommit(ctx context.Context, nameOrID string, options CommitOptions) (*CommitReport, error)
// ContainerCp(ctx context.Context, source, dest string, options ContainerCpOptions) (*ContainerCpReport, error)
// ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*ContainerCreateReport, error)
// ContainerDiff(ctx context.Context, nameOrID string, options DiffOptions) (*DiffReport, error)
// ContainerExec(ctx context.Context, nameOrID string, options ExecOptions, streams define.AttachStreams) (int, error)
// ContainerExecDetached(ctx context.Context, nameOrID string, options ExecOptions) (string, error)
// ContainerExists(ctx context.Context, nameOrID string) (*BoolReport, error)
// ContainerExport(ctx context.Context, nameOrID string, options ContainerExportOptions) error
// ContainerInit(ctx context.Context, namesOrIds []string, options ContainerInitOptions) ([]*ContainerInitReport, error)
// ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, []error, error)
// ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
// ContainerList(ctx context.Context, options ContainerListOptions) ([]ListContainer, error)
// ContainerLogs(ctx context.Context, containers []string, options ContainerLogsOptions) error
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
