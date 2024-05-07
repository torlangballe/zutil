package zfilelister

import "github.com/torlangballe/zutil/zgeo"

type DirOptions struct {
	ViewOnly          bool
	ChooseFolders     bool
	FoldersOnly       bool // show folders only, need this option if only showing folders, as no wildcard for that
	ExtensionsAllowed []string
	PickedPaths       []string // ends in / if folders
	StoreName         string
	PathStub          string
	IconSize          zgeo.Size
	MaxFiles          int // used when getting all file paths from picked files/folders
}

const (
	cachePrefix = "caches/filelister-icons"
)
