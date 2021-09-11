## fdups
Сommand line utility that searches for duplicate files in specified directories [roots] 
matching with [patterns] based on equality of file content as well as various customizable combinations of meta information.

Main priority of this program is fast finding duplicates (trying to get the most out of *Go*) 
with flexible settings and at the same time with full monitoring of processing progress / statistics on all stages
(which often comes at the expense of speed))

Idea inspired by https://github.com/pauldreik/rdfind

### Features:
- glob patterns with extended support **[**]** and classes **{ ... [, ...] }**;
- glob search is concurrent, found paths are returned asap (not blocked) for further processing;
- correct path resolving in sudo mode
- grouping of duplicates can be refined based on coincidence of file meta info combinations 
such as base name, modification date, owner user / group, permissions (use of size is assumed)
- to resolve links, program deals with file inodes internally
- to speed up content filtering (especially for large files) uses multi-stage filters, 
based on hash file head and / or tail checksums ("pre-filters") that can be enabled by specifying size and hashing algorithm,
hashing algo used on final filtering stage (for full file content) also can be specified;
- during program execution, user receives all necessary processing statistics and 
can interrupt execution to get intermediate results;
- results are saved to text file(s) as grouped sorted list of found duplicates;
- in order to facilitate making further decision on duplicates (program does not delete anything!) multilevel sorting of results is used; 
  - at top level, dup groups are sorted by metakey (size, mt, etc), 
  - within dup groups, duplicates are grouped by priority ([roots] membership), file type (regular / link), modification time, path depth etc.

### Install and usage:
    > git clone github.com/nj-eka/fdups
    > cd fdups
    > go mod tidy
    > go build .
    > ./fdups --help
    Usage of ./fdups:
      -blocks
        Prefilter (head/tail) size is given in file blocks (otherwise in bytes)
      -dry
        Run mode without saving duplications into files
      -full string
        Final hash filter settings in format [algo] (default "sha256")
      -head string
        Head hash filter settings in format [algo;size]
      -l string
        Logging level: panic fatal error warn info debug trace (short) (default "info")
      -log string
        Path to log output file; empty = os.Stdout (default "fdups.log")
      -log_format string
        Logging format: text json (default "text")
      -log_level string
        Logging level: panic fatal error warn info debug trace (default "info")
      -max int
        Max file size to search, -1 = no upper limit (default -1)
      -mg string
        Dup grouping based on meta info; string combination of file base (n)ame - (m)odification time - (p)ermition owner - (u)ser owner - (g)roup
      -min int
        Min file size to search (default 1)
      -output_dir string
        Output dir for found duplication results
      -prefix string
        Base prefix of output file in output dir (default "fdups")
      -groups int
        Maximum number of groups of duplicates per output file (default 100)
      -patterns value
        Glob patterns (including ** and {}) to search in roots. (default **/*)
      -refresh duration
        Statistics update rate (how often stats are printed out to os.Stdout) (default 5s)
      -roots value
        List of dirs to search. Order sets priority of sorting found duplicates. Empty = pwd. (default "")
      -tail string
        Tail hash filter settings in format [algo;size]
      -trace string
        Trace file; tracing is on if LogLevel = trace; empty = os.Stderr (default "fdups.trace.out")


### Output example:
    os.Stdout:

    ==========    Final stats   ==============
    Time elapsed:  5s
    Runtime mem usage:      Alloc = 236 MiB TotalAlloc = 1.3 GiB    Sys = 542 MiB   Mallocs = 7.2 MiB       Frees = 5.4 MiB GCSys = 22 MiB  NumGC = 10
    Search & validation:               0/0 files (found/unique)        58956(786 MiB) validated           58944(786 MiB) inodes
    sizing (quantiles):
    14738:          10-3551        
    14737:        3551-8200        
    14740:        8200-19351       
    14741:       19351-9258642
    
    Hash filters:
    [ 0]:    24704(groups)    49207(inodes)      3.0 MiB(read)
    [ 1]:     9481(groups)    33312(inodes)      4.1 MiB(read)
    [ 2]:    10527(groups)    35859(inodes)      344 MiB(read)
    
    Duplicates found:
    8999(groups)    34335(inodes)      100 MiB(unique)      341 MiB(total)      241 MiB(can be freed)
    sizing (quantiles):
    8581 :          10-882         
    8586 :         882-2609        
    8583 :        2609-6938        
    8585 :        6938-5447984
___  
    output files:

    #1: 3(3) mid{size:     5447983;mt:*;uid:*;gid:*;perm:*;name:*};cid{-&0:64:md5:affbc761b58b05c7a72c97adeae3541b&1:128:sha1:b8907e8e731c470d9e94e7540824999fe034a4ab&2:5447983:sha256:a78a559398239038f67c5737bc73b3674f74eccfcaa2a0339c49af904495dfee}
    10753278( 1)|-r--r--r--|     5447983|Thu, 08 Jul 2021 16:37:09 MSK|UUU:GGG|XXX/go/pkg/mod/golang.org/x/text@v0.3.4/date/tables.go
    11671045( 1)|-r--r--r--|     5447983|Fri, 03 Sep 2021 10:49:57 MSK|UUU:GGG|XXX/go/pkg/mod/golang.org/x/text@v0.3.1-0.20181227161524-e6919f6577db/date/tables.go
    9982144( 1)|-rw-rw-r--|     5447983|Fri, 03 Sep 2021 11:26:19 MSK|UUU:GGG|XXX/go/src/golang.org/x/text/date/tables.go
    ...
    #8901: 2(2) mid{size:          36;mt:*;uid:*;gid:*;perm:*;name:*};cid{-&2:36:sha256:050cb1c50ab73cd5b3a784564c52de4d5d37a613e7e5bc748e7c6f5f2497c295}
    10751052( 1)|-rw-rw-r--|          36|Wed, 07 Jul 2021 14:16:08 MSK|UUU:GGG|XXX/go/pkg/mod/cache/download/github.com/prometheus/common/@v/v0.0.0-20181126121408-4724e9255275.mod
    10761423( 1)|-rw-rw-r--|          36|Wed, 21 Jul 2021 19:05:04 MSK|UUU:GGG|XXX/go/pkg/mod/cache/download/github.com/prometheus/common/@v/v0.0.0-20181113130724-41aa239b4cce.mod

    links example output:
    #1: 3(9) mid{size:       29572;mt:*;uid:*;gid:*;perm:*;name:*};cid{-&0:64:md5:c41e6a113ede3180d4917aa1d725dd82&1:128:sha1:03a26676924b45469937d1abdaeeb3860885b3e5&2:29572:sha256:e6b5118193cc6f9dd061020f86722322b29dac9f94fe897655b589cfcfbc2435}
    9843512( 3)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1
    9843512( 3)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1_h1
    9843512( 3)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1_h2
    9843626( 1)|-rw-rw-r--|       29572|Wed, 04 Aug 2021 20:36:48 MSK|user:group|XXX/dups/ps1_1
    9843696( 1)|-rwxr-xr-x|       29572|Fri, 06 Aug 2021 20:57:07 MSK|root:root|XXX/dups/ps11
    9843524( 1)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/Link to ps1 -> XXX/dups/ps1
    9843525( 1)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1_s1 -> XXX/dups/ps1
    9843526( 1)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1_s2 -> XXX/dups/ps1
    9843618( 1)|-rwxrwxrwx|       29572|Wed, 05 May 2021 07:43:20 MSK|root:root|XXX/dups/ps1_s1_s1 -> XXX/dups/ps1

### Ideas for the future:
- save intermediate results in runtime by sending some (like user1/2) signals to process without interrupting / stopping program
- if it's not about cross-platform, glob function can be rewritten to use os system calls directly (example: https://habr.com/ru/post/281382/)
- add support for finding duplicates based on file types (especially media types with parsing media containers, etc.)
- implement self-tuning on the basis of collected statistics in runtime and os resources, to set optimal parameters (size/algo for pre-filters, number of workers, etc.)
