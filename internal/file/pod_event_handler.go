package file

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	exporter "github.com/shirolimit/pod-meta-exporter"
)

const gloggingPattern = "*.meta"

type fileToDelete struct {
	fullname string
	deleteAt time.Time
}

// PodMetaWriter is a PodMetaHandler that stores pod metadata in a specified directory.
// Files have a name format of `<namespace>_<pod_name>.meta` and are kept in `directory`.
type PodMetaWriter struct {
	retention time.Duration
	directory string
	logger    *log.Logger

	filesToDelete []fileToDelete
}

// NewPodMetaWriter creates a new PodMetaWriter with specified directory and retention period.
func NewPodMetaWriter(directory string, retention time.Duration, logger *log.Logger) *PodMetaWriter {
	writer := &PodMetaWriter{
		directory:     directory,
		retention:     retention,
		logger:        logger,
		filesToDelete: make([]fileToDelete, 0, 16),
	}
	writer.removeAllMetaFiles()
	return writer
}

// Handle handles PodEvent. The exact behaviour depends on event type:
//  - ADDED event - creates or updates file with metadata
//  - DELETED event - marks file for cleanup
func (w *PodMetaWriter) Handle(event exporter.PodEvent) error {
	w.deleteOldFiles()

	switch event.Type {
	case watch.Added:
		return w.createOrUpdateFile(event.Meta)
	case watch.Deleted:
		w.scheduleForDeletion(event.Meta)
	}
	return nil
}

func (w *PodMetaWriter) removeAllMetaFiles() error {
	files, err := filepath.Glob(filepath.Join(w.directory, gloggingPattern))
	if err != nil {
		return err
	}
	for _, file := range files {
		if err = os.Remove(file); err != nil {
			w.logger.Printf("Error deleting file %s: %v", file, err)
		}
	}
	return nil
}

func (w *PodMetaWriter) createOrUpdateFile(podMeta *v1.Pod) error {
	fullname := filepath.Join(w.directory, filenameForPod(podMeta))
	file, err := os.Create(fullname)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", fullname, err)
	}

	encoder := json.NewEncoder(file)
	return encoder.Encode(podMeta)
}

func (w *PodMetaWriter) scheduleForDeletion(podMeta *v1.Pod) {
	toDelete := fileToDelete{
		fullname: filepath.Join(w.directory, filenameForPod(podMeta)),
		deleteAt: time.Now().UTC().Add(w.retention),
	}
	w.filesToDelete = append(w.filesToDelete, toDelete)
}

func (w *PodMetaWriter) deleteOldFiles() {
	now := time.Now().UTC()

	filesRemoved := 0
	for i := range w.filesToDelete {
		toDelete := &w.filesToDelete[i]
		if now.Before(toDelete.deleteAt) {
			break
		}
		if err := os.Remove(toDelete.fullname); err != nil {
			w.logger.Printf("Error deleting file %s: %v", toDelete.fullname, err)
		}
		filesRemoved++
	}
	// It is ok to shift a slice here, cause the underlying array will eventually be replaced
	// after several appends and collected by GC along with its elements.
	w.filesToDelete = w.filesToDelete[filesRemoved:]
}

func filenameForPod(podMeta *v1.Pod) string {
	return fmt.Sprintf("%s_%s.meta", podMeta.Namespace, podMeta.Name)
}

var _ exporter.PodEventHandler = (*PodMetaWriter)(nil)
