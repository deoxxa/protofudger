package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"time"
	"unicode/utf8"
)

var (
	showAll     = flag.Bool("show_all", false, "Show all possible types instead of just most likely.")
	showOffsets = flag.Bool("show_offsets", false, "Show byte offsets of each value.")
)

type pbType int

const (
	typeNumber pbType = 0
	type64Bit  pbType = 1
	typeBytes  pbType = 2
	type32Bit  pbType = 5
)

func formatKey(k uint64, o int64) string {
	if *showOffsets {
		return fmt.Sprintf("%d @ %d", k, o)
	}

	return fmt.Sprintf("%d", k)
}

func decode(input []byte, offset int64, depth int) (int, []string, error) {
	r := bytes.NewReader(input)
	var a []string

	n := 0
	for {
		o, err := r.Seek(0, os.SEEK_CUR)
		if err != nil {
			return n, a, err
		}
		o += offset

		i, err := binary.ReadUvarint(r)
		if err != nil {
			if err == io.EOF {
				return n, a, nil
			}

			return n, a, err
		}

		if i == 0 {
			return n, a, fmt.Errorf("tag was zero")
		}

		t := pbType(i & 0x03)
		k := i >> 3

		if k < 0 {
			return n, a, fmt.Errorf("invalid key %d", k)
		}

		if k > 1024 {
			return n, a, fmt.Errorf("probably invalid key %d", k)
		}

		switch t {
		case typeNumber:
			v, err := binary.ReadUvarint(r)
			if err != nil {
				return n, a, err
			}

			switch {
			case v > 1400000000000 && v < 1500000000000:
				a = append(a, fmt.Sprintf("%s: (varint, microseconds) %s", formatKey(k, o), time.Unix(int64(v/1000), 0)))
			case v > 1400000000 && v < 1500000000:
				a = append(a, fmt.Sprintf("%s: (varint, milliseconds) %s", formatKey(k, o), time.Unix(int64(v), 0)))
			case uint64(v) == v:
				a = append(a, fmt.Sprintf("%s: (varint) %d", formatKey(k, o), v))
			case !*showAll:
				a = append(a, fmt.Sprintf("%s: (varint) %d OR %d", formatKey(k, o), v, uint64(v)))
			}

			if *showAll {
				a = append(a, fmt.Sprintf("%s: (varint) %d OR %d", formatKey(k, o), v, uint64(v)))
			}

		case type64Bit:
			v := make([]byte, 8)
			if _, err := r.Read(v); err != nil {
				return n, a, err
			}

			{
				closestValue := math.MaxFloat64
				closestName := "none"
				closestText := ""

				var s []string

				var fbe float64
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &fbe); err == nil {
					s = append(s, fmt.Sprintf("  (doublebe) %f", fbe))

					if a := math.Abs(fbe); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "doublebe"
						closestText = fmt.Sprintf("%f", fbe)
					}
				}

				var fle float64
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &fle); err == nil {
					s = append(s, fmt.Sprintf("  (doublele) %f", fle))

					if a := math.Abs(fle); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "doublele"
						closestText = fmt.Sprintf("%f", fle)
					}
				}

				var slbe int64
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &slbe); err == nil {
					s = append(s, fmt.Sprintf("  (int64be) %d", slbe))

					if a := math.Abs(float64(slbe)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "int64be"
						closestText = fmt.Sprintf("%d", slbe)
					}
				}

				var slle int64
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &slle); err == nil {
					s = append(s, fmt.Sprintf("  (int64le) %d", slle))

					if a := math.Abs(float64(slle)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "int64le"
						closestText = fmt.Sprintf("%d", slle)
					}
				}

				var ulbe uint64
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &ulbe); err == nil {
					s = append(s, fmt.Sprintf("  (uint64be) %d", ulbe))

					if a := math.Abs(float64(ulbe)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "uint64"
						closestText = fmt.Sprintf("%d", ulbe)
					}
				}

				var ulle uint64
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &ulle); err == nil {
					s = append(s, fmt.Sprintf("  (uint64le) %d", ulle))

					if a := math.Abs(float64(ulle)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "uint64"
						closestText = fmt.Sprintf("%d", ulle)
					}
				}

				a = append(a, fmt.Sprintf("%s: (%s) %s", formatKey(k, o), closestName, closestText))

				if *showAll {
					a = append(a, s...)
				}
			}
		case typeBytes:
			l, err := binary.ReadUvarint(r)
			if err != nil {
				return n, a, err
			}

			if l < 0 {
				return n, a, fmt.Errorf("invalid length %d", l)
			}
			if l > 1024*32 {
				return n, a, fmt.Errorf("probably invalid length %d", l)
			}

			o, err = r.Seek(0, os.SEEK_CUR)
			if err != nil {
				return n, a, err
			}
			o += offset

			v := make([]byte, l)
			if j, err := r.Read(v); err != nil && err != io.EOF {
				return n, a, err
			} else if j < len(v) {
				return n, a, fmt.Errorf("couldn't read enough data")
			}

			_, ma, merr := decode(v, o, depth+1)
			if merr == nil {
				a = append(a, fmt.Sprintf("%s: {", formatKey(k, o)))
				for _, s := range ma {
					a = append(a, "  "+s)
				}
				a = append(a, "}")
			} else {
				if utf8.ValidString(string(v)) {
					a = append(a, fmt.Sprintf("%s: (string) %q", formatKey(k, o), string(v)))
				} else {
					a = append(a, fmt.Sprintf("%s: (bytes) %s", formatKey(k, o), hex.EncodeToString(v)))
				}
			}
		case type32Bit:
			v := make([]byte, 4)
			if _, err := r.Read(v); err != nil {
				return n, a, err
			}

			{
				closestValue := math.MaxFloat64
				closestName := "none"
				closestText := ""

				var s []string

				var fbe float32
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &fbe); err == nil {
					s = append(s, fmt.Sprintf("  (floatbe) %f", fbe))

					if a := math.Abs(float64(fbe)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "floatbe"
						closestText = fmt.Sprintf("%f", fbe)
					}
				}

				var fle float32
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &fle); err == nil {
					s = append(s, fmt.Sprintf("  (floatle) %f", fle))

					if a := math.Abs(float64(fle)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "floatle"
						closestText = fmt.Sprintf("%f", fle)
					}
				}

				var slbe int32
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &slbe); err == nil {
					s = append(s, fmt.Sprintf("  (int32be) %d", slbe))

					if a := math.Abs(float64(slbe)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "int32be"
						closestText = fmt.Sprintf("%f", slbe)
					}
				}

				var slle int32
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &slle); err == nil {
					s = append(s, fmt.Sprintf("  (int32le) %d", slle))

					if a := math.Abs(float64(slle)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "int32le"
						closestText = fmt.Sprintf("%f", slle)
					}
				}

				var ulbe uint32
				if err := binary.Read(bytes.NewReader(v), binary.BigEndian, &ulbe); err == nil {
					s = append(s, fmt.Sprintf("  (uin32be) %d", ulbe))

					if a := math.Abs(float64(ulbe)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "uin32be"
						closestText = fmt.Sprintf("%f", ulbe)
					}
				}

				var ulle uint32
				if err := binary.Read(bytes.NewReader(v), binary.LittleEndian, &ulle); err == nil {
					s = append(s, fmt.Sprintf("  (uin32le) %d", ulle))

					if a := math.Abs(float64(ulle)); a > 0.01 && a < closestValue {
						closestValue = a
						closestName = "uin32le"
						closestText = fmt.Sprintf("%f", ulle)
					}
				}

				a = append(a, fmt.Sprintf("%s: (%s) %s", formatKey(k, o), closestName, closestText))
				if *showAll {
					a = append(a, s...)
				}
			}
		default:
			return n, a, fmt.Errorf("invalid type %s", t, o)
		}

		n++
	}
}

func parseStream(rd io.Reader) error {
	d, err := ioutil.ReadAll(rd)
	if err != nil {
		return err
	}

	n, lines, err := decode(d, 0, 0)
	if n > 0 && err == nil {
		fmt.Printf("decoded %d fields\n", n)

		if err != nil {
			fmt.Printf("error was %s\n", err.Error())
		}

		fmt.Printf("\n")

		for _, l := range lines {
			fmt.Printf("  %s\n", l)
		}

		fmt.Printf("\n")
	}

	return err
}

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		parseStream(os.Stdin)
	}

	for _, f := range flag.Args() {
		fmt.Printf("parsing %s\n", f)

		func() {
			fd, err := os.Open(f)
			if err != nil {
				panic(err)
			}
			defer fd.Close()

			parseStream(fd)
		}()
	}
}
