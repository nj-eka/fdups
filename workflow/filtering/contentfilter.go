package filtering

import (
	"context"
	"fmt"
	cou "github.com/nj-eka/fdups/contexts"
	"github.com/nj-eka/fdups/errs"
	. "github.com/nj-eka/fdups/filestat"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type ContentFilter interface {
	ErrCh() <-chan errs.Error
	Run(ctx context.Context) <-chan struct{}
	Stats() interface{}
}

type contentFilter struct {
	inputCh <-chan ContentId
	errCh   chan errs.Error
	stats   ContentFilterStats

	hashFilterFuncs           []HashFileFunc
	skipPrefiltersMaxSizeFunc FileSizeLesserFunc
	contentIds                []chan ContentId
	maxWorkersPerStage        int
}

func NewContentFilter(ctx context.Context, inputCh <-chan ContentId, metaRegister registrator.MifsRegister, hashFilterFuncs []HashFileFunc, skipPrefiltersMaxSizeFunc FileSizeLesserFunc, initCap int) ContentFilter {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("4.0.contentfilter_init"))
	maxStageWorkers := runtime.NumCPU()
	contentIds := make([]chan ContentId, 0, len(hashFilterFuncs))
	stageRegisters := make([]registrator.McifsRegister, 0, len(hashFilterFuncs))
	stageInodeStats := make([]registrator.InodeChecksums, 0, len(hashFilterFuncs))
	for range hashFilterFuncs {
		contentIds = append(contentIds, make(chan ContentId, maxStageWorkers*2))
		stageRegisters = append(stageRegisters, registrator.NewMcifsRegister(initCap))
		stageInodeStats = append(stageInodeStats, registrator.NewInodeChecksums(initCap))
	}
	return &contentFilter{
		inputCh: inputCh,
		errCh:   make(chan errs.Error, maxStageWorkers*len(hashFilterFuncs)*2),
		stats: ContentFilterStats{
			MetaRegister:    metaRegister,
			StageRegisters:  stageRegisters,
			StageInodeStats: stageInodeStats,
			ContentRegister: registrator.NewMcifsRegister(initCap),
		},
		hashFilterFuncs:           hashFilterFuncs,
		skipPrefiltersMaxSizeFunc: skipPrefiltersMaxSizeFunc,
		contentIds:                contentIds,
		maxWorkersPerStage:        maxStageWorkers,
	}
}

func (r *contentFilter) Run(ctx context.Context) <-chan struct{} {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("4.contentfilter_run"))
	done := make(chan struct{})

	lastIndex := len(r.hashFilterFuncs) - 1
	finalCidsStream := r.contentIds[lastIndex]

	// start hash cropping
	go func(ctx context.Context) {
		ctx = cou.BuildContext(ctx, cou.AddContextOperation("stages"))
		var wgStages sync.WaitGroup
		defer func() {
			wgStages.Wait()
			logging.LogMsg(ctx).Debugf("closed")
		}()
		inputStream := r.inputCh

		for i := range r.hashFilterFuncs {
			if i > 0 {
				inputStream = r.contentIds[i-1]
			}
			wgStages.Add(1)

			go func(ctx context.Context, wgStages *sync.WaitGroup, inputStream <-chan ContentId, index int) {
				ctx = cou.BuildContext(ctx, cou.AddContextOperation(cou.Operation(fmt.Sprintf("stage_[%d]", index))))
				var (
					outputStream   = r.contentIds[index]
					hashFilterFunc = r.hashFilterFuncs[index]
					register       = r.stats.StageRegisters[index]
					iS             = r.stats.StageInodeStats[index]
					wgStageWorkers sync.WaitGroup
					wpStageWorkers = make(chan struct{}, r.maxWorkersPerStage)
				)
				defer workflow.OnExit(ctx, r.errCh, fmt.Sprintf("stage_[%d]", index), func() {
					wgStageWorkers.Wait()
					close(wpStageWorkers)
					close(outputStream)
					wgStages.Done()
				})
				for {
					select {
					case <-ctx.Done():
						return
					case cid, more := <-inputStream:
						if more {
							// if file's size is relative small, skip prefilters
							if (index < lastIndex) && r.skipPrefiltersMaxSizeFunc(cid.fileStat) {
								r.contentIds[lastIndex-1] <- cid // bypass all prehashing stages
							} else {
								select {
								case <-ctx.Done():
									return
								case wpStageWorkers <- struct{}{}:
									wgStageWorkers.Add(1)

									go func(cid ContentId, wg *sync.WaitGroup) {
										defer wg.Done()
										defer func() { <-wpStageWorkers }()
										var (
											err          error
											written      int64
											checksums    string
											exist, valid bool
										)
										checksums, exist, valid, pending := iS.CheckIn(cid.fileStat)
										if !exist {
											if pending == nil { // first time checksum calculation
												checksums, written, err = hashFilterFunc(cid.fileStat, strconv.Itoa(index))
												if err == nil {
													checksums = strings.Join([]string{cid.checksums, checksums}, "&")
													iS.Update(cid.fileStat, checksums, written)
													valid = true
												} else {
													iS.Delete(cid.fileStat)
													r.errCh <- errs.E(ctx, errs.KindIO, fmt.Errorf("content hashing stage [%d] with processing file [%s] failed: %w", index, cid.fileStat, err))
													return
												}
											} else { // checksum pending
												<-pending
												checksums, _, valid, _ = iS.CheckIn(cid.fileStat)
											}
										}
										if !valid {
											r.errCh <- errs.E(ctx, errs.KindIO, fmt.Sprintf("content hashing stage [%d] with processing file [%s]: invalid checksum", index, cid.fileStat))
											return
										}
										// filtering duplicates based on meta key and checksums
										if inodes := register.CheckIn(cid.fileStat, checksums); len(inodes) > 1 {
											if len(inodes) == 2 {
												for inode, fss := range inodes {
													if inode != cid.fileStat.Inode() {
														select {
														case <-ctx.Done():
															return
														case outputStream <- ContentId{checksums, fss[0]}:
														}
													}
												}
											}
											select {
											case <-ctx.Done():
												return
											case outputStream <- ContentId{checksums, cid.fileStat}:
											}
										}
									}(cid, &wgStageWorkers)
								}
							}
						} else {
							return
						}
					}
				}
			}(ctx, &wgStages, inputStream, i)
		}

	}(ctx)

	// handling results from final hash cropping stream
	go func(ctx context.Context) {
		defer workflow.OnExit(ctx, r.errCh, "Final content filter stage", func() {
			close(r.errCh)
			close(done)
		})
	finalLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case cid, more := <-finalCidsStream:
				if more {
					r.stats.ContentRegister.CheckIn(cid.fileStat, cid.checksums)
				} else {
					break finalLoop
				}
			}
		}
		r.stats.setCompleted()
	}(ctx)

	return done
}

func (r *contentFilter) ErrCh() <-chan errs.Error {
	return r.errCh
}

func (r *contentFilter) Stats() interface{} {
	return &r.stats
}
