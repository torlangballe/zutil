package zfilelister

type Options struct {
	ViewOnly          bool
	ChooseFolders     bool
	FoldersOnly       bool // show folders only, need this option if only showing folders, as no wildcard for that
	ExtensionsAllowed []string
	PickedPaths       []string // ends in / if folders
	StoreName         string
	PathStub          string
}
