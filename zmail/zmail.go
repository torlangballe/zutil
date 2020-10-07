package zmail

import (
	"fmt"
	"net/smtp"

	"github.com/torlangballe/zutil/zlog"
)

type Authentication struct {
	UserID   string
	Password string
	Server   string
	Port     int
}

type Address struct {
	Name  string
	Email string
}

type Mail struct {
	To          []Address
	From        Address
	Subject     string
	TextContent string
	HTMLContent string
}

// Test with: https://www.smtper.net

func (m Mail) SendWithSMTP(a Authentication) (err error) {
	zlog.Assert(len(m.To) != 0 && m.To[0].Email != "")
	auth := smtp.PlainAuth("", a.UserID, a.Password, a.Server)
	server := fmt.Sprintf("%s:%d", a.Server, a.Port)
	zlog.Info("zmail.SendSMTP:", a, "to:", m.To)

	var emails []string
	bulk := true
	for _, t := range m.To {
		if t.Name != "" {
			bulk = false
			return
		}
		emails = append(emails, t.Email)
	}
	if len(emails) < 2 {
		bulk = false
	}
	header := "Subject: " + m.Subject + "\r\n\r\n"
	if bulk {
		content := []byte(header + m.TextContent)
		err = smtp.SendMail(server, auth, m.From.Email, emails, content)
		if err != nil {
			return zlog.Error(err, "send", m, a)
		}
		return nil
	}
	for _, t := range m.To {
		var toheader string
		toheader = "To: " + t.Email
		if t.Name != "" {
			toheader += " <" + t.Name + ">"
		}
		toheader += "\r\n"
		content := []byte(toheader + header + m.TextContent)
		berr := smtp.SendMail(server, auth, m.From.Email, []string{t.Email}, content)
		if berr != nil {
			err = zlog.Error(berr, "send single", a)
		}
	}
	return err
}
