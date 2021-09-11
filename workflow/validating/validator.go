package validating

import (
	"context"
	"fmt"
	cou "github.com/nj-eka/fdups/contextutils"
	"github.com/nj-eka/fdups/errs"
	. "github.com/nj-eka/fdups/filestat"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"runtime"
	"sync"
)

type ValidatorStats struct {
	FileStats, InodeStats registrator.Encounter
}

type Validator interface {
	ValidatedFileStatCh() <-chan FileStat
	ErrCh() <-chan errs.Error
	Run(ctx context.Context) <-chan struct{}
	Stats() interface{}
}

type validator struct {
	inputCh <-chan string
	resCh   chan FileStat
	errCh   chan errs.Error
	stats   ValidatorStats

	metaKeyFunc    MetaKeyFunc
	priorFunc      PriorFunc
	validatorFunc  FileStatValidatorFunc
	symLinkEnabled bool
	maxWorkers     int
}

func NewValidator(ctx context.Context,
	inputCh <-chan string,
	metaKeyFunc MetaKeyFunc,
	priorFunc PriorFunc,
	validatorFunc FileStatValidatorFunc,
	symLinkEnabled bool,
	initCap int) Validator {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("2.0.validation_init"))
	maxWorkers := cap(inputCh) * runtime.NumCPU()
	v := validator{
		inputCh: inputCh,
		resCh:   make(chan FileStat, maxWorkers),
		errCh:   make(chan errs.Error, maxWorkers*2),
		stats: ValidatorStats{
			FileStats:  registrator.NewEncounter(initCap),
			InodeStats: registrator.NewEncounter(initCap),
		},
		metaKeyFunc:    metaKeyFunc,
		priorFunc:      priorFunc,
		validatorFunc:  validatorFunc,
		symLinkEnabled: symLinkEnabled,
		maxWorkers:     maxWorkers,
	}
	return &v
}

func (r *validator) Run(ctx context.Context) <-chan struct{} {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("2.validation"))
	done := make(chan struct{})

	go func() {
		ctx = cou.BuildContext(ctx, cou.AddContextOperation("workers"))
		var (
			wg    sync.WaitGroup
			wPool = make(chan struct{}, r.maxWorkers)
		)
		defer workflow.OnExit(ctx, r.errCh, "workers", func() {
			wg.Wait()
			close(wPool)
			close(r.resCh)
			close(r.errCh)
			close(done)
		})
		for {
			select {
			case <-ctx.Done():
				return
			case filePath, more := <-r.inputCh:
				if !more {
					return
				}
				select {
				case <-ctx.Done():
					return
				case wPool <- struct{}{}:
					wg.Add(1)

					go func(filePath string) {
						defer wg.Done()
						defer func() { <-wPool }()
						if fs, err := GetFileStat(filePath, r.metaKeyFunc, r.priorFunc, r.symLinkEnabled); err == nil {
							if r.validatorFunc(fs) {
								select {
								case <-ctx.Done():
									return
								case r.resCh <- fs:
									r.stats.FileStats.CheckIn(registrator.KeySize{Key: fs.String(), Size: fs.Size()})
									r.stats.InodeStats.CheckIn(registrator.KeySize{Key: fs.Inode(), Size: fs.Size()})
								}
							}
						} else {
							r.errCh <- errs.E(ctx, errs.KindFileStat, fmt.Errorf("creating FileStat of [%s] failed: %w", filePath, err))
						}
					}(filePath)

				}
			}
		}
	}()
	return done
}

func (r *validator) ValidatedFileStatCh() <-chan FileStat {
	return r.resCh
}

func (r *validator) ErrCh() <-chan errs.Error {
	return r.errCh
}

func (r *validator) Stats() interface{} {
	return &r.stats
}
