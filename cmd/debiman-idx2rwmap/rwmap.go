// idx2rwmap converts an auxserver index into a file that can be used
// as an Apache RewriteMap. The resulting file contains all possible
// URLs under which manpages can be reached. For a 30MB auxserver
// index, the resulting rwmap is 1.6GB.
//
// The -concurrency option determines how many shards are created in
// -output_dir. To sort and combine the individual shards, use:
//
//    LC_ALL=C sort output.* > /srv/man/rwmap.txt
//
// Usually, the resulting file is then converted to DBM so that Apache
// can quickly look up keys:
//
//    httxt2dbm -i /srv/man/rwmap.txt -o /srv/man/rwmap.dbm
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/Debian/debiman/internal/redirect"
)

var (
	indexPath = flag.String("index",
		"/srv/man/auxserver.idx",
		"Path to an auxserver index generated by debiman")

	concurrency = flag.Int("concurrency",
		0,
		"Number of output files to create in parallel. Defaults to the number of logical CPUs.")

	outputDir = flag.String("output_dir",
		"",
		"Directory in which to store the output.n (with n = 0 to -concurrency) files. Defaults to the working directory")
)

type oncePrinter struct {
	printed  map[string]bool
	w        *bufio.Writer
	idx      redirect.Index
	variants []redirect.IndexEntry
}

func (op oncePrinter) mustPrint(key string, template redirect.IndexEntry) {
	if op.printed[key] {
		return
	}
	filtered := op.idx.Narrow("", template, redirect.IndexEntry{}, op.variants)
	if _, err := op.w.WriteString(key); err != nil {
		log.Fatal(err)
	}
	if err := op.w.WriteByte(' '); err != nil {
		log.Fatal(err)
	}
	if _, err := op.w.WriteString(filtered[0].ServingPath(".html")); err != nil {
		log.Fatal(err)
	}
	if err := op.w.WriteByte('\n'); err != nil {
		log.Fatal(err)
	}
	op.printed[key] = true
}

func printAll(bufw *bufio.Writer, idx redirect.Index, name string) {
	variants := idx.Entries[name]

	op := oncePrinter{
		printed:  make(map[string]bool),
		w:        bufw,
		idx:      idx,
		variants: variants,
	}

	for _, v := range variants {
		suites := []string{v.Suite}
		for name, rewrite := range idx.Suites {
			if rewrite == v.Suite {
				suites = append(suites, name)
			}
		}

		lcName := strings.ToLower(v.Name)

		// case 01
		op.mustPrint(fmt.Sprintf("/%s", lcName),
			redirect.IndexEntry{})

		// case 02
		op.mustPrint(fmt.Sprintf("/%s.%s", lcName, v.Language),
			redirect.IndexEntry{Language: v.Language})

		// case 03
		op.mustPrint(fmt.Sprintf("/%s.%s", lcName, v.Section),
			redirect.IndexEntry{Section: v.Section})

		// case 03
		op.mustPrint(fmt.Sprintf("/%s.%s", lcName, v.Section[:1]),
			redirect.IndexEntry{Section: v.Section[:1]})

		// FreeBSD-style case 03
		op.mustPrint(fmt.Sprintf("/%s/%s", lcName, v.Section),
			redirect.IndexEntry{Section: v.Section})

		// FreeBSD-style case 03
		op.mustPrint(fmt.Sprintf("/%s/%s", lcName, v.Section[:1]),
			redirect.IndexEntry{Section: v.Section[:1]})

		// case 04
		op.mustPrint(fmt.Sprintf("/%s.%s.%s", lcName, v.Section, v.Language),
			redirect.IndexEntry{Language: v.Language, Section: v.Section})

		// case 04
		op.mustPrint(fmt.Sprintf("/%s.%s.%s", lcName, v.Section[:1], v.Language),
			redirect.IndexEntry{Language: v.Language, Section: v.Section[:1]})

		// case 05
		op.mustPrint(fmt.Sprintf("/%s/%s", v.Binarypkg, lcName),
			redirect.IndexEntry{Binarypkg: v.Binarypkg})

		// case 06
		op.mustPrint(fmt.Sprintf("/%s/%s.%s", v.Binarypkg, lcName, v.Language),
			redirect.IndexEntry{Language: v.Language, Binarypkg: v.Binarypkg})

		// case 07
		op.mustPrint(fmt.Sprintf("/%s/%s.%s", v.Binarypkg, lcName, v.Section),
			redirect.IndexEntry{Binarypkg: v.Binarypkg, Section: v.Section})

		// case 07
		op.mustPrint(fmt.Sprintf("/%s/%s.%s", v.Binarypkg, lcName, v.Section[:1]),
			redirect.IndexEntry{Binarypkg: v.Binarypkg, Section: v.Section[:1]})

		// case 08
		op.mustPrint(fmt.Sprintf("/%s/%s.%s.%s", v.Binarypkg, lcName, v.Section, v.Language),
			redirect.IndexEntry{Language: v.Language, Section: v.Section, Binarypkg: v.Binarypkg})

		// case 08
		op.mustPrint(fmt.Sprintf("/%s/%s.%s.%s", v.Binarypkg, lcName, v.Section[:1], v.Language),
			redirect.IndexEntry{Language: v.Language, Section: v.Section[:1], Binarypkg: v.Binarypkg})

		for _, suite := range suites {
			// case 09
			op.mustPrint(fmt.Sprintf("/%s/%s", suite, lcName),
				redirect.IndexEntry{Suite: v.Suite})

			// case 10
			op.mustPrint(fmt.Sprintf("/%s/%s.%s", suite, lcName, v.Language),
				redirect.IndexEntry{Language: v.Language, Suite: v.Suite})

			// case 11
			op.mustPrint(fmt.Sprintf("/%s/%s.%s", suite, lcName, v.Section),
				redirect.IndexEntry{Section: v.Section, Suite: v.Suite})

			// case 11
			op.mustPrint(fmt.Sprintf("/%s/%s.%s", suite, lcName, v.Section[:1]),
				redirect.IndexEntry{Section: v.Section, Suite: v.Suite})

			// case 12
			op.mustPrint(fmt.Sprintf("/%s/%s.%s.%s", suite, lcName, v.Section, v.Language),
				redirect.IndexEntry{Language: v.Language, Section: v.Section, Suite: v.Suite})

			// case 12
			op.mustPrint(fmt.Sprintf("/%s/%s.%s.%s", suite, lcName, v.Section[:1], v.Language),
				redirect.IndexEntry{Language: v.Language, Section: v.Section[:1], Suite: v.Suite})

			// case 13
			op.mustPrint(fmt.Sprintf("/%s/%s/%s", suite, v.Binarypkg, lcName),
				redirect.IndexEntry{Binarypkg: v.Binarypkg, Suite: v.Suite})

			// case 14
			op.mustPrint(fmt.Sprintf("/%s/%s/%s.%s", suite, v.Binarypkg, lcName, v.Language),
				redirect.IndexEntry{Language: v.Language, Binarypkg: v.Binarypkg, Suite: v.Suite})

			// case 15
			op.mustPrint(fmt.Sprintf("/%s/%s/%s.%s", suite, v.Binarypkg, lcName, v.Section),
				redirect.IndexEntry{Section: v.Section, Binarypkg: v.Binarypkg, Suite: v.Suite})

			// case 15
			op.mustPrint(fmt.Sprintf("/%s/%s/%s.%s", suite, v.Binarypkg, lcName, v.Section[:1]),
				redirect.IndexEntry{Section: v.Section[:1], Binarypkg: v.Binarypkg, Suite: v.Suite})

			// case 16
			op.mustPrint(fmt.Sprintf("/%s/%s/%s.%s.%s", suite, v.Binarypkg, lcName, v.Section, v.Language),
				redirect.IndexEntry{Language: v.Language, Binarypkg: v.Binarypkg, Section: v.Section, Suite: v.Suite})

			// case 16
			op.mustPrint(fmt.Sprintf("/%s/%s/%s.%s.%s", suite, v.Binarypkg, lcName, v.Section[:1], v.Language),
				redirect.IndexEntry{Language: v.Language, Binarypkg: v.Binarypkg, Section: v.Section[:1], Suite: v.Suite})
		}
	}
}

func main() {
	flag.Parse()

	idx, err := redirect.IndexFromProto(*indexPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Loaded %d index entries from %q", len(idx.Entries), *indexPath)

	work := make(chan string)
	var wg sync.WaitGroup
	workers := *concurrency
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f, err := os.Create(filepath.Join(*outputDir, "output."+strconv.Itoa(i)))
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			bufw := bufio.NewWriter(f)
			for name := range work {
				printAll(bufw, idx, name)
			}
			if err := bufw.Flush(); err != nil {
				log.Fatal(err)
			}
		}(i)
	}

	for name, _ := range idx.Entries {
		work <- name
	}
	close(work)

	wg.Wait()
}
