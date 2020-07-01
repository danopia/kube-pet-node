package podman

import (
	"net/http"
	"context"
	"net"
	"log"
	"encoding/json"
	"bytes"
)

type PodmanClient struct {
	http http.Client
}

func NewPodmanClient(socket string) (*PodmanClient) {
	return &PodmanClient{
		http: http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", socket)
				},
			},
		},
	}
}

func (pc *PodmanClient) performGet(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/v1.0.0"+path, nil)
	req.Header.Set("accept", "application/json")
	if err != nil {
		return nil, err
	}

	resp, err := pc.http.Do(req)
	if err != nil {
		log.Println("GET", path, err.Error())
	} else {
		log.Println("GET", path, resp.Status)
	}
	return resp, err
}

func (pc *PodmanClient) performPost(ctx context.Context, path string, input interface{}) (*http.Response, error) {
	reqBody, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://unix/v1.0.0"+path, bytes.NewBuffer(reqBody))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	if err != nil {
		return nil, err
	}

	resp, err := pc.http.Do(req)
	if err != nil {
		log.Println("POST", path, err.Error())
	} else {
		log.Println("POST", path, resp.Status)
	}
	return resp, err
}
