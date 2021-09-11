package output

import (
	"bufio"
	"context"
	"fmt"
	cou "github.com/nj-eka/fdups/contextutils"
	"github.com/nj-eka/fdups/errs"
	fh "github.com/nj-eka/fdups/fh"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow/filtering"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type SaveDupsReport struct {
	FileName                                     string
	Err                                          errs.Error
	IndexFrom, DupGroupsCount, FilesCount, Bytes int
}

func SaveDupsResults(ctx context.Context, outputDir, outputFilePrefix string, maxCountDupsPerOutputFile int, stats *filtering.ContentFilterStats) <-chan SaveDupsReport {
	ctx = cou.BuildContext(ctx, cou.SetContextOperation("save results"))
	var maxWorkers = runtime.NumCPU() * 64
	dups, isCompleted := stats.GetResult()
	if !isCompleted {
		outputFilePrefix = outputFilePrefix + "_p"
	} else {
		outputFilePrefix = outputFilePrefix + "_f"
	}
	dupGroupsCount := len(dups)
	if dupGroupsCount == 0 {
		return nil
	}
	if ok, _ := fh.IsDirectory(outputDir); !ok {
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			logging.LogError(errs.E(ctx, fmt.Errorf("output to [%s] failed: %w", outputDir, err)))
		}
	}
	sortedMCKeys := dups.GetKeysSortedByMid()
	ts := time.Now().Format("20060102_150405")
	reports := make(chan SaveDupsReport, maxWorkers)
	var (
		wg    sync.WaitGroup
		wPool = make(chan struct{}, maxWorkers)
	)
	fileNum := 0
	for fileNum*maxCountDupsPerOutputFile <= dupGroupsCount {
		fn := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%d.dat", outputFilePrefix, ts, fileNum+1))
		wPool <- struct{}{}
		wg.Add(1)
		go func(filePath string, indexFrom int, res chan<- SaveDupsReport) {
			defer wg.Done()
			defer func() { <-wPool }()
			var dupsGroupsWritten, filesWritten, bytesWritten int
			out := func(err errs.Error) {
				res <- SaveDupsReport{filePath, err, indexFrom, dupsGroupsWritten, filesWritten, bytesWritten}
			}
			file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
			if err != nil {
				out(errs.E(ctx, err))
				return
			}
			defer func() {
				if err := file.Close(); err != nil {
					out(errs.E(ctx, err))
				}
			}()
			writer := bufio.NewWriter(file)
			defer func() {
				if err = writer.Flush(); err != nil {
					out(errs.E(ctx, err))
				}
			}()
			indexTo := indexFrom + maxCountDupsPerOutputFile
			if indexTo > dupGroupsCount {
				indexTo = dupGroupsCount
			}
			for i, mckey := range sortedMCKeys[indexFrom:indexTo] {
				bytes, err := writer.WriteString(fmt.Sprintf("#%d: %d(%d) %s\n", indexFrom+i+1, len(dups[mckey]), registrator.Inofs(dups[mckey]).Length(), mckey))
				if err != nil {
					out(errs.E(ctx, err))
					return
				}
				bytesWritten += bytes
				fss := registrator.Inofs(dups[mckey]).GetFileStatSorted()
				for _, fs := range fss {
					if bytes, err = writer.WriteString(fmt.Sprintf("%s\n", fs)); err != nil {
						out(errs.E(ctx, err))
						return
					}
					filesWritten++
					bytesWritten += bytes
				}
				dupsGroupsWritten++
			}
			out(nil)
		}(fn, fileNum*maxCountDupsPerOutputFile, reports)
		fileNum++
	}
	go func() {
		wg.Wait()
		close(wPool)
		close(reports)
	}()
	return reports
}
