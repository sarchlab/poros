// Command disasm is a proof-of-concept ARM64 Mach-O disassembler.
// Usage: disasm <binary-path> [instruction-count]
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sarchlab/m2sim3/pkg/loader"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: disasm <binary> [count]\n")
		os.Exit(1)
	}

	path := os.Args[1]
	count := 50
	if len(os.Args) >= 3 {
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid count: %s\n", os.Args[2])
			os.Exit(1)
		}
		count = n
	}

	text, err := loader.LoadTextSection(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %s: __text section at 0x%x (%d bytes)\n\n",
		path, text.BaseAddr, text.Size)

	instructions := loader.DisassembleN(text.Bytes, text.BaseAddr, count)
	for _, inst := range instructions {
		fmt.Println(loader.FormatInstruction(inst))
	}
}
