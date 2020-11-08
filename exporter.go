package exporter

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// PodEvent represents an event that happened with a pod.
type PodEvent struct {
	Meta *v1.Pod
	Type watch.EventType
}

// PodTracker is responsible for tracking pod events.
type PodTracker interface {
	// TrackPods starts tracking pod events on a specified node.
	// Returns a channel of PodEvents that can be used to fetch corresponding events.
	TrackPods(ctx context.Context, node string) (<-chan PodEvent, error)
}

// PodEventHandler is responsible for handling pod events.
type PodEventHandler interface {
	Handle(event PodEvent) error
}
