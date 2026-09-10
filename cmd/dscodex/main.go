// Command dscodex is the Go port of DSCodex: a local loopback router that
// exposes DeepSeek V4 Flash and Pro in the Codex / ChatGPT desktop model
// picker while GPT models keep working through ChatGPT OAuth.
//
// The port targets full behavioral parity with the upstream Node.js
// implementation: https://github.com/fish2lab/DSCodex
//
// This entry point is a P0 scaffold placeholder; commands are implemented in
// phases P4 and beyond (see docs/04-implementation-plan.md).
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println(version)
		return
	}
	fmt.Fprintln(os.Stderr, "dscodex: not implemented yet (P0 scaffold)")
	os.Exit(1)
}
