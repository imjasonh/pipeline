package main

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
)

// realWaiter actually waits for files, by polling.
type realWaiter struct{}

var _ entrypoint.Waiter = (*realWaiter)(nil)

var skipError = errors.New("error file found, bail and skip the steP")

// Wait watches a file and returns when either a) the file exists and, if
// the expectContent argument is true, the file has non-zero size or b) there
// is an error polling the file.
//
// If the passed-in file is an empty string then this function returns
// immediately.
//
// If a file of the same name with a ".err" extension exists then this Wait
// will returrn skipError.
func (*realWaiter) Wait(file string, expectContent bool) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("Error creating Watcher: %w", err)
	}

	log.Printf("watching %q...", filepath.Dir(file)) // TODO remove

	// Watch the dir of the expected file.
	if err := w.Add(filepath.Dir(file)); err != nil {
		return fmt.Errorf("Error watching %q: %w", filepath.Dir(file), err)
	}
	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				return errors.New("Events channel closed without Event")
			}
			log.Printf("Event: name=%q, op=%d", e.Name, e.Op) // TODO remove
			if e.Name == file+".err" {
				return skipError
			}
			if e.Name != file {
				continue
			}

			// File was created or written.
			if e.Op&fsnotify.Create == fsnotify.Create ||
				e.Op&fsnotify.Write == fsnotify.Write {
				// If we're expecting contents, open the file to see if there are contents.
				/*			if expectContent {
								b, err := ioutil.ReadFile(file)
								if err != nil {
									return fmt.Errorf("Error opening file after create or write: %w", err)
								}
								if len(b) > 0 {
									return nil
								}
							}
				*/
				return nil
			}
		case err, ok := <-w.Errors:
			if !ok || err == nil {
				return errors.New("Errors channel closed without error")
			}
			return err
		}
	}
}
