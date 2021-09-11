package main

import (
	"context"
	"fmt"
	conf "github.com/heetch/confita"
	"github.com/heetch/confita/backend/file"
	"github.com/heetch/confita/backend/flags"
	cu "github.com/nj-eka/fdups/contextutils"
	erf "github.com/nj-eka/fdups/errflow"
	"github.com/nj-eka/fdups/errs"
	"github.com/nj-eka/fdups/fh"
	fs "github.com/nj-eka/fdups/filestat"
	"github.com/nj-eka/fdups/logging"
	out "github.com/nj-eka/fdups/output"
	"github.com/nj-eka/fdups/registrator"
	"github.com/nj-eka/fdups/workflow"
	"github.com/nj-eka/fdups/workflow/filtering"
	"github.com/nj-eka/fdups/workflow/searching"
	"github.com/nj-eka/fdups/workflow/validating"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"os/user"
	fp "path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRoot                   = ""
	DefaultPattern                = "**/*"
	DefaultOutputDir              = ""
	DefaultMaxGroupsPerOutputFile = 100
)

var (
	AppName                 = fp.Base(os.Args[0])
	DefaultConfigFile       = "fdups.yml" // fmt.Sprintf("%s.yml", AppName)
	DefaultLogFile          = fmt.Sprintf("%s.log", AppName)
	DefaultTraceFile        = fmt.Sprintf("%s.trace.out", AppName)
	DefaultOutputFilePrefix = AppName
)

type Config struct {
	//// 0. logging
	// Path to log output file; empty = os.Stdout
	LogFile string `config:"log,description=Path to log output file; empty = os.Stdout" yaml:"log_file"`
	// logrus logging levels: panic, fatal, error, warn / warning, info, debug, trace
	LogLevel string `config:"log_level,short=l,description=Logging level: panic fatal error warn info debug trace" yaml:"log_level"`
	// supported logging formats: text, json
	LogFormat string `config:"log_format,description=Logging format: text json" yaml:"log_format"`
	//// 0.1 trace
	// Trace file; tracing is on if LogLevel = trace; empty = os.Stderr
	TraceFile string `config:"trace,description=Trace file; tracing is on if LogLevel = trace; empty = os.Stderr" yaml:"trace_file"`

	// List of dirs to search. Order sets priority of sorting found duplicates.
	Roots []string `config:"roots,short=r,description=List of dirs to search. Order sets priority of sorting found duplicates. Empty = pwd." yaml:"roots"`
	// Glob patterns (including ** and {}) to search in roots.
	// note: confita pkg make slice by comma separated list on flags backend (so as current workaround specify {,} in config file)
	Patterns []string `config:"patterns,short=p,description=Glob patterns (including ** and {}) to search in roots. default: **/*" yaml:"patterns"`

	// Min file size to search
	MinSize int64 `config:"min,description=Min file size to search" yaml:"min_size"`
	// Max file size to search, -1 = no upper limit
	MaxSize int64 `config:"max,description=Max file size to search, -1 = no upper limit" yaml:"max_size"`

	// Include symbolic links in processing.
	// When searching , grouping and sorting results, the name and path from the link itself is used.
	//When processing the content for links, the inode of the final regular file that the link points to is used. when grouping and sorting results, the name and path from the link itself is used.
	SLinkEnabled bool `config:"slink,description=Include symbolic links in processing" yaml:"slink_enabled"`

	// Dup grouping based on meta info
	// string combination of file base (n)ame, (m)odification time, (p)ermition, owner (u)ser, owner (g)roup - note (s)ize id on by definition
	// example: "nu" - for additional (by size and content itself) duplicates grouping by name and owner user
	MetaGroupping string `config:"mg,description=Dup grouping based on meta info; string combination of file base (n)ame - (m)odification time - (p)ermition owner - (u)ser owner - (g)roup" yaml:"meta_groups"`

	// Run mode without saving results to files
	IsDry bool `config:"dry,description=Run mode without saving duplications into files" yaml:"is_dry"`

	// Dir for output results.
	OutputDir string `config:"output,description=Output dir for found duplication results" yaml:"output_dir"`
	// Base prefix of output file (in output dir)
	OutputFilePrefix string `config:"prefix,description=Base prefix of output file in output dir" yaml:"output_file_prefix"`
	// Maximum number of groups of duplicates per output file
	MaxGroupsPerOutputFile int `config:"groups,description=Maximum number of groups of duplicates per output file" yaml:"output_groups_per_file"`

	// Head hash filter settings in format [algo;size]
	HeadHashing string `config:"head,description=Head hash filter settings in format [algo;size]" yaml:"head_hashing"`
	// Tail hash filter settings in format [algo;size]
	TailHashing string `config:"tail,description=Tail hash filter settings in format [algo;size]" yaml:"tail_hashing"`
	// Final hash filter settings in format [algo]
	FullHashing string `config:"full,description=Final hash filter settings in format [algo]" yaml:"full_hashing"`
	// Prefilter (head/tail) size is given in file blocks (otherwise in bytes)
	SizeInBlocks bool `config:"blocks,description=Prefilter (head/tail) size is given in file blocks (otherwise in bytes)" yaml:"size_in_blocks"`

	// Statistics update rate (how often stats are printed out to os.Stdout)
	StatsUpdateRate time.Duration `config:"refresh,description=Statistics update rate (how often stats are printed out to os.Stdout)" yaml:"stats_update_rate"`

	// various initial map length settings
	// Estimated number of files found
	PatternFoundFilesInitCapacity int
	// Estimated number of duplicate groups
	DupGroupsInitCapacity int
}

// default config values
var cfg = Config{
	LogFile:   DefaultLogFile,
	LogLevel:  logging.DefaultLevel.String(),
	LogFormat: logging.DefaultFormat,
	TraceFile: DefaultTraceFile,

	Roots:    []string{DefaultRoot},
	Patterns: []string{DefaultPattern},

	MinSize: 1,
	MaxSize: -1,

	MetaGroupping: "", // default: meta dup grouping is only by size

	IsDry: false,

	OutputDir:              DefaultOutputDir,
	OutputFilePrefix:       DefaultOutputFilePrefix,
	MaxGroupsPerOutputFile: DefaultMaxGroupsPerOutputFile,

	HeadHashing: "", // off by default
	TailHashing: "", // off by default
	FullHashing: fs.SHA256,

	SizeInBlocks: false,

	StatsUpdateRate: 5 * time.Second,

	// some internal optimization params
	PatternFoundFilesInitCapacity: 1024 * 256,
	DupGroupsInitCapacity:         1024,
}

var (
	startTime                            time.Time
	currentUser                          *user.User
	statValidatorFunc                    fs.FileStatValidatorFunc
	statMetaKeyFunc                      fs.MetaKeyFunc
	skipPrefiltersMaxSizeFunc            fs.FileSizeLesserFunc
	priorDupsFunc                        fs.PriorFunc
	hashFilterFuncs                      []fs.HashFileFunc
	prefilterHeadSize, prefilterTailSize int64
	minSize2Prefilters                   int64 // = 1 * (prefilterHeadSize + prefilterTailSize)
)

func init() {
	startTime = time.Now()
	ctx := cu.BuildContext(context.Background(), cu.SetContextOperation("00.init"))
	loader := conf.NewLoader(
		file.NewBackend(DefaultConfigFile),
		// env.NewBackend(), - no use cases yet
		flags.NewBackend(),
	)
	if err := loader.Load(ctx, &cfg); err != nil {
		logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid config: %w", err))
		log.Exit(1)
	}
	if err := logging.Initialize(ctx, cfg.LogFile, cfg.LogLevel, cfg.LogFormat, cfg.TraceFile, currentUser); err != nil {
		logging.LogError(err)
		log.Exit(1)
	}

	var (
		err      error
		ok       bool
		patterns = make([]string, 0, len(cfg.Patterns))
	)
	for i, root := range cfg.Roots {
		if root, err = fh.ResolvePath(root, currentUser); err == nil {
			if ok, err = fh.IsDirectory(root); err == nil && ok {
				cfg.Roots[i] = root
			}
		}
		if err != nil {
			logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid root: %w", err))
			log.Exit(1)
		}
	}
	for _, pattern := range cfg.Patterns {
		if patternExts, err := fh.ExpandPatternLists(pattern); err == nil {
			patterns = append(patterns, patternExts...)
		} else {
			logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid pattern: %w", err))
			log.Exit(1)
		}
	}
	cfg.Patterns = patterns

	if cfg.OutputDir, err = fh.ResolvePath(cfg.OutputDir, currentUser); err != nil {
		logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid pattern: %w", err))
		log.Exit(1)
	}
	if err = os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("create output dir [%s] failed: %w", cfg.OutputDir, err))
		log.Exit(1)
	}

	statValidatorFunc = fs.NewRegularSizeStatValidator(cfg.MinSize, cfg.MaxSize)

	metaGroups := map[rune]bool{'s': true}
	for _, rc := range strings.ToLower(cfg.MetaGroupping) {
		metaGroups[rc] = true
	}
	statMetaKeyFunc = fs.NewMetaKeyFunc(metaGroups['s'], metaGroups['n'], metaGroups['p'], metaGroups['u'], metaGroups['g'], metaGroups['m'])

	if len(cfg.HeadHashing) > 0 {
		if parts := strings.Split(cfg.HeadHashing, ";"); len(parts) == 2 {
			if size, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				if hasher, err := fs.GetHashFileFunc(parts[0], size, cfg.SizeInBlocks); err == nil {
					hashFilterFuncs = append(hashFilterFuncs, hasher)
					prefilterHeadSize = size
				} else {
					logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("head hashing init [%v] failed: %w", parts, err))
					log.Exit(1)
				}
			} else {
				logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid head hashing size [%s]", parts[1]))
				log.Exit(1)
			}
		} else {
			logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid head hashing settings [%s]", cfg.HeadHashing))
			log.Exit(1)
		}
	}

	if len(cfg.TailHashing) > 0 {
		if parts := strings.Split(cfg.TailHashing, ";"); len(parts) == 2 {
			if size, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				if hasher, err := fs.GetHashFileFunc(parts[0], -size, cfg.SizeInBlocks); err == nil {
					hashFilterFuncs = append(hashFilterFuncs, hasher)
					prefilterTailSize = size
				} else {
					logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("tail hashing init [%v] failed: %w", parts, err))
					log.Exit(1)
				}
			} else {
				logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid tail hashing size [%s]", parts[1]))
				log.Exit(1)
			}
		} else {
			logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("invalid tail hashing settings [%s]", cfg.TailHashing))
			log.Exit(1)
		}
	}

	if hasher, err := fs.GetHashFileFunc(cfg.FullHashing, 0, cfg.SizeInBlocks); err == nil {
		hashFilterFuncs = append(hashFilterFuncs, hasher)
		minSize2Prefilters = 1 * (prefilterHeadSize + prefilterTailSize) // 1.5 2 ...
	} else {
		logging.LogError(ctx, errs.SeverityCritical, errs.KindInvalidValue, fmt.Errorf("result hashing init [%s] failed: %w", cfg.FullHashing, err))
		log.Exit(1)
	}

	skipPrefiltersMaxSizeFunc = fs.NewFileSizeLesserFunc(minSize2Prefilters, cfg.SizeInBlocks)

	priorDupsFunc = fs.NewPriorFunc(cfg.Roots)
}

func main() {
	defer logging.Finalize()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt) //, syscall.SIGINT, syscall.SIGQUIT)
	ctx = cu.BuildContext(ctx, cu.SetContextOperation("0.main"))
	logging.Msg(ctx).Debug("start listening for signals")
	go func() {
		<-ctx.Done()
		cancel() // stop listening for signed signals asap
		logging.Msg(ctx).Debugf("stop listening for signed signals: %v", ctx.Err())
	}()
	defer cancel() // in case of early return (on error) - signal to close already running goroutines

	// init pipeline
	searcher := searching.NewSearcher(
		ctx,
		cfg.Roots,
		cfg.Patterns,
		cfg.PatternFoundFilesInitCapacity,
	)
	validator := validating.NewValidator(
		ctx,
		searcher.FoundFilePathsCh(),
		statMetaKeyFunc,
		priorDupsFunc,
		statValidatorFunc,
		cfg.SLinkEnabled,
		cfg.PatternFoundFilesInitCapacity,
	)
	metafilter := filtering.NewMetaFilter(
		ctx,
		validator.ValidatedFileStatCh(),
		cfg.PatternFoundFilesInitCapacity,
	)
	contentfilter := filtering.NewContentFilter(
		ctx,
		metafilter.DuplicateCh(),
		metafilter.Stats().(registrator.MifsRegister),
		hashFilterFuncs,
		skipPrefiltersMaxSizeFunc,
		cfg.DupGroupsInitCapacity,
	)
	errmoder, err := erf.NewErrorModerator(
		ctx,
		cancel,
		searcher,
		validator,
		metafilter,
		contentfilter,
	)
	if err != nil {
		logging.LogError(err)
		return
	}
	pipeline := []workflow.Pipeliner{
		searcher,
		validator,
		metafilter,
		contentfilter,
		errmoder,
	}

	// launch pipeline
	finish := workflow.Run(ctx, workflow.Pipelines(pipeline).Runners()...)

monitoring:
	for {
		select {
		case <-finish:
			break monitoring
		case <-ctx.Done():
			fmt.Println("\nProcessing stopped")
			if !cfg.IsDry {
				fmt.Println("Do you wish to save reports of already found duplicates? [Y]es / [N]o")
				var answer string
				if n, err := fmt.Scanln(&answer); n >= 1 && err == nil {
					if strings.ToUpper(strings.SplitN(answer, "", 1)[0]) == "Y" {
						SaveResults(ctx, contentfilter.Stats().(*filtering.ContentFilterStats))
					}
				}
			}
			<-finish
			return
		case <-time.After(cfg.StatsUpdateRate):
			out.PrintMonitors(ctx, startTime, workflow.Pipelines(pipeline).StatProducers()...)
		}
	}
	out.PrintMonitors(ctx, startTime, workflow.Pipelines(pipeline).StatProducers()...)
	if !cfg.IsDry {
		SaveResults(ctx, contentfilter.Stats().(*filtering.ContentFilterStats))
	}
}

func SaveResults(ctx context.Context, dups *filtering.ContentFilterStats) {
	reports := out.SaveDupsResults(ctx, cfg.OutputDir, cfg.OutputFilePrefix, cfg.MaxGroupsPerOutputFile, dups)
	for report := range reports {
		if report.Err != nil {
			logging.LogError(report.Err)
		} else {
			logging.Msg(ctx).Info(
				fmt.Sprintf("results witten to file [%s]: %d(indexFrom) %d(dups) %d(files) %d(bytes)",
					report.FileName,
					report.IndexFrom,
					report.DupGroupsCount,
					report.FilesCount,
					report.Bytes,
				))
		}
	}
}
