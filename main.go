// Command maat is the Ma'at CLI: documentation-as-code for humans and AI
// agents. It scaffolds (init), regenerates derived artifacts (sync), and
// validates the docs set as a CI gate (check).
package main

import (
	"os"

	"github.com/getmaat/maat/internal/maat"
)

func main() {
	os.Exit(maat.Main(os.Args[1:], os.Stdout, os.Stderr))
}
