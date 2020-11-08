package file_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	exporter "github.com/shirolimit/pod-meta-exporter"
	"github.com/shirolimit/pod-meta-exporter/internal/file"
)

type PodMetaWriterSuite struct {
	suite.Suite
	dir string
}

func (s *PodMetaWriterSuite) SetupTest() {
	s.dir, _ = ioutil.TempDir("", "")
}

func (s *PodMetaWriterSuite) TearDownTest() {
	os.RemoveAll(s.dir)
}

func (s *PodMetaWriterSuite) Test_ShouldCreateFileOnAddedEvent() {
	writer := file.NewPodMetaWriter(s.dir, 1*time.Minute, nopLogger())
	expectedFile := filepath.Join(s.dir, "kube-system_etcd-docker-desktop.meta")

	event := exporter.PodEvent{
		Type: watch.Added,
		Meta: podMetaFromFile("testdata/pod_payload.json"),
	}

	err := writer.Handle(event)

	assert.NoError(s.T(), err)
	assert.FileExists(s.T(), expectedFile)
	assert.JSONEq(s.T(), readFile("testdata/pod_payload.json"), readFile(expectedFile))
}

func (s *PodMetaWriterSuite) Test_ShouldReturnErrorOnFileCreationFailure() {
	writer := file.NewPodMetaWriter(s.dir, 1*time.Minute, nopLogger())

	event := exporter.PodEvent{
		Type: watch.Added,
		Meta: podMetaFromFile("testdata/impossible_name.json"),
	}

	err := writer.Handle(event)

	assert.Error(s.T(), err)
}

func (s *PodMetaWriterSuite) Test_ShouldNotCreateFileOnModifiedEvent() {
	writer := file.NewPodMetaWriter(s.dir, 1*time.Minute, nopLogger())
	expectedFile := filepath.Join(s.dir, "kube-system_etcd-docker-desktop.meta")

	event := exporter.PodEvent{
		Type: watch.Modified,
		Meta: podMetaFromFile("testdata/pod_payload.json"),
	}

	err := writer.Handle(event)

	assert.NoError(s.T(), err)
	assert.False(s.T(), fileExists(expectedFile))
}

func (s *PodMetaWriterSuite) Test_ShouldDeleteFileAfterRetention() {
	writer := file.NewPodMetaWriter(s.dir, 3*time.Second, nopLogger())
	expectedFile := filepath.Join(s.dir, "kube-system_etcd-docker-desktop.meta")

	addedEvent := exporter.PodEvent{
		Type: watch.Added,
		Meta: podMetaFromFile("testdata/pod_payload.json"),
	}
	deletedEvent := exporter.PodEvent{
		Type: watch.Deleted,
		Meta: podMetaFromFile("testdata/pod_payload.json"),
	}

	writer.Handle(addedEvent)
	assert.FileExists(s.T(), expectedFile)

	writer.Handle(deletedEvent)
	assert.FileExists(s.T(), expectedFile)

	<-time.After(3 * time.Second)

	writer.Handle(exporter.PodEvent{Type: watch.Bookmark})
	assert.False(s.T(), fileExists(expectedFile))
}

func (s *PodMetaWriterSuite) Test_ShouldDeleteFilesOnStart() {
	staleFile := filepath.Join(s.dir, "test-file.meta")
	os.Create(staleFile)

	writer := file.NewPodMetaWriter(s.dir, 3*time.Second, nopLogger())

	writer.Handle(exporter.PodEvent{Type: watch.Bookmark})
	assert.False(s.T(), fileExists(staleFile))
}

func TestPodMetaWriterSuite(t *testing.T) {
	suite.Run(t, new(PodMetaWriterSuite))
}

func nopLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0)
}

func podMetaFromFile(path string) *v1.Pod {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var pod v1.Pod
	if err = json.Unmarshal(data, &pod); err != nil {
		panic(err)
	}
	return &pod
}

func readFile(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func fileExists(path string) bool {
	nfo, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return !nfo.IsDir()
}
