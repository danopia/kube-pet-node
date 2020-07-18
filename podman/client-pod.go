package podman

import (
	"context"
	"time"
)

type PodActionReport struct {
	Id   string
	Errs []string
}

// PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
func (pc *PodmanClient) PodCreate(ctx context.Context, spec *PodSpecGenerator) (*PodCreateReport, error) {
	var out PodCreateReport
	return &out, pc.performPost(ctx, "/libpod/pods/create", spec, &out)
}

type PodCreateReport struct {
	Id string
}

// PodExists(ctx context.Context, nameOrID string) (*BoolReport, error)

// PodInspect(ctx context.Context, options PodInspectOptions) (*PodInspectReport, error)
func (pc *PodmanClient) PodInspect(ctx context.Context, nameOrId string) (*InspectPodData, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out InspectPodData
	return &out, pc.performGet(ctx, "/libpod/pods/"+encoded+"/json", &out)
}

type InspectPodData struct {
	ID               string `json:"Id"`
	Name             string
	Namespace        string `json:"Namespace,omitempty"`
	Created          time.Time
	CreateCommand    []string `json:"CreateCommand,omitempty"`
	State            string   `json:"State"`
	Hostname         string
	Labels           map[string]string `json:"Labels,omitempty"`
	CgroupParent     string            `json:"CgroupParent,omitempty"`
	CgroupPath       string            `json:"CgroupPath,omitempty"`
	InfraContainerID string            `json:"InfraContainerID,omitempty"`
	SharedNamespaces []string          `json:"SharedNamespaces,omitempty"`
	NumContainers    uint
	Containers       []InspectPodContainerInfo `json:"Containers,omitempty"`
}
type InspectPodContainerInfo struct {
	ID    string `json:"Id"`
	Name  string
	State string
}

// PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)

// PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)

// PodPrune(ctx context.Context, options PodPruneOptions) ([]*PodPruneReport, error)

// PodPs(ctx context.Context, options PodPSOptions) ([]*ListPodsReport, error)
func (pc *PodmanClient) PodPs(ctx context.Context) ([]*ListPodsReport, error) {
	var out []*ListPodsReport
	return out, pc.performGet(ctx, "/libpod/pods/json", &out)
}

type ListPodsReport struct {
	Cgroup     string
	Containers []ListPodsContainer
	Created    string
	Id         string
	InfraId    string
	Name       string
	Namespace  string
	Status     string
	Labels     map[string]string
}
type ListPodsContainer struct {
	Id     string
	Names  string
	Status string
}

// PodRestart(ctx context.Context, namesOrIds []string, options PodRestartOptions) ([]*PodRestartReport, error)
func (pc *PodmanClient) PodRestart(ctx context.Context, nameOrId string) (*PodActionReport, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out PodActionReport
	return &out, pc.performPost(ctx, "/libpod/pods/"+encoded+"/restart", nil, &out)
}

// PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
func (pc *PodmanClient) PodRm(ctx context.Context, nameOrId string, force bool) (*PodRmReport, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}
	if force {
		encoded += "?force=true"
	}

	var out PodRmReport
	return &out, pc.performDelete(ctx, "/libpod/pods/"+encoded, &out)
}

type PodRmReport struct {
	Id  string
	Err string
}

// PodStart(ctx context.Context, namesOrIds []string, options PodStartOptions) ([]*PodStartReport, error)
func (pc *PodmanClient) PodStart(ctx context.Context, nameOrId string) (*PodActionReport, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out PodActionReport
	return &out, pc.performPost(ctx, "/libpod/pods/"+encoded+"/start", nil, &out)
}

// PodStats(ctx context.Context, namesOrIds []string, options PodStatsOptions) ([]*PodStatsReport, error)

// PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
func (pc *PodmanClient) PodStop(ctx context.Context, nameOrId string) (*PodActionReport, error) {
	// TODO: ?t= timeout secs
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out PodActionReport
	if err := pc.performPost(ctx, "/libpod/pods/"+encoded+"/stop", nil, &out); err != nil {
		if err, ok := err.(*ApiError); ok {
			if err.Status == 304 {
				return &PodActionReport{
					Errs: []string{"Already Stopped"},
				}, nil
			}
		}
		return nil, err
	}
	return &out, nil
}

// PodTop(ctx context.Context, options PodTopOptions) (*StringSliceReport, error)

// PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)
