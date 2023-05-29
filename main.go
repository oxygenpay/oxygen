package main

import (
	"github.com/oxygenpay/oxygen/cmd"
)

// set by LDFLAGS at compile time
var gitCommit, gitVersion string

func main() {
	cmd.Version = gitVersion
	cmd.Commit = gitCommit
	cmd.Execute()
}
