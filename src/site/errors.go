package site

import "errors"

var (
	// ErrRepositoryBehind signals that the local clone is stale vs the remote.
	ErrRepositoryBehind  = errors.New("repository has newer remote revisions")
	ErrProtectedDocument = errors.New("document is protected")
)
