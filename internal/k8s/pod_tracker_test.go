package k8s_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	exporter "github.com/shirolimit/pod-meta-exporter"
	"github.com/shirolimit/pod-meta-exporter/internal/k8s"
)

type K8sPodTrackerSuite struct {
	suite.Suite
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldReturnErrorOnMultipleTracks() {
	fakeClientset := fake.NewSimpleClientset()
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := tracker.TrackPods(ctx, "node")
	assert.NoError(s.T(), err)

	_, err = tracker.TrackPods(ctx, "node")
	assert.Error(s.T(), err)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldStopOnContextCancel() {
	fakeClientset := fake.NewSimpleClientset()
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	events, _ := tracker.TrackPods(ctx, "node")

	cancel()
	<-events
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldNotReturnErrorOnWatchError() {
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, nil, fmt.Errorf("kernel panic")
	})
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldTrackPodsFromAllNamespaces() {
	fakeClientset := fake.NewSimpleClientset()
	waitCh := make(chan struct{})
	fakeClientset.PrependWatchReactor("*", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		assert.Equal(s.T(), "", action.GetNamespace())
		assert.Equal(s.T(), "pods", action.GetResource().Resource)
		close(waitCh)
		return true, nil, fmt.Errorf("kernel panic")
	})
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	tracker.TrackPods(ctx, "node")

	<-waitCh

	cancel()
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldReturnAddedEvent() {
	fakeWatcher := watch.NewFake()
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakeWatcher, nil))
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	fakeWatcher.Add(pod)

	var event exporter.PodEvent
	select {
	case event = <-events:
		cancel()
	default:
	}

	assert.Equal(s.T(), watch.Added, event.Type)
	assert.Equal(s.T(), pod, event.Meta)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldReturnDeletedEvent() {
	fakeWatcher := watch.NewFake()
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakeWatcher, nil))
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	fakeWatcher.Delete(pod)

	var event exporter.PodEvent
	select {
	case event = <-events:
		cancel()
	default:
	}

	assert.Equal(s.T(), watch.Deleted, event.Type)
	assert.Equal(s.T(), pod, event.Meta)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldReturnModifiedEvent() {
	fakeWatcher := watch.NewFake()
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakeWatcher, nil))
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	fakeWatcher.Modify(pod)

	var event exporter.PodEvent
	select {
	case event = <-events:
		cancel()
	default:
	}

	assert.Equal(s.T(), watch.Modified, event.Type)
	assert.Equal(s.T(), pod, event.Meta)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldIgnoreNonPodEvents() {
	fakeWatcher := watch.NewFake()
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakeWatcher, nil))
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
	replicationController := &v1.ReplicationController{}
	fakeWatcher.Modify(replicationController)

	var event exporter.PodEvent
	select {
	case event = <-events:
		cancel()
	default:
	}

	assert.Nil(s.T(), event.Meta)
}

func (s *K8sPodTrackerSuite) Test_TrackPodsShouldNotReturnBookmarkEvent() {
	fakeWatcher := watch.NewFake()
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakeWatcher, nil))
	tracker := k8s.NewPodTracker(fakeClientset, nopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := tracker.TrackPods(ctx, "node")

	assert.NoError(s.T(), err)
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	fakeWatcher.Action(watch.Bookmark, pod)

	var event exporter.PodEvent
	select {
	case event = <-events:
	default:
	}

	assert.Nil(s.T(), event.Meta)
}

func nopLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0)
}

func TestK8sPodTrackerSuite(t *testing.T) {
	suite.Run(t, new(K8sPodTrackerSuite))
}
