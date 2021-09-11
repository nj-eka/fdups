package searching

import (
	"context"
	"fmt"
	cou "github.com/nj-eka/fdups/contextutils"
	"github.com/nj-eka/fdups/errs"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/workflow"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// CGlob - adapted version of Glob function from standard library (without ** support) to work in concurrent mode
func CGlob(ctx context.Context, wg *sync.WaitGroup, pattern string, cres chan<- string, cerr chan<- errs.Error) {
	ctx = cou.BuildContext(ctx, cou.AddContextOperation("CGlob"))
	defer workflow.OnExit(ctx, cerr, fmt.Sprintf("CGlob [%s]", pattern), func() {
		wg.Done()
	})
	logging.Msg(ctx).Debugf(fmt.Sprintf("CGlob execution with pattern [%s] - started", pattern))

	// Check pattern is well-formed.
	if _, err := filepath.Match(pattern, ""); err != nil {
		cerr <- errs.E(ctx, errs.KindInvalidValue, fmt.Errorf("Wrong pattern format [%s]: %w", pattern, err))
		return
	}
	if !hasMeta(pattern) {
		if _, err := os.Lstat(pattern); err != nil {
			cerr <- errs.E(ctx, errs.KindOSStat, err) // fmt.Errorf("getting stat of [%s]: %w", pattern, err)) -> see PathError
		} else {
			cres <- pattern
		}
		return
	}
	dir, file := filepath.Split(pattern)
	// todo: add windows volume support
	dir = cleanGlobPath(dir)
	if !hasMeta(dir) {
		// file contains meta (pattern)
		glob(ctx, dir, file, cres, cerr)
	} else {
		// dir contains meta -> go deeper
		// Prevent infinite recursion. See issue 15879. on Windows with patterns like `\\?\C:\*`
		if dir == pattern {
			cerr <- errs.E(ctx, errs.KindFileSystemOther, fmt.Errorf("infinite recursion with pattern [%s] on dir [%s] (see filepath issue 15879)", pattern, dir))
			return
		}
		done := make(chan struct{})
		go func(dir, pattern string) {
			defer close(done)
			dirs, err := filepath.Glob(dir) // blocking function call with long execution time (depending on number of files being processed)
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				cerr <- errs.E(ctx, errs.KindInvalidValue, fmt.Errorf("globbing on dir [%s]: %w", dir, err)) // ErrBadPattern
				return
			}
			for _, dir := range dirs {
				glob(ctx, dir, pattern, cres, cerr)
			}
		}(dir, file)
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		}
	}
}

func glob(ctx context.Context, dir, pattern string, cres chan<- string, cerr chan<- errs.Error) {
	defer workflow.OnExit(ctx, cerr, fmt.Sprintf("Glob in dir [%s] with pattern [%s]", dir, pattern), func() {
	})
	logging.Msg(ctx).Debugf(fmt.Sprintf("Glob searching in dir [%s] with pattern [%s] - started", dir, pattern))
	fi, err := os.Stat(dir)
	if err != nil {
		cerr <- errs.E(ctx, errs.KindOSStat, fmt.Errorf("getting stat of [%s]: %w", dir, err))
		return
	}
	if !fi.IsDir() {
		cerr <- errs.E(ctx, errs.SeverityWarning, errs.KindNotDir, fmt.Errorf("[%s] is not dir", dir)) // or ignore I/O error
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		cerr <- errs.E(ctx, errs.SeverityWarning, errs.KindIO, fmt.Errorf("opening [%s]: %w", dir, err)) // or ignore I/O error
		return
	}
	defer d.Close() // open for reading
	names, err := d.Readdirnames(-1)
	if err != nil {
		cerr <- errs.E(ctx, errs.KindIO, fmt.Errorf("reading [%s]: %w", dir, err))
		return
	}
	for _, name := range names {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			cerr <- errs.E(ctx, errs.KindInvalidValue, fmt.Errorf("matching [%s] with pattern [%s] in dir [%s] failed: %w", name, pattern, dir, err)) //  ErrBadPattern or ignore I/O error
			continue
		}
		if matched {
			select {
			case <-ctx.Done():
				return
			case cres <- filepath.Join(dir, name):
			}
		}
	}
}

// cleanGlobPath prepares path for glob matching (copy from filepath pkg)
func cleanGlobPath(path string) string {
	switch path {
	case "":
		return "."
	case string(filepath.Separator):
		// do nothing to the path
		return path
	default:
		return path[0 : len(path)-1] // chop off trailing separator
	}
}

func hasMeta(path string) bool {
	magicChars := `*?[`
	if runtime.GOOS != "windows" {
		magicChars = `*?[\`
	}
	return strings.ContainsAny(path, magicChars)
}
