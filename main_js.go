// +build js

package main

import (
	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("protofudger", map[string]interface{}{
		"parse": func(d []byte, showAll, showOffsets bool) []string {
			l, err := parseBuffer(d, showAll, showOffsets)
			if err != nil {
				panic(err)
			}

			return l
		},
	})
}
