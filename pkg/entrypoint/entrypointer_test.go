/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package entrypoint

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEntrypointerFailures(t *testing.T) {
	for _, c := range []struct {
		desc, postFile string
		waitFiles      []string
		waiter         Waiter
		runner         Runner
		expectedError  string
	}{{
		desc:          "failing runner with no postFile",
		runner:        &fakeRunner{err: errors.New("runner failed")},
		expectedError: "runner failed",
	}, {
		desc:          "failing runner with postFile",
		runner:        &fakeRunner{err: errors.New("runner failed")},
		expectedError: "runner failed",
		postFile:      "foo",
	}, {
		desc:          "failing waiter with no postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeWaiter{err: errors.New("waiter failed")},
		expectedError: "waiter failed",
	}, {
		desc:          "failing waiter with postFile",
		waitFiles:     []string{"foo"},
		waiter:        &fakeWaiter{err: errors.New("waiter failed")},
		expectedError: "waiter failed",
		postFile:      "bar",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw := c.waiter
			if fw == nil {
				fw = &fakeWaiter{}
			}
			fr := c.runner
			if fr == nil {
				fr = &fakeRunner{}
			}
			ffw := fakeWriter{}
			err := Entrypointer{
				Entrypoint: "echo",
				WaitFiles:  c.waitFiles,
				PostFile:   c.postFile,
				Args:       []string{"some", "args"},
				Waiter:     fw,
				Runner:     fr,
				Writer:     ffw,
			}.Go()
			if err == nil {
				t.Fatalf("Entrpointer didn't fail")
			}
			if d := cmp.Diff(c.expectedError, err.Error()); d != "" {
				t.Errorf("Entrypointer error diff -want, +got: %v", d)
			}

			if c.postFile != "" {
				if len(ffw) == 0 {
					t.Error("Wanted post file written, got none")
				} else if !ffw.wrote(c.postFile + ".err") {
					t.Errorf("Didn't write file %q, got %v", c.postFile+".err", ffw)
				}
			}
			if c.postFile == "" && len(ffw) != 0 {
				t.Errorf("Wrote post file when not required")
			}
		})
	}
}

func TestEntrypointer(t *testing.T) {
	for _, c := range []struct {
		desc, entrypoint, postFile, startFile string
		waitFiles, args                       []string
	}{{
		desc: "do nothing",
	}, {
		desc:       "just entrypoint",
		entrypoint: "echo",
	}, {
		desc:       "entrypoint and args",
		entrypoint: "echo", args: []string{"some", "args"},
	}, {
		desc: "just args",
		args: []string{"just", "args"},
	}, {
		desc:      "wait file",
		waitFiles: []string{"waitforme"},
	}, {
		desc:     "post file",
		postFile: "writeme",
	}, {
		desc:       "all together now",
		entrypoint: "echo", args: []string{"some", "args"},
		waitFiles: []string{"waitforme"},
		postFile:  "writeme",
	}, {
		desc:      "multiple wait files",
		waitFiles: []string{"waitforme", "metoo", "methree"},
	}, {
		desc:      "start file",
		startFile: "istarted",
	}, {
		desc:      "start and post file",
		startFile: "istarted",
		postFile:  "writeme",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			fw, fr, ffw := &fakeWaiter{}, &fakeRunner{}, fakeWriter{}
			err := Entrypointer{
				Entrypoint: c.entrypoint,
				WaitFiles:  c.waitFiles,
				PostFile:   c.postFile,
				StartFile:  c.startFile,
				Args:       c.args,
				Waiter:     fw,
				Runner:     fr,
				Writer:     ffw,
			}.Go()
			if err != nil {
				t.Fatalf("Entrypointer failed: %v", err)
			}

			if len(c.waitFiles) > 0 {
				if fw.waited == nil {
					t.Error("Wanted waited file, got nil")
				} else if !reflect.DeepEqual(fw.waited, c.waitFiles) {
					t.Errorf("Waited for %v, want %v", fw.waited, c.waitFiles)
				}
			}
			if len(c.waitFiles) == 0 && fw.waited != nil {
				t.Errorf("Waited for file when not required")
			}

			wantArgs := c.args
			if c.entrypoint != "" {
				wantArgs = append([]string{c.entrypoint}, c.args...)
			}
			if len(wantArgs) != 0 {
				if fr.args == nil {
					t.Error("Wanted command to be run, got nil")
				} else if !reflect.DeepEqual(*fr.args, wantArgs) {
					t.Errorf("Ran %s, want %s", *fr.args, wantArgs)
				}
			}
			if len(wantArgs) == 0 && c.args != nil {
				t.Errorf("Ran command when not required")
			}

			if c.postFile != "" {
				if len(ffw) == 0 {
					t.Error("Wanted post file written, got none")
				} else if !ffw.wrote(c.postFile) {
					t.Errorf("Didn't write post file %q, got %v", c.postFile, ffw)
				}
			}
			if c.startFile != "" {
				if len(ffw) == 0 {
					t.Error("Wanted start file written, got none")
				} else if !ffw.wrote(c.startFile) {
					t.Errorf("Didn't write start file %q, got %v", c.startFile, ffw)
				}
			}
			if c.postFile == "" && c.startFile == "" && len(ffw) != 0 {
				t.Errorf("Wrote file when not required")
			}
		})
	}
}

type fakeWaiter struct {
	waited []string
	err    error
}

func (f *fakeWaiter) Wait(file string, _ bool) error {
	f.waited = append(f.waited, file)
	return f.err
}

type fakeRunner struct {
	args *[]string
	err  error
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) error {
	f.args = &args
	return f.err
}

type fakeWriter map[string]struct{}

func (f fakeWriter) Write(file string) error {
	f[file] = struct{}{}
	return nil
}

func (f fakeWriter) wrote(file string) bool {
	_, found := f[file]
	return found
}
