package searching

import (
	"context"
	cou "github.com/nj-eka/fdups/contexts"
	"github.com/nj-eka/fdups/errs"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"path/filepath"
	"strings"
	"sync"
)

type SearcherStats registrator.Encounter

type Searcher interface {
	FoundFilePathsCh() <-chan string
	ErrCh() <-chan errs.Error
	Run(ctx context.Context) <-chan struct{}
	Stats() interface{}
}

type searcher struct {
	patterns []string
	resCh    chan string
	errCh    chan errs.Error
	stats    SearcherStats
}

func NewSearcher(ctx context.Context, roots []string, filePatterns []string, initCap int) Searcher {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("1.0.search_init"))
	patternsCount := len(roots) * len(filePatterns) // = maxWorkers
	sr := searcher{
		patterns: make([]string, 0, patternsCount),
		resCh:    make(chan string, patternsCount),
		errCh:    make(chan errs.Error, patternsCount*2),
		stats:    SearcherStats(registrator.NewEncounter(initCap)),
	}
	for _, rootDir := range roots {
		for _, filePattern := range filePatterns {
			sr.patterns = append(sr.patterns, filepath.Join(rootDir, filePattern))
		}
	}
	return &sr
}

func (r *searcher) Run(ctx context.Context) <-chan struct{} {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("1.search_run"))
	draftCh := make(chan string, cap(r.resCh))
	wg := sync.WaitGroup{}
	done := make(chan struct{})

	for _, pattern := range r.patterns {
		logging.LogMsg(ctx).Debugf("searching with pattern [%s]", pattern)
		// todo: merge globs
		if !strings.Contains(pattern, "**") {
			wg.Add(1)
			go CGlob(ctx, &wg, pattern, draftCh, r.errCh)
		} else {
			wg.Add(1)
			go Globs(strings.Split(pattern, "**")).CExpand(ctx, &wg, draftCh, r.errCh)
		}
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		close(draftCh)
	}()

	go func() {
		defer workflow.OnExit(ctx, r.errCh, "workers", func() {
			close(r.resCh)
			close(r.errCh)
		})
		for {
			select {
			case <-ctx.Done():
				return
			case path, more := <-draftCh:
				if !more {
					return
				}
				if r.stats.CheckIn(path) == 1 {
					select {
					case <-ctx.Done():
						return
					case <-done:
						return
					case r.resCh <- path:
					}
				}
			}
		}
	}()
	return done
}

func (r *searcher) FoundFilePathsCh() <-chan string {
	return r.resCh
}

func (r *searcher) ErrCh() <-chan errs.Error {
	return r.errCh
}

func (r *searcher) Stats() interface{} {
	return r.stats
}
