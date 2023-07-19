package zupload

const (
	Drop   = "drop"
	URL    = "url"
	SCP    = "scp" // https://pkg.go.dev/github.com/lkbhargav/go-scp#section-readme
	Upload = "upload"
	ShowMessageKey = "show-message"
)

type UploadPayload struct {
	HandleID string
	Type     string
	Name     string
	Text     string
	Password string
	// Data     []byte
}
