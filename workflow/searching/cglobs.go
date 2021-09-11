// Package wfs
// implements concurrent glob functionality with recursive "**" supports
// "**" in glob represents a recursive wildcard matching zero-or-more directory levels deep
// base on src: https://github.com/yargevad/filepathx/blob/master/filepathx.go
// todo: add rglob
package searching

import (
	"context"
	"fmt"
	cou "github.com/nj-eka/fdups/contextutils"
	"github.com/nj-eka/fdups/errs"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"io/fs"
	fp "path/filepath"
	"strings"
	"sync"
)

type Globs []string

// CExpand is concurrent extention of standard Glob function with support **.
func (globs Globs) CExpand(ctx context.Context, wg *sync.WaitGroup, cres chan<- string, cerr chan<- errs.Error) {
	ctx = cou.BuildContext(ctx, cou.AddContextOperation("CGlobs"))
	defer workflow.OnExit(ctx, cerr, fmt.Sprintf("CGlobs [%s]", globs), func() {
		wg.Done()
	})
	logging.Msg(ctx).Debug(fmt.Sprintf("CGlobs execution with globs [%s] - started", globs))

	var matches = []string{""}
	pathesDone := registrator.NewEncounter(1024)
	// todo: add check len(globs) > 1 cuz ** can be inside pattern as dirs substitution else err
	for globIndex := 0; globIndex < len(globs)-1; globIndex++ {
		glob := globs[globIndex]
		var hits []string
		var hitMap = map[string]bool{}
		for _, match := range matches {
			paths, err := fp.Glob(strings.TrimRight(match+glob, string(fp.Separator)))
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				cerr <- errs.E(ctx, errs.KindInvalidValue, fmt.Errorf("globbing for [%s]: %w", match+glob, err))
				return
			}
			for _, path := range paths {
				err := fp.WalkDir(path,
					func(path string, d fs.DirEntry, err error) error {
						if err == nil && d != nil {
							if d.IsDir() {
								if _, ok := hitMap[path]; !ok {
									hits = append(hits, path)
									hitMap[path] = true
								}
							} else {
								filePattern := fp.Join(fp.Dir(path), fp.Clean(strings.Join(globs[globIndex+1:], "")))
								if matched, _ := fp.Match(filePattern, path); matched {
									if pathesDone.CheckIn(path) == 1 {
										select {
										case <-ctx.Done():
											return fmt.Errorf("interrupted by context")
										case cres <- path:
										}
									}
								}
							}
						}
						return nil
					})
				if err != nil {
					cerr <- errs.E(ctx, errs.KindFileSystemOther, fmt.Errorf("walking dir [%s]: %w", path, err))
				}
			}
		}
		matches = hits
	}
}
