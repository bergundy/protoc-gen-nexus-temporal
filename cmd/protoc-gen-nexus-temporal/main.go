package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/bergundy/protoc-gen-nexus-temporal/internal/plugin"
	"google.golang.org/protobuf/compiler/protogen"
)

var (
	version = "dev"
	commit  = "latest"
)

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-nexus-temporal: %s (%s)\n", version, commit)
		fmt.Printf("go: %s\n", runtime.Version())
		return
	}

	p := plugin.New(version, commit)

	opts := protogen.Options{
		ParamFunc: p.Param,
	}

	opts.Run(p.Run)
}
