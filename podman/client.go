package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

type PodmanClient struct {
	http http.Client
}

func NewPodmanClient(proto, socket string) *PodmanClient {
	return &PodmanClient{
		http: http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, proto, socket)
				},
			},
		},
	}
}

func (pc *PodmanClient) performRequest(req *http.Request, path string) (*http.Response, error) {
	resp, err := pc.http.Do(req)
	if err != nil {
		log.Println(req.Method, path, err.Error())
		return resp, err
	} else {
		log.Println(req.Method, path, resp.Status)
	}

	// check for happy path, return on the spot
	if resp.StatusCode == 101 || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return resp, nil
	} else if resp.StatusCode == 304 {
		// Not Modified seemingly doesn't have an error payload even though swagger shows one
		return resp, &ApiError{
			request: req,
			Cause:   resp.Status,
			Status:  resp.StatusCode,
		}
	}

	// decode common error fields
	var apiErr ApiError
	apiErr.request = req
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return resp, err
	}

	return resp, &apiErr
}

func (pc *PodmanClient) performJsonRequest(req *http.Request, path string, output interface{}) error {
	resp, err := pc.performRequest(req, path)
	if err != nil {
		return err
	}

	if output == nil {
		if resp.StatusCode == 201 || resp.StatusCode == 204 {
			return nil
		} else {
			return fmt.Errorf("Client caller didn't expect a body, but we got an HTTP %v", resp.StatusCode)
		}
	} else {
		return json.NewDecoder(resp.Body).Decode(output)
	}
}

type ApiError struct {
	request *http.Request
	Cause   string `json:"cause"`
	Message string `json:"message"`
	Status  int    `json:"response"`
}

func (ae *ApiError) Error() string {
	return fmt.Sprintf("HTTP %v from %v %v -- %v", ae.Status, ae.request.Method, ae.request.URL.Path, ae.Message)
}

func (pc *PodmanClient) performRawRequest(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://podman/v1.0.0"+path, nil)
	req.Header.Set("accept", "application/json")
	if err != nil {
		return nil, err
	}

	return pc.performRequest(req, path)
}

func (pc *PodmanClient) performGet(ctx context.Context, path string, output interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://podman/v1.0.0"+path, nil)
	req.Header.Set("accept", "application/json")
	if err != nil {
		return err
	}

	return pc.performJsonRequest(req, path, output)
}

func (pc *PodmanClient) performPost(ctx context.Context, path string, input interface{}, output interface{}) error {
	reqBody, err := json.Marshal(input)
	if err != nil {
		return err
	}

	log.Printf("curl -XPOST http://podman/v1.0.0/%v -H 'Content-Type: application/json' --data '%v'", path, string(reqBody))
	req, err := http.NewRequestWithContext(ctx, "POST", "http://podman/v1.0.0"+path, bytes.NewBuffer(reqBody))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	if err != nil {
		return err
	}

	return pc.performJsonRequest(req, path, output)
}

func (pc *PodmanClient) performDelete(ctx context.Context, path string, output interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", "http://podman/v1.0.0"+path, nil)
	req.Header.Set("accept", "application/json")
	if err != nil {
		return err
	}

	return pc.performJsonRequest(req, path, output)
}
