// Package notify renders and sends the hub's emails in the recipient's language.
package notify

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/locale"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/product"
)

type Kind string

const (
	KindVerifyEmail   Kind = "verify"
	KindPasswordReset Kind = "reset"
	KindTest          Kind = "test"
)

// Data fills a message. Link is empty for messages without an action.
type Data struct {
	Instance string
	Name     string
	Link     string
}

//go:embed layout.txt.tmpl
var textLayout string

//go:embed layout.html.tmpl
var htmlLayout string

var (
	textTmpl = texttemplate.Must(texttemplate.New("text").Parse(textLayout))
	htmlTmpl = htmltemplate.Must(htmltemplate.New("html").Parse(htmlLayout))
)

type view struct {
	Lang     string
	Subject  string
	Greeting string
	Intro    string
	Action   string
	Link     string
	Outro    string
	Instance string
}

// Render builds the message for kind in loc, falling back to the default locale.
func Render(kind Kind, loc, to string, d Data) (ports.Mail, error) {
	msgs, ok := catalog[loc]
	if !ok {
		loc, msgs = locale.Default, catalog[locale.Default]
	}
	m, ok := msgs[kind]
	if !ok {
		return ports.Mail{}, fmt.Errorf("no %q email for locale %s", kind, loc)
	}
	fill := strings.NewReplacer("{instance}", d.Instance, "{name}", d.Name)
	v := view{
		Lang:     loc,
		Subject:  fill.Replace(m.Subject),
		Greeting: fill.Replace(m.Greeting),
		Intro:    fill.Replace(m.Intro),
		Action:   fill.Replace(m.Action),
		Link:     d.Link,
		Outro:    fill.Replace(m.Outro),
		Instance: d.Instance,
	}
	var text, html bytes.Buffer
	if err := textTmpl.Execute(&text, v); err != nil {
		return ports.Mail{}, fmt.Errorf("render %s text: %w", kind, err)
	}
	if err := htmlTmpl.Execute(&html, v); err != nil {
		return ports.Mail{}, fmt.Errorf("render %s html: %w", kind, err)
	}
	return ports.Mail{To: to, Subject: v.Subject, Text: text.String(), HTML: html.String()}, nil
}

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

// Notifier sends rendered emails through the configured mailer.
type Notifier struct {
	Mailer   ports.Mailer
	Settings SettingsReader
}

func (n *Notifier) VerifyEmail(ctx context.Context, to, name, loc, link string) error {
	return n.send(ctx, KindVerifyEmail, to, name, loc, link)
}

func (n *Notifier) PasswordReset(ctx context.Context, to, name, loc, link string) error {
	return n.send(ctx, KindPasswordReset, to, name, loc, link)
}

func (n *Notifier) Test(ctx context.Context, to, loc string) error {
	return n.send(ctx, KindTest, to, "", loc, "")
}

func (n *Notifier) send(ctx context.Context, kind Kind, to, name, loc, link string) error {
	m, err := Render(kind, loc, to, Data{Instance: InstanceName(ctx, n.Settings), Name: name, Link: link})
	if err != nil {
		return err
	}
	if err := n.Mailer.Send(ctx, m); err != nil {
		return fmt.Errorf("send %s email: %w", kind, err)
	}
	return nil
}

// InstanceName is the name emails sign with: the instance's, or the product's
// before setup names it.
func InstanceName(ctx context.Context, settings SettingsReader) string {
	if s, err := settings.Get(ctx); err == nil && s.Name != "" {
		return s.Name
	}
	return product.Name
}
