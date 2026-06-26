package zfilelister

import "github.com/torlangballe/zutil/zgeo"

type DirOptions struct {
	ViewOnly          bool
	ChooseFolders     bool
	FoldersOnly       bool     // show folders only, need this option if only showing folders, as no wildcard for that
	AllowedExtensions []string // if empty, all extensions are allowed
	PickedPaths       []string // ends in / if folders
	StoreName         string
	PathStub          string
	IconSize          zgeo.Size
	MaxFiles          int // used when getting all file paths from picked files/folders
}

const (
	cachePrefix = "caches/filelister-icons"
)

var ExtensionToSymbol = map[string]string{
	".txt":  "📝",
	".md":   "📄",
	".rtf":  "📄",
	".doc":  "📄",
	".docx": "📄",
	".pdf":  "📕",
	".epub": "📚",

	".xls":  "📊",
	".xlsx": "📊",
	".csv":  "📊",

	".ppt":  "📈",
	".pptx": "📈",

	".json": "🧩",
	".xml":  "🏷",
	".yaml": "⚙",
	".yml":  "⚙",
	".ini":  "⚙",
	".cfg":  "⚙",

	".log": "📜",

	".sql":    "🗄",
	".db":     "🗄",
	".sqlite": "🗄",

	".zip": "📦",
	".7z":  "📦",
	".rar": "📦",
	".tar": "📦",
	".gz":  "📦",

	".iso": "💿",
	".img": "💿",

	".exe": "⚙",
	".msi": "⚙",
	".app": "⚙",

	".sh":  "🖥",
	".bat": "🖥",
	".ps1": "🖥",

	".py":   "🐍",
	".js":   "🟨",
	".ts":   "🔷",
	".java": "☕",
	".c":    "⚡",
	".cpp":  "⚡",
	".cc":   "⚡",
	".h":    "⚡",
	".hpp":  "⚡",

	".rs":  "🦀",
	".go":  "🐹",
	".php": "🐘",
	".rb":  "💎",

	".html": "🌐",
	".htm":  "🌐",
	".css":  "🎨",
	".svg":  "🎨",

	".png":  "🖼",
	".jpg":  "🖼",
	".jpeg": "🖼",
	".gif":  "🎞",
	".webp": "🖼",
	".ico":  "🔲",

	".mp3":  "🎵",
	".wav":  "🎵",
	".flac": "🎵",
	".ogg":  "🎵",

	".mp4": "🎬",
	".mkv": "🎬",
	".avi": "🎬",
	".mov": "🎬",

	".srt": "💬",

	".torrent": "🧲",

	".pem": "🔐",
	".crt": "🔐",
	".cer": "🔐",
	".key": "🔑",

	".asc": "✍",
}
