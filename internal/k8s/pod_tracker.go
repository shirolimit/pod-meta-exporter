package k8s

import (
	"context"
	"errors"
	"fmt"
	"log"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	exporter "github.com/shirolimit/pod-meta-exporter"
)

const allNamespaces = ""
const nodeSelectorTemplate = "spec.nodeName=%s"

// PodTracker tracks pods using Kubernetes API client.
type PodTracker struct {
	client kubernetes.Interface
	logger *log.Logger

	running bool
}

// NewPodTracker creates a new PodTracker that watches pods using specified kuberneter API client.
func NewPodTracker(k8sClient kubernetes.Interface, logger *log.Logger) *PodTracker {
	return &PodTracker{
		client: k8sClient,
		logger: logger,
	}
}

// TrackPods starts tracking pod events on a specified node.
// Returns a channel of PodEvents that can be used to fetch corresponding events.
func (t *PodTracker) TrackPods(ctx context.Context, node string) (<-chan exporter.PodEvent, error) {
	if t.running {
		return nil, fmt.Errorf("already tracking")
	}
	t.running = true
	events := make(chan exporter.PodEvent)

	watchOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf(nodeSelectorTemplate, node),
	}

	go t.watcherWorker(ctx, watchOptions, events)

	return events, nil
}

func (t *PodTracker) watcherWorker(ctx context.Context, options metav1.ListOptions, events chan<- exporter.PodEvent) {
	defer func() {
		close(events)
		t.running = false
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := t.client.CoreV1().Pods(allNamespaces).Watch(options)
		if err != nil {
			t.logger.Printf("Error opening watcher: %s", err)
			if utilnet.IsConnectionRefused(err) {
				continue
			}
			return
		}

		err = t.watchHandler(ctx, watcher, events)
		if err != nil {
			t.logger.Printf("Error while watching: %s", err)
			if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
				continue
			}
			if errors.Is(err, context.Canceled) {
				t.logger.Printf("Wathcher normal exit")
				return
			}
			t.logger.Fatalf("Watching error: %s", err)
		}
	}
}

func (t *PodTracker) watchHandler(ctx context.Context, watcher watch.Interface, events chan<- exporter.PodEvent) error {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}

			if event.Type == watch.Error {
				t.logger.Printf("Error event received: %v", event.Object)
				return apierrors.FromObject(event.Object)
			}
			if event.Type == watch.Bookmark {
				continue
			}

			podMeta, ok := event.Object.(*v1.Pod)
			if !ok {
				t.logger.Printf("Error casting event object to v1.Pod, actual type is %T", event.Object)
				continue
			}

			events <- exporter.PodEvent{
				Type: event.Type,
				Meta: podMeta,
			}
		}
	}

}

var _ exporter.PodTracker = (*PodTracker)(nil)
