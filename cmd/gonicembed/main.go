package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff"
	"github.com/pkg/errors"
)

// once i had this written with ~100% text/template but it was very
// slow. now this thing is not nice on the eyes or easy to change
// but it's pretty fast. which i needed it to for live reloading stuff

const (
	programName = "gonicembed"
	byteCols    = 24

	// begin file template
	fileHeader = `// file generated with embed tool
// do not edit

// %s

package %s
import "time"
type EmbeddedAsset struct {
	ModTime time.Time
	Bytes []byte
}
var %s = map[string]*EmbeddedAsset{`
	fileFooter = `
}`

	// begin asset template
	assetHeader = `
%q: &EmbeddedAsset{
	ModTime: time.Unix(%d, 0),
	Bytes: []byte{
`
	assetFooter = `}},`
)

type config struct {
	packageName     string
	outPath         string
	tagList         string
	assetsVarName   string
	assetPathPrefix string
}

type file struct {
	data    io.ReadSeeker
	path    string
	modTime time.Time
}

//nolint:errcheck
func processAsset(c *config, f *file, out io.Writer) {
	out.Write([]byte(fmt.Sprintf(assetHeader,
		strings.TrimPrefix(f.path, c.assetPathPrefix),
		f.modTime.Unix(),
	)))
	defer out.Write([]byte(assetFooter))
	buffer := make([]byte, byteCols)
	for {
		read, err := f.data.Read(buffer)
		for i := 0; i < read; i++ {
			fmt.Fprintf(out, "0x%02x,", buffer[i])
		}
		if err != nil {
			break
		}
		fmt.Fprintln(out)
	}
}

//nolint:errcheck
func processAssets(c *config, files []string) error {
	outWriter, err := os.Create(c.outPath)
	if err != nil {
		return errors.Wrap(err, "creating out path")
	}
	if c.tagList != "" {
		c.tagList = fmt.Sprintf("+build %s", c.tagList)
	}
	outWriter.Write([]byte(fmt.Sprintf(fileHeader,
		c.tagList,
		c.packageName,
		c.assetsVarName,
	)))
	defer outWriter.Write([]byte(fileFooter))
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return errors.Wrap(err, "stating asset")
		}
		if info.IsDir() {
			continue
		}
		data, err := os.Open(path)
		if err != nil {
			return errors.Wrap(err, "opening asset")
		}
		processAsset(
			c,
			&file{
				data:    data,
				path:    path,
				modTime: info.ModTime(),
			},
			outWriter,
		)
	}
	return nil
}

func main() {
	set := flag.NewFlagSet(programName, flag.ExitOnError)
	outPath := set.String(
		"out-path", "",
		"generated file's path (required)")
	pkgName := set.String(
		"package-name", "assets",
		"generated file's package name")
	tagList := set.String(
		"tag-list", "",
		"generated file's build tag list")
	assetsVarName := set.String(
		"assets-var-name", "Assets",
		"generated file's assets var name")
	assetPathPrefix := set.String(
		"asset-path-prefix", "",
		"generated file's assets map key prefix to be stripped")
	if err := ff.Parse(set, os.Args[1:]); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}
	if *outPath == "" {
		log.Fatalln("invalid arguments. see -h")
	}
	c := &config{
		packageName:     *pkgName,
		outPath:         *outPath,
		tagList:         *tagList,
		assetsVarName:   *assetsVarName,
		assetPathPrefix: *assetPathPrefix,
	}
	if err := processAssets(c, set.Args()); err != nil {
		log.Fatalf("error processing files: %v\n", err)
	}
}
