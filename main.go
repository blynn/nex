package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

var outFilename string
var nfadotFile, dfadotFile string
var autorun, standalone, customError bool
var prefix string

var prefixReplacer *strings.Replacer

func init() {
	prefixReplacer = strings.NewReplacer()
}

func main() {
	flag.StringVar(&prefix, "p", "yy", "name prefix to use in generated code")
	flag.StringVar(&outFilename, "o", "", `output file`)
	flag.BoolVar(&standalone, "s", false, `standalone code; NN_FUN macro substitution, no Lex() method`)
	flag.BoolVar(&customError, "e", false, `custom error func; no Error() method`)
	flag.BoolVar(&autorun, "r", false, `run generated program`)
	flag.StringVar(&nfadotFile, "nfadot", "", `show NFA graph in DOT format`)
	flag.StringVar(&dfadotFile, "dfadot", "", `show DFA graph in DOT format`)
	flag.Parse()

	if len(prefix) > 0 {
		prefixReplacer = strings.NewReplacer("yy", prefix)
	}

	nfadot = createDotFile(nfadotFile)
	dfadot = createDotFile(dfadotFile)
	defer func() {
		if nfadot != nil {
			dieErr(nfadot.Close(), "Close")
		}
		if dfadot != nil {
			dieErr(dfadot.Close(), "Close")
		}
	}()
	infile, outfile := os.Stdin, os.Stdout
	var err error
	if flag.NArg() > 0 {
		dieIf(flag.NArg() > 1, "nex: extraneous arguments after", flag.Arg(0))
		dieIf(strings.HasSuffix(flag.Arg(0), ".go"), "nex: input filename ends with .go:", flag.Arg(0))
		basename := flag.Arg(0)
		n := strings.LastIndex(basename, ".")
		if n >= 0 {
			basename = basename[:n]
		}
		infile, err = os.Open(flag.Arg(0))
		dieErr(err, "nex")
		defer infile.Close()
		if !autorun {
			if outFilename == "" {
				outFilename = basename + ".nn.go"
				outfile, err = os.Create(outFilename)
			} else {
				outfile, err = os.Create(outFilename)
			}
			dieErr(err, "nex")
			defer outfile.Close()
		}
	}
	if autorun {
		tmpdir, err := ioutil.TempDir("", "nex")
		dieIf(err != nil, "tempdir:", err)
		defer func() {
			dieErr(os.RemoveAll(tmpdir), "RemoveAll")
		}()
		outfile, err = os.Create(tmpdir + "/lets.go")
		dieErr(err, "nex")
		defer outfile.Close()
	}
	err = process(outfile, infile)
	if err != nil {
		log.Fatal(err)
	}
	if autorun {
		c := exec.Command("go", "run", outfile.Name())
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		dieErr(c.Run(), "go run")
	}
}
