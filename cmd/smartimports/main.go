package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/imports"
)

var verbose bool

func main() {
	var targetPath string
	var localPackage string
	var excludedPaths string
	var filter string

	flag.StringVar(&targetPath, "path", ".", "target path to apply smart goimports, can be a file or a directory")
	flag.StringVar(&localPackage, "local", "", "put imports beginning with this string after 3rd-party packages; comma-separated list")
	flag.StringVar(&excludedPaths, "exclude", "", "paths which should be excluded from processing; comma-separated list")
	flag.StringVar(&filter, "filter", "", "regexp of paths which should be processing from processing")
	flag.BoolVar(&verbose, "v", false, "verbose output")

	flag.Parse()

	opts := getDefaultOpts()
	imports.LocalPrefix = localPackage

	excludedPathsList := strings.Split(excludedPaths, ",")
	filteredExcludedPaths := make([]string, 0, len(excludedPathsList))
	for _, path := range excludedPathsList {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		filteredExcludedPaths = append(filteredExcludedPaths, path)
	}

	var filterRegexp *regexp.Regexp
	if filter != "" {
		filterRegexp = regexp.MustCompile(filter)
	}

	err := processDir(targetPath, opts, filteredExcludedPaths, filterRegexp)
	if err != nil {
		fmt.Println("error while formatting:", err.Error())
		os.Exit(1)
	}
}

func processDir(path string, opts *imports.Options, excludedPaths []string, filterRegexp *regexp.Regexp) error {
	return filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if verbose {
			fmt.Println("processing path", path)
		}
		for _, excludedPath := range excludedPaths {
			if strings.HasPrefix(path, excludedPath) {
				if verbose {
					fmt.Println("   skipped because matched this excluded path:", excludedPath)
				}
				return nil
			}
		}
		if info.IsDir() {
			if verbose {
				fmt.Println("   skipped because it's a dir")
			}
			return nil
		}
		if filterRegexp != nil && !filterRegexp.MatchString(info.Name()) {
			if verbose {
				fmt.Println("   skipped because it's matched this filter:", filterRegexp)
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") || !strings.HasSuffix(info.Name(), ".go") {
			if verbose {
				fmt.Println("   skipped because it's not a go file")
			}
			return nil
		}
		if verbose {
			fmt.Println("   formatting")
		}
		return processFile(path, info, opts)
	})
}

func processFile(filename string, info fs.FileInfo, opts *imports.Options) error {
	rawData, err := os.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "os.ReadFile")
	}

	res, err := processData(rawData, opts)
	if err != nil {
		return errors.Wrap(err, "processData")
	}

	err = os.WriteFile(filename, res, info.Mode())
	if err != nil {
		return errors.Wrap(err, "os.WriteFile")
	}
	return nil
}

func processData(src []byte, opts *imports.Options) ([]byte, error) {
	res, err := imports.Process("", src, opts)
	if err != nil {
		return nil, errors.Wrap(err, "imports.Process 1")
	}

	res = removeImportEmptyLines(res)

	res, err = imports.Process("", res, opts)
	if err != nil {
		return nil, errors.Wrap(err, "imports.Process 2")
	}

	return res, nil
}

func removeImportEmptyLines(src []byte) []byte {
	r := bytes.NewBuffer(src)
	w := bytes.NewBuffer(make([]byte, 0, len(src)))

	importsStarted := false
	importsEnded := false

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}

		if importsStarted {
			if !importsEnded {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if strings.HasPrefix(line, ")") {
					importsEnded = true
				}
			}
		} else {
			if strings.HasPrefix(line, "import (") {
				importsStarted = true
			}
		}

		_, _ = w.WriteString(line)
	}

	return w.Bytes()
}

func getDefaultOpts() *imports.Options {
	return &imports.Options{
		TabWidth:   8,
		TabIndent:  true,
		FormatOnly: true,
		Comments:   true,
	}
}
