package output

import (
	"bufio"
	"context"
	"fmt"
	cu "github.com/nj-eka/fdups/contextutils"
	"github.com/nj-eka/fdups/errflow"
	fh "github.com/nj-eka/fdups/fh"
	"github.com/nj-eka/fdups/logging"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"github.com/nj-eka/fdups/workflow/filtering"
	"github.com/nj-eka/fdups/workflow/searching"
	"github.com/nj-eka/fdups/workflow/validating"
	"gonum.org/v1/gonum/stat"
	"os"
	"runtime"
	"sort"
	"time"
)

const (
	upLeft     = "\n\033[H\033[2J"
	colorReset = "\033[0m"

	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	//colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	//colorWhite  = "\033[37m"
)

func PrintMonitors(ctx context.Context, startTime time.Time, statProducers ...workflow.StatProducer) {
	ctx = cu.BuildContext(nil, cu.AddContextOperation("print_monitors"))
	bufOut := bufio.NewWriter(os.Stdout)
	bout := func(s string) {
		if _, err := bufOut.WriteString(s); err != nil {
			logging.LogError(ctx, fmt.Sprintf("bufio write string [%s] failed: %w", s, err))
		}
	}
	var (
		foundPaths, validFileStats, validInodes, errsStats registrator.Encounter
		dups                                               *filtering.ContentFilterStats
	)
	for _, statProducer := range statProducers {
		switch st := statProducer.Stats().(type) {
		case searching.SearcherStats:
			foundPaths = st
		case *validating.ValidatorStats:
			validFileStats = st.FileStats
			validInodes = st.InodeStats
		case errflow.ErrsStats:
			errsStats = st
		case *filtering.ContentFilterStats:
			dups = st
		}
	}
	bout(upLeft)
	isCompleted := dups.IsCompleted()
	if !isCompleted {
		bout(fmt.Sprintln("---------- Processing stats --------------"))
	} else {
		bout(fmt.Sprintln("==========    Final stats   =============="))
	}
	bout(fmt.Sprintln("Time elapsed: ", time.Since(startTime).Round(time.Second)))

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	bout(fmt.Sprint(colorCyan, "Runtime mem usage:"))
	bout(fmt.Sprintf("\tAlloc = %v", fh.BytesToHuman(ms.Alloc)))           // Alloc is bytes of allocated heap objects. HeapAlloc is bytes of allocated heap objects.
	bout(fmt.Sprintf("\tTotalAlloc = %v", fh.BytesToHuman(ms.TotalAlloc))) // TotalAlloc is cumulative bytes allocated for heap objects.
	bout(fmt.Sprintf("\tSys = %v", fh.BytesToHuman(ms.Sys)))               // Sys is the total bytes of memory obtained from the OS.
	bout(fmt.Sprintf("\tMallocs = %v", fh.BytesToHuman(ms.Mallocs)))       // Mallocs is the cumulative count of heap objects allocated.
	bout(fmt.Sprintf("\tFrees = %v", fh.BytesToHuman(ms.Frees)))           // Frees is the cumulative count of heap objects freed.
	bout(fmt.Sprintf("\tGCSys = %v", fh.BytesToHuman(ms.GCSys)))           // GCSys is bytes of memory in garbage collection metadata.
	bout(fmt.Sprintf("\tNumGC = %v\n", ms.NumGC))

	bout(fmt.Sprint(colorBlue, "Search & validation:"))
	if foundPaths != nil {
		bout(fmt.Sprintf("\t%12d/%d files (found/unique)", foundPaths.TotalCount(), foundPaths.KeysCount()))
	}
	if validFileStats != nil {
		uniqueSizes, _ := registrator.GetKeySizes(validFileStats.GetScores())
		bout(fmt.Sprintf("\t%8d(%v) validated", validFileStats.KeysCount(), fh.BytesToHuman(uint64(uniqueSizes))))
	}
	if validInodes != nil {
		uniqueSizes, _ := registrator.GetKeySizes(validInodes.GetScores())
		bout(fmt.Sprintf("\t%11d(%v) inodes\n", validInodes.KeysCount(), fh.BytesToHuman(uint64(uniqueSizes))))
	}
	if dups != nil {
		bout(fmt.Sprintln("sizing (quantiles):"))
		sizesScore := dups.MetaRegister.GetSizesCounter().GetScores()
		PrintFilesStat(sizesScore, "\t", bufOut)

		bout(fmt.Sprintln(colorGreen, "\nHash filters:"))
		for stageNumber, stageInodesStat := range dups.StageInodeStats {
			inodesCount, totalSize := stageInodesStat.GetStats()
			stageGroupsCount := dups.StageRegisters[stageNumber].GetKeysCounter().KeysCount()
			bout(fmt.Sprintf("\t[%2d]: %8d(groups) %8d(inodes) %12v(read)\n", stageNumber, stageGroupsCount, inodesCount, fh.BytesToHuman(uint64(totalSize))))
		}

		bout(fmt.Sprintln(colorPurple, "\nDuplicates found:"))
		keysCounter := dups.ContentRegister.GetKeysCounter()
		scores := keysCounter.GetScores()
		uniqueSizes, totalSizes := registrator.GetKeySizes(scores)
		bout(fmt.Sprintf("\t%14d(groups) %8d(inodes) %12v(unique) %12v(total) %12v(can be freed)\n", keysCounter.KeysCount(), keysCounter.TotalCount(), fh.BytesToHuman(uint64(uniqueSizes)), fh.BytesToHuman(uint64(totalSizes)), fh.BytesToHuman(uint64(totalSizes-uniqueSizes))))
		bout(fmt.Sprintln("sizing (quantiles):"))
		PrintFilesStat(scores, "\t", bufOut)
	}
	if errsStats != nil {
		cp := errsStats.GetCounterPairs()
		if len(cp) > 0 {
			bout(fmt.Sprintln(colorRed, "\nErrors:"))
			sort.Sort(registrator.CounterPairsByKey(cp))
			for _, cp := range cp {
				esk := cp.Key.(errflow.ErrStatKey)
				bout(fmt.Sprintf(" *%-8s: %-48s # %4d - %s\n", esk.Severity, esk.Operations, cp.Count, esk.Kind))
			}
		}
	}
	bout(fmt.Sprint(colorReset))
	if err := bufOut.Flush(); err != nil {
		logging.LogError(ctx, fmt.Errorf("bufio flush failed: %w", err))
	}
}

func PrintFilesStat(sizesScore map[interface{}]int, tab string, bufout *bufio.Writer) {
	numSizes := len(sizesScore)
	if numSizes == 0 {
		return
	}
	sizes, counts := make([]float64, 0, numSizes), make([]float64, 0, numSizes)
	for size, count := range sizesScore {
		if ks, ok := size.(registrator.KeySize); ok {
			size = ks.Size
		}
		sizes = append(sizes, float64(size.(int64)))
		counts = append(counts, float64(count))
	}
	sort.SliceStable(counts, func(i, j int) bool { return sizes[i] < sizes[j] })
	sort.SliceStable(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	dividers := []float64{
		sizes[0],
		stat.Quantile(0.25, stat.Empirical, sizes, counts),
		stat.Quantile(0.5, stat.Empirical, sizes, counts),
		stat.Quantile(0.75, stat.Empirical, sizes, counts),
		sizes[numSizes-1] + 1,
	}
	hist := stat.Histogram(nil, dividers, sizes, counts)
	for i := 0; i < len(hist); i++ {
		if hist[i] > 0 {
			_, _ = bufout.WriteString(fmt.Sprintf("%s%-5.0f:%12.0f-%-12.0f\n", tab, hist[i], dividers[i], dividers[i+1]))
		}
	}
}
