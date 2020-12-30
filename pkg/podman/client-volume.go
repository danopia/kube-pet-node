package podman

import (
	"context"
	"encoding/json"
	"time"
)

type VolumeInfo struct {
	CreatedAt  time.Time
	Name       string
	Driver     string
	Mountpoint string
	Labels     map[string]string
	Options    map[string]string
	Anonymous  bool
	UID        int
	GID        int
	Scope      string
}

// VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IDOrNameResponse, error)
type VolumeCreateOptions struct {
	Name    string
	Driver  string
	Label   map[string]string
	Options map[string]string
}

func (pc *PodmanClient) VolumeCreate(ctx context.Context, opts *VolumeCreateOptions) (*VolumeInfo, error) {
	var out VolumeInfo
	return &out, pc.performPost(ctx, "/libpod/volumes/create", opts, &out)
}

// VolumeInspect(ctx context.Context, namesOrIds []string, opts VolumeInspectOptions) ([]*VolumeInspectReport, error)
func (pc *PodmanClient) VolumeInspect(ctx context.Context, nameOrId string) (*VolumeInfo, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out VolumeInfo
	return &out, pc.performGet(ctx, "/libpod/volumes/"+encoded+"/json", &out)
}

// VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
func (pc *PodmanClient) VolumeList(ctx context.Context, filters map[string][]string) ([]VolumeInfo, error) {
	serialized, err := json.Marshal(filters)
	if err != nil {
		return nil, err
	}
	encoded, err := UrlEncoded(string(serialized))
	if err != nil {
		return nil, err
	}

	var out []VolumeInfo
	return out, pc.performGet(ctx, "/libpod/volumes/json?filters="+encoded, &out)
}

// VolumePrune(ctx context.Context, opts VolumePruneOptions) ([]*VolumePruneReport, error)

// VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
func (pc *PodmanClient) VolumeRm(ctx context.Context, nameOrId string, force bool) error {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return err
	}
	if force {
		encoded += "?force=true"
	}

	return pc.performDelete(ctx, "/libpod/volumes/"+encoded, nil)
}
