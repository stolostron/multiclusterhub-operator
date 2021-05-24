// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package webhook

import (
	"fmt"
	// "log"
	"github.com/fsnotify/fsnotify"
)

// Start starts the watch on the chart directory files.
func Start(s *fsnotify.Watcher, certDir string) error {
	if err := s.Add(certDir); err != nil {
		return err
	}

	go Watch(s)

	log.Info("Starting file watcher")
	return nil
}

// Stop closes the server's file watcher.
func Stop(s *fsnotify.Watcher) error {
	log.Info("Stopping file watcher")
	return s.Close()
}

// Watch reads events from the watcher's channel and reacts to changes.
func Watch(s *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-s.Events:
			// Channel is closed.
			if !ok {
				return
			}

			log.Info("File change detected: ", event.String())
			

		case err, ok := <-s.Errors:
			// Channel is closed.
			if !ok {
				return
			}

			log.Error(err, fmt.Sprintf("Something happened with the close"))
		}
	}
}

