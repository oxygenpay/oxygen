package main

import (
	"strconv"

	"github.com/oxygenpay/oxygen/cmd"
	"github.com/samber/lo"
)

// set by LDFLAGS at compile time
var (
	gitCommit     string
	gitVersion    string
	embedFrontend string
)

func main() {
	cmd.Version = gitVersion
	cmd.Commit = gitCommit
	cmd.EmbedFrontend = lo.Must(strconv.ParseBool(embedFrontend))

	cmd.Execute()
}
