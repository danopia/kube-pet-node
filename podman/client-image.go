package podman

import (
	"context"
	"encoding/json"
	"time"
)

// Build(ctx context.Context, containerFiles []string, opts BuildOptions) (*BuildReport, error)

// Config(ctx context.Context) (*config.Config, error)

// Diff(ctx context.Context, nameOrID string, options DiffOptions) (*DiffReport, error)

// Exists(ctx context.Context, nameOrID string) (*BoolReport, error)

// History(ctx context.Context, nameOrID string, opts ImageHistoryOptions) (*ImageHistoryReport, error)

// Import(ctx context.Context, opts ImageImportOptions) (*ImageImportReport, error)

// Inspect(ctx context.Context, namesOrIDs []string, opts InspectOptions) ([]*ImageInspectReport, []error, error)

// List(ctx context.Context, opts ImageListOptions) ([]*ImageSummary, error)
func (pc *PodmanClient) List(ctx context.Context) ([]ImageSummary, error) {
	response, err := pc.performGet(ctx, "/libpod/images/json")
	if err != nil {
		return nil, err
	}

	var out []ImageSummary
	return out, json.NewDecoder(response.Body).Decode(&out)
}

type ImageSummary struct {
	ID          string            `json:"Id"`
	ParentId    string            `json:",omitempty"`
	RepoTags    []string          `json:",omitempty"`
	Created     time.Time         `json:",omitempty"`
	Size        int64             `json:",omitempty"`
	SharedSize  int               `json:",omitempty"`
	VirtualSize int64             `json:",omitempty"`
	Labels      map[string]string `json:",omitempty"`
	Containers  int               `json:",omitempty"`
	ReadOnly    bool              `json:",omitempty"`
	Dangling    bool              `json:",omitempty"`

	// Podman extensions
	Names        []string `json:",omitempty"`
	Digest       string   `json:",omitempty"`
	Digests      []string `json:",omitempty"`
	ConfigDigest string   `json:",omitempty"`
	History      []string `json:",omitempty"`
}

// Load(ctx context.Context, opts ImageLoadOptions) (*ImageLoadReport, error)

// Prune(ctx context.Context, opts ImagePruneOptions) (*ImagePruneReport, error)

// Pull(ctx context.Context, rawImage string, opts ImagePullOptions) (*ImagePullReport, error)

// Push(ctx context.Context, source string, destination string, opts ImagePushOptions) error

// Remove(ctx context.Context, images []string, opts ImageRemoveOptions) (*ImageRemoveReport, []error)

// Save(ctx context.Context, nameOrID string, tags []string, options ImageSaveOptions) error

// Search(ctx context.Context, term string, opts ImageSearchOptions) ([]ImageSearchReport, error)

// SetTrust(ctx context.Context, args []string, options SetTrustOptions) error

// ShowTrust(ctx context.Context, args []string, options ShowTrustOptions) (*ShowTrustReport, error)

// Shutdown(ctx context.Context)

// Tag(ctx context.Context, nameOrID string, tags []string, options ImageTagOptions) error

// Tree(ctx context.Context, nameOrID string, options ImageTreeOptions) (*ImageTreeReport, error)

// Untag(ctx context.Context, nameOrID string, tags []string, options ImageUntagOptions) error

// ManifestCreate(ctx context.Context, names, images []string, opts ManifestCreateOptions) (string, error)

// ManifestInspect(ctx context.Context, name string) ([]byte, error)

// ManifestAdd(ctx context.Context, opts ManifestAddOptions) (string, error)

// ManifestAnnotate(ctx context.Context, names []string, opts ManifestAnnotateOptions) (string, error)

// ManifestRemove(ctx context.Context, names []string) (string, error)

// ManifestPush(ctx context.Context, names []string, manifestPushOpts ManifestPushOptions) error

// Sign(ctx context.Context, names []string, options SignOptions) (*SignReport, error)
