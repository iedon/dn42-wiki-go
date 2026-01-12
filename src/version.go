package main

import "fmt"

const (
	SERVER_NAME    = "iEdon-DN42-Wiki-Go"
	SERVER_VERSION = "1.1.5"
)

// Set at link stage via `-ldflags "-X main.GIT_COMMIT=$(git rev-parse --short HEAD)"`
var GIT_COMMIT string

// Server header string
var SERVER_SIGNATURE = fmt.Sprintf("%s (%s)", SERVER_NAME+"/"+SERVER_VERSION, func() string {
	if GIT_COMMIT != "" {
		return GIT_COMMIT
	}
	return "unknown"
}())
