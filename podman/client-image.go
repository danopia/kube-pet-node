package podman

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
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
	var out []ImageSummary
	return out, pc.performGet(ctx, "/libpod/images/json", &out)
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
func (pc *PodmanClient) Pull(ctx context.Context, reference string, authConfig []byte) (<-chan ImagePullReport, error) {
	encoded, err := UrlEncoded(reference)
	if err != nil {
		return nil, err
	}
	path := "/libpod/images/pull?reference=" + encoded

	req, err := http.NewRequestWithContext(ctx, "POST", "http://podman/v1.0.0"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")

	// Pass docker auth if provided
	// var authHeader string
	if authConfig != nil {
		// authHeader = " -H 'X-Registry-Auth: [redacted]'"
		req.Header.Set("x-registry-auth", base64.StdEncoding.EncodeToString(authConfig))
	}

	resp, err := pc.performRequest(req, path)
	if err != nil {
		return nil, err
	}
	log.Println("ImagePullReport stream resp:", resp)

	stream := make(chan ImagePullReport)
	go func(r io.ReadCloser, c chan<- ImagePullReport) {
		scanner := bufio.NewScanner(r)
		defer close(c)
		for scanner.Scan() {
			chunk := []byte(scanner.Text())

			if chunk[0] == '[' {
				var events []ImagePullReport
				if err := json.Unmarshal(chunk, &events); err != nil {
					log.Println("ImagePullReport array err:", err)
					c <- ImagePullReport{Error: err.Error()}
					break
				} else {
					for _, event := range events {
						c <- event
					}
				}

			} else {
				var event ImagePullReport
				if err := json.Unmarshal(chunk, &event); err != nil {
					log.Println("ImagePullReport stream err:", err)
					c <- ImagePullReport{Error: err.Error()}
					break
				} else {
					c <- event
				}
			}
		}
		log.Println("ImagePullReport stream closed")
	}(resp.Body, stream)

	return stream, nil
}

type ImagePullReport struct {
	Stream string
	Error  string
	Images []string
	Id     string
}

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
