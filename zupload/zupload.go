package zupload

const (
	Drop                    = "drop"
	URL                     = "url"
	SCP                     = "scp" // https://pkg.go.dev/github.com/lkbhargav/go-scp#section-readme
	Select                  = "select"
	ShowMessageKey          = "zshow-message"
	UploadedTempFilePathKey = "zup-temp-filepath"
	UploadIDKey             = "zup-id"
)

type UploadPayload struct {
	HandleID string
	Type     string
	Name     string
	Text     string
	Password string
	// Data     []byte
}

var UploadTimeoutMinutes = 10
