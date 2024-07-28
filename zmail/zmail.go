package zmail

import (
	"errors"
	"fmt"
	"net/smtp"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
)

type ServiceType string

const (
	PlunkType    ServiceType = "plunk"
	SMTPType     ServiceType = "smtp"
	SendGridType ServiceType = "sendgrid"
)

type Authentication struct {
	ServiceType ServiceType
	UserID      string
	Password    string
	Server      string
	Port        int
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

func (m *Mail) AddTo(name, email string) {
	m.To = append(m.To, Address{Name: name, Email: email})
}

// https://zetcode.com/golang/email-smtp/
// Test with: https://www.smtper.net

func (m Mail) SendWithSMTP(a Authentication) (err error) {
	zlog.Assert(len(m.To) != 0 && m.To[0].Email != "")
	auth := smtp.PlainAuth("", a.UserID, a.Password, a.Server)
	server := fmt.Sprintf("%s:%d", a.Server, a.Port)
	zlog.Info("zmail.SendSMTP:", auth, server)

	if m.From.Email == "" {
		m.From.Email = a.UserID
	}
	zlog.Info("zmail.SendWithSMTP from:", zlog.Full(m.From))
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
	header := "From: " + m.From.Email + " <" + m.From.Name + ">\r\n"
	header += "Subject: " + m.Subject + "\r\n\r\n"
	if bulk {
		content := []byte(header + m.TextContent)
		err = smtp.SendMail(server, auth, m.From.Email, emails, content)
		if err != nil {
			return zlog.Error("send", m, a)
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
		// zlog.Info("SEND:::", m.From.Email, t.Email)
		berr := smtp.SendMail(server, auth, m.From.Email, []string{t.Email}, content)
		if berr != nil {
			err = berr
			zlog.Error("send single", a, err)
		}
	}
	return err
}

func (m Mail) Send(a Authentication) error {
	switch a.ServiceType {
	case PlunkType:
		return m.SendWithPlunk(a)
	case SMTPType:
		return m.SendWithSMTP(a)
	}
	return errors.New("Bad type: " + string(a.ServiceType))
}

func (m Mail) SendWithPlunk(a Authentication) error {
	var body = struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}{}
	params := zhttp.MakeParameters()
	params.Headers["Authorization"] = "Bearer " + a.Password
	surl := "https://api.useplunk.com/v1/send"

	body.To = m.To[0].Email
	body.Subject = m.Subject
	body.Body = m.TextContent
	_, err := zhttp.Post(surl, params, body, nil)
	return err
}

