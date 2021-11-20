package filtering

import (
	"context"
	cou "github.com/nj-eka/fdups/contexts"
	"github.com/nj-eka/fdups/errs"
	. "github.com/nj-eka/fdups/filestat"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"runtime"
)

type MetaFilterStats registrator.MifsRegister

type ContentId struct {
	checksums string
	fileStat  FileStat
}

type MetaFilter interface {
	DuplicateCh() <-chan ContentId
	ErrCh() <-chan errs.Error
	Run(ctx context.Context) <-chan struct{}
	Stats() interface{}
}

type metaFilter struct {
	inputCh <-chan FileStat
	resCh   chan ContentId
	errCh   chan errs.Error
	stats   MetaFilterStats
}

func NewMetaFilter(ctx context.Context, inputCh <-chan FileStat, initCapacity int) MetaFilter {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("3.0.metafilter_init"))
	maxWorkers := runtime.NumCPU() * 2
	return &metaFilter{
		inputCh: inputCh,
		resCh:   make(chan ContentId, maxWorkers),    // = number of hash filter workers
		errCh:   make(chan errs.Error, maxWorkers*2), // = cap(resCh) * 2
		stats:   registrator.NewMifsRegister(initCapacity),
	}
}

func (r *metaFilter) Run(ctx context.Context) <-chan struct{} {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("3.metafilter_run"))
	done := make(chan struct{})
	go func() {
		ctx = cou.BuildContext(ctx, cou.AddContextOperation("worker"))
		defer workflow.OnExit(ctx, r.errCh, "worker", func() {
			close(r.resCh)
			close(r.errCh)
			close(done)
		})
		for {
			select {
			case <-ctx.Done():
				return
			case inputFileStat, more := <-r.inputCh:
				if more {
					if inodes := r.stats.CheckIn(inputFileStat); len(inodes) > 1 {
						if len(inodes) == 2 {
							for inode, fss := range inodes {
								if inode != inputFileStat.Inode() {
									select {
									case <-ctx.Done():
										return
									case r.resCh <- ContentId{EMPTY_CHECKSUM, fss[0]}:
									}
								}
							}
						}
						select {
						case <-ctx.Done():
							return
						case r.resCh <- ContentId{EMPTY_CHECKSUM, inputFileStat}:
						}
					}
				} else {
					return
				}
			}
		}
	}()
	return done
}

func (r *metaFilter) DuplicateCh() <-chan ContentId {
	return r.resCh
}

func (r *metaFilter) ErrCh() <-chan errs.Error {
	return r.errCh
}

func (r *metaFilter) Stats() interface{} {
	return r.stats
}
