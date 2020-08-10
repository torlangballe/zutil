package zmail

import (
	"github.com/sendgrid/sendgrid-go"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/torlangballe/zutil/zlog"
)

func (m Mail) SendGridSend(apiKey string) error {
	zlog.Assert(len(m.To) != 0)
	from := sgmail.NewEmail(m.From.Name, m.From.Email)
	for _, t := range m.To {
		to := sgmail.NewEmail(t.Name, t.Email)
		message := sgmail.NewSingleEmail(from, m.Subject, to, m.TextContent, m.HTMLContent)
		client := sendgrid.NewSendClient(apiKey)
		_, err := client.Send(message)
		if err != nil {
			return zlog.Error(err, "send")
		}
		zlog.Info("send:", to.Address)
	}
	return nil
}
