// +build !js

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

var (
	showAll     = flag.Bool("show_all", false, "Show all possible types instead of just most likely.")
	showOffsets = flag.Bool("show_offsets", false, "Show byte offsets of each value.")
)

func parseStream(rd io.Reader, showAll, showOffsets bool) ([]string, error) {
	d, err := ioutil.ReadAll(rd)
	if err != nil {
		return nil, err
	}

	return parseBuffer(d, showAll, showOffsets)
}

func parseAndPrint(rd io.Reader, showAll, showOffsets bool) error {
	lines, err := parseStream(rd, showAll, showOffsets)
	if err != nil {
		return err
	}

	for _, l := range lines {
		if _, err := fmt.Println(l); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		if err := parseAndPrint(os.Stdin, *showAll, *showOffsets); err != nil {
			panic(err)
		}
	}

	for _, f := range flag.Args() {
		fmt.Printf("parsing %s\n", f)

		func() {
			fd, err := os.Open(f)
			if err != nil {
				panic(err)
			}
			defer fd.Close()

			if err := parseAndPrint(fd, *showAll, *showOffsets); err != nil {
				panic(err)
			}
		}()
	}
}
