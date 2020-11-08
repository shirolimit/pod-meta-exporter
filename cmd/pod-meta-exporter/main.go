package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/oklog/run"
	"github.com/peterbourgon/ff/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/shirolimit/pod-meta-exporter/internal/file"
	"github.com/shirolimit/pod-meta-exporter/internal/k8s"
)

const logFlags = log.LstdFlags | log.LUTC | log.Lmsgprefix

func main() {
	defaultNodeName, _ := os.Hostname()
	fs := flag.NewFlagSet("pod-meta-exporter", flag.ExitOnError)
	var (
		nodeName        = fs.String("node-name", defaultNodeName, "The name of a node to track pods on")
		retentionPeriod = fs.Duration("retention-period", 1*time.Minute, "The amount of time to keep a file with metadata after a pod was deleted")
		destinationDir  = fs.String("destination-dir", "/var/kube_meta", "The path to a directory to put files to")
	)

	ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("EXPORTER"),
		ff.WithConfigFileParser(ff.PlainParser),
	)

	log.SetFlags(logFlags)
	log.Printf("Running with config: retention = %v, directory = %s", *retentionPeriod, *destinationDir)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Error creating k8s client config: %v", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error creating k8s client: %v", err)
		os.Exit(1)
	}

	tracker := k8s.NewPodTracker(clientset, log.New(os.Stderr, "PodTracker ", logFlags))
	handler := file.NewPodMetaWriter(*destinationDir, *retentionPeriod, log.New(os.Stderr, "PodMetaWriter ", logFlags))

	ctx, cancel := context.WithCancel(context.Background())
	var g run.Group
	{
		g.Add(run.SignalHandler(ctx, os.Interrupt))
		g.Add(func() error {
			events, err := tracker.TrackPods(ctx, *nodeName)
			if err != nil {
				return err
			}

			for event := range events {
				if err = handler.Handle(event); err != nil {
					return err
				}
			}
			return nil
		}, func(err error) {
			cancel()
		})
	}
	if err := g.Run(); err != context.Canceled {
		log.Printf("Application error: %s", err)
		os.Exit(1)
	}
	cancel()
}
