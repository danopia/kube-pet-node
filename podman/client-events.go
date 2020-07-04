package podman

import (
	"context"
	"io"
	"log"
	"encoding/json"
)

// since, until, filters, stream
func (pc *PodmanClient) StreamEvents(ctx context.Context) (<-chan Event, error) {
	resp, err := pc.performGet(ctx, "/libpod/events")
	if err != nil {
		return nil, err
	}
	log.Println("Event stream resp:", resp)

	stream := make(chan Event)
	go func(r io.ReadCloser, c chan<- Event) {
		decoder := json.NewDecoder(r)
		defer close(c)
		for {
			var event Event
			if err := decoder.Decode(&event); err != nil {
				log.Println("Event stream err:", err)
				break
			}
		}
		log.Println("Event stream closed")
	}(resp.Body, stream)

	return stream, nil
}

// Actor describes something that generates events,
// like a container, or a network, or a volume.
// It has a defined name and a set or attributes.
// The container attributes are its labels, other actors
// can generate these attributes from other properties.
type EventActor struct {
	ID         string
	Attributes map[string]string // image, name, containerExitCode
}

// Message represents the information an event contains
type Event struct {
	// Deprecated information from JSONMessage.
	// With data only in container events.
	Status string `json:"status,omitempty"`
	ID     string `json:"id,omitempty"`
	From   string `json:"from,omitempty"`

	Type   string // pod, container, daemon, image, network, plugin, volume, service, node, secret, config
	Action string // pull, create, init, start, remove,
	Actor  EventActor
	Scope  string `json:"scope,omitempty"`	// Engine events are "local". Cluster events are "swarm".

	Time     int64 `json:"time,omitempty"`
	TimeNano int64 `json:"timeNano,omitempty"`
}
