package main

import "io"

// version is stamped at build time with
//
//	-ldflags "-X main.version=v1.2.3"
//
// Untagged local builds stay "dev".
var version = "dev"

func writeVersion(w io.Writer) {
	_, _ = io.WriteString(w, version+"\n")
}
