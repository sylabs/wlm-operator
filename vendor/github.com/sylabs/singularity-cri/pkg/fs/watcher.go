// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fs

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
)

const (
	// OpUnsupported stands for currently unsupported file operation type.
	OpUnsupported = Op(iota)
	// OpRemove is used when watched file was removed.
	OpRemove
	// OpCreate is used when watched file was created.
	OpCreate
)

// Watcher is a filesystem watcher that can be used
// to watch filesystem changes.
type Watcher struct {
	*fsnotify.Watcher
}

// Op is a separate type for watch file events.
type Op int

// WatchEvent is a single event that happens during filesystem watch.
type WatchEvent struct {
	Path string
	Op   Op
}

// NewWatcher creates new Watcher that will be watching passed files or directories
// that already exist. Currently only create and remove operations are supported.
// NOTE: when watching a single file no new event will be triggered after it's removal.
func NewWatcher(files ...string) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("could not create file watcher: %v", err)
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			watcher.Close()
			return nil, fmt.Errorf("could not add %s to file watcher: %v", f, err)
		}
	}

	return &Watcher{Watcher: watcher}, nil
}

// Watch starts filesystem watching, all occurred events will be sent
// to returned channel. Returned channel is unbuffered, so make sure to read from
// it. Watcher will be cancelled as soon as context is done.
func (w *Watcher) Watch(ctx context.Context) <-chan WatchEvent {
	events := make(chan WatchEvent)
	go func() {
		defer close(events)
		for {
			select {
			case event := <-w.Events:
				var op Op
				if event.Op&fsnotify.Create == fsnotify.Create {
					op = OpCreate
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					op = OpRemove
				}
				if op == OpUnsupported {
					continue
				}
				events <- WatchEvent{
					Path: event.Name,
					Op:   op,
				}
			case err := <-w.Errors:
				// we skip errors for now
				_ = err
			case <-ctx.Done():
				return
			}
		}
	}()
	return events
}
