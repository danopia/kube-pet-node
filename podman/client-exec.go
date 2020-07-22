package podman

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net/http"
)

// Exec is stateful moreso than other 'created' things, so there's a special golang struct here
// The API methods for a running exec are there instead of on PodmanClient

type ContainerExecOptions struct {
	AttachStderr bool
	AttachStdin  bool
	AttachStdout bool
	Cmd          []string
	DetachKeys   string
	Env          []string
	Privileged   bool
	Tty          bool
	User         string
	WorkingDir   string
}

/// ContainerExec(ctx context.Context, nameOrID string, options ExecOptions, streams define.AttachStreams) (int, error)
func (pc *PodmanClient) ContainerExec(ctx context.Context, nameOrId string, options *ContainerExecOptions) (*ExecSession, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out ExecSession
	if err := pc.performPost(ctx, "/libpod/containers/"+encoded+"/exec", options, &out); err != nil {
		return nil, err
	}

	out.podmanClient = pc // attach an API client to the structure
	return &out, err
}

type ExecSession struct {
	podmanClient *PodmanClient
	Id           string
}

func (es *ExecSession) Inspect(ctx context.Context) (*ExecInspectResult, error) {
	var out ExecInspectResult
	return &out, es.podmanClient.performGet(ctx, "/libpod/exec/"+es.Id+"/json", &out)
}

type ExecInspectResult struct {
	CanRemove     bool               `json:"CanRemove"`
	ContainerID   string             `json:"ContainerID"`
	DetachKeys    string             `json:"DetachKeys"`
	ExitCode      int                `json:"ExitCode"`
	ID            string             `json:"ID"`
	OpenStderr    bool               `json:"OpenStderr"`
	OpenStdin     bool               `json:"OpenStdin"`
	OpenStdout    bool               `json:"OpenStdout"`
	Running       bool               `json:"Running"`
	Pid           int                `json:"Pid"`
	ProcessConfig *ExecProcessConfig `json:"ProcessConfig"`
}

type ExecProcessConfig struct {
	Arguments  []string `json:"arguments"`
	Entrypoint string   `json:"entrypoint"`
	Privileged bool     `json:"privileged"`
	Tty        bool     `json:"tty"`
	User       string   `json:"user"`
}

type ExecResizeOptions struct {
	Width  uint16 `json:"w"`
	Height uint16 `json:"h"`
}

func (es *ExecSession) Resize(ctx context.Context, newSize *ExecResizeOptions) error {
	var out struct{}
	return es.podmanClient.performPost(ctx, "/libpod/exec/"+es.Id+"/resize", newSize, &out)
}

func (es *ExecSession) Start(ctx context.Context) (io.Writer, io.ReadCloser, error) {
	path := "/libpod/exec/" + es.Id + "/start"
	req, err := http.NewRequestWithContext(ctx, "POST", "http://podman/v1.0.0"+path, bytes.NewBuffer([]byte("{}")))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("connection", "upgrade")
	req.Header.Set("upgrade", "tcp")
	if err != nil {
		return nil, nil, err
	}

	resp, err := es.podmanClient.performRequest(req, path)
	if err != nil {
		return nil, nil, err
	}
	log.Println("Exec start resp:", resp)

	if input, ok := resp.Body.(io.Writer); ok {
		return input, resp.Body, nil
	} else {
		return nil, resp.Body, nil
	}
}

func DemuxRawStream(input io.ReadCloser, stdout io.WriteCloser, stderr io.WriteCloser, addNewLines bool) error {
	header := make([]byte, 8)
	newline := []byte{0xa}

	var err error
	for err == nil {
		_, err = io.ReadFull(input, header)
		if err != nil {
			break
		}

		pktLen := binary.BigEndian.Uint32(header[4:])
		output := stdout
		if header[0] >= 2 {
			output = stderr
		}

		_, err = io.CopyN(output, input, int64(pktLen))
		if err == nil && addNewLines {
			_, err = output.Write(newline)
		}
	}

	stdout.Close()
	stderr.Close()
	return err
}
