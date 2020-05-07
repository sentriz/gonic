package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic/version"
)

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) write(buf []byte) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.Write(buf)
}

func (ew *errWriter) printf(format string, a ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, a...)
}

func (ew *errWriter) println(a ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew.w, a...)
}

// once i had this written with ~100% text/template but it was very
// slow. now this thing is not nice on the eyes or easy to change
// but it's pretty fast. which i needed it to for live reloading stuff

const (
	byteCols = 24
	// ** begin file template
	fileHeader = `// file generated with embed tool; DO NOT EDIT.
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
	// ** begin asset template
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

func processAsset(c *config, ew *errWriter, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stating asset: %w", err)
	}
	if info.IsDir() {
		return nil
	}
	data, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("opening asset: %w", err)
	}
	defer data.Close()
	ew.write([]byte(fmt.Sprintf(assetHeader,
		strings.TrimPrefix(path, c.assetPathPrefix),
		info.ModTime().Unix(),
	)))
	buffer := make([]byte, byteCols)
	for {
		read, err := data.Read(buffer)
		for i := 0; i < read; i++ {
			ew.printf("0x%02x,", buffer[i])
		}
		if err != nil {
			break
		}
		ew.println()
	}
	ew.write([]byte(assetFooter))
	return ew.err
}

func processAssets(c *config, files []string) error {
	out, err := os.Create(c.outPath)
	if err != nil {
		return fmt.Errorf("creating out path: %w", err)
	}
	ew := &errWriter{w: out}
	if c.tagList != "" {
		c.tagList = fmt.Sprintf("+build %s", c.tagList)
	}
	ew.write([]byte(fmt.Sprintf(fileHeader,
		c.tagList,
		c.packageName,
		c.assetsVarName,
	)))
	defer ew.write([]byte(fileFooter))
	for _, path := range files {
		if err := processAsset(c, ew, path); err != nil {
			return fmt.Errorf("processing asset: %w", err)
		}
	}
	return ew.err
}

func main() {
	set := flag.NewFlagSet(version.NAME_EMBED, flag.ExitOnError)
	outPath := set.String("out-path", "", "generated file's path (required)")
	pkgName := set.String("package-name", "assets", "generated file's package name")
	tagList := set.String("tag-list", "", "generated file's build tag list")
	assetsVarName := set.String("assets-var-name", "Assets", "generated assets var name")
	assetPathPrefix := set.String("asset-path-prefix", "", "generated assets map key prefix")
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
