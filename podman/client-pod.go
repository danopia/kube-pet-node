package podman

import (
	"context"
	"encoding/json"
)

type PodActionReport struct {
	Id   string
	Errs []string
}

// PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
func (pc *PodmanClient) PodCreate(ctx context.Context, spec PodSpecGenerator) (*PodCreateReport, error) {
	response, err := pc.performPost(ctx, "/libpod/pods/create", spec)
	if err != nil {
		return nil, err
	}

	var out PodCreateReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
}

type PodCreateReport struct {
	Id string
}

// PodExists(ctx context.Context, nameOrID string) (*BoolReport, error)

// PodInspect(ctx context.Context, options PodInspectOptions) (*PodInspectReport, error)

// PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)

// PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)

// PodPrune(ctx context.Context, options PodPruneOptions) ([]*PodPruneReport, error)

// PodPs(ctx context.Context, options PodPSOptions) ([]*ListPodsReport, error)
func (pc *PodmanClient) PodPs(ctx context.Context) ([]*ListPodsReport, error) {
	response, err := pc.performGet(ctx, "/libpod/pods/json")
	if err != nil {
		return nil, err
	}

	var out []*ListPodsReport
	return out, json.NewDecoder(response.Body).Decode(&out)
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

	response, err := pc.performPost(ctx, "/libpod/pods/"+encoded+"/restart", nil)
	if err != nil {
		return nil, err
	}

	var out PodActionReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
}

// PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
func (pc *PodmanClient) PodRm(ctx context.Context, nameOrId string) (*PodRmReport, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	response, err := pc.performDelete(ctx, "/libpod/pods/"+encoded)
	if err != nil {
		return nil, err
	}

	var out PodRmReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
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

	response, err := pc.performPost(ctx, "/libpod/pods/"+encoded+"/start", nil)
	if err != nil {
		return nil, err
	}

	var out PodActionReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
}

// PodStats(ctx context.Context, namesOrIds []string, options PodStatsOptions) ([]*PodStatsReport, error)

// PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
func (pc *PodmanClient) PodStop(ctx context.Context, nameOrId string) (*PodActionReport, error) {
	// TODO: ?t= timeout secs
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	response, err := pc.performPost(ctx, "/libpod/pods/"+encoded+"/stop", nil)
	if err != nil {
		return nil, err
	} else if response.StatusCode == 304 {
		return &PodActionReport{
			Errs: []string{"Already Stopped"},
		}, nil
	}

	var out PodActionReport
	return &out, json.NewDecoder(response.Body).Decode(&out)
}

// PodTop(ctx context.Context, options PodTopOptions) (*StringSliceReport, error)

// PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)
