// Package mail — отправка писем через SMTP (наш docker-mailserver или любой релей). Без настроек
// (MAIL_HOST пуст) письма не уходят, а пишутся в лог: так работает локальный стенд.
package mail

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	Host string // smtp-сервер; пусто — режим лога
	Port string // 587 — STARTTLS, 465 — TLS сразу
	User string
	Pass string
	From string // «Рацион <info@racion.app>»
}

type Mailer struct {
	cfg Config
	log *zap.Logger
}

func New(cfg Config, log *zap.Logger) *Mailer { return &Mailer{cfg: cfg, log: log} }

// Enabled — настроен ли настоящий сервер
func (m *Mailer) Enabled() bool { return m.cfg.Host != "" }

// Send шлёт простое текстовое письмо. В режиме лога печатает тему и текст.
func (m *Mailer) Send(to, subject, text string) error { return m.send(to, subject, text, "") }

// SendLetter — письмо по шаблону (Render): текст и HTML в одном письме, клиент покажет то, что умеет.
func (m *Mailer) SendLetter(to string, l Letter) error {
	subject, text, html, err := Render(l)
	if err != nil {
		return err
	}
	if err := m.send(to, subject, text, html); err != nil {
		m.log.Warn("mail: letter not sent", zap.String("kind", l.Kind), zap.Error(err))
		return err
	}
	return nil
}

func (m *Mailer) send(to, subject, text, html string) error {
	if !m.Enabled() {
		m.log.Info("mail (not configured, logged only)", zap.String("to", to), zap.String("subject", subject), zap.String("text", text), zap.Int("html", len(html)))
		return nil
	}
	from := m.cfg.From
	if from == "" {
		from = m.cfg.User
	}
	fromAddr := from
	if i := strings.Index(from, "<"); i >= 0 {
		fromAddr = strings.Trim(from[i:], "<>")
	}
	msg := buildMessage(encodeName(from), to, subject, text, html, time.Now())

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	var c *smtp.Client
	var err error
	if m.cfg.Port == "465" {
		conn, derr := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
		if derr != nil {
			return fmt.Errorf("mail: dial: %w", derr)
		}
		c, err = smtp.NewClient(conn, m.cfg.Host)
	} else {
		c, err = smtp.Dial(addr)
		if err == nil {
			_ = c.Hello("racion.app") // вместо localhost по умолчанию: строгие серверы смотрят на HELO
			if ok, _ := c.Extension("STARTTLS"); ok {
				err = c.StartTLS(&tls.Config{ServerName: m.cfg.Host})
			}
		}
	}
	if err != nil {
		return fmt.Errorf("mail: connect: %w", err)
	}
	defer c.Close()
	if m.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(fromAddr); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return c.Quit()
}

// encodeName кодирует имя отправителя («Рацион <info@…>»), адрес оставляет как есть
func encodeName(from string) string {
	i := strings.Index(from, "<")
	if i <= 0 {
		return from
	}
	name := strings.TrimSpace(from[:i])
	return mime.QEncoding.Encode("utf-8", name) + " " + from[i:]
}

// buildMessage — заголовки и тело письма. С HTML — multipart/alternative: сначала текст, потом HTML
// (клиент показывает последнюю часть, которую умеет). Части в quoted-printable: строки HTML бывают
// длиннее 998 знаков, а SMTP такие не пропускает.
func buildMessage(from, to, subject, text, html string, now time.Time) []byte {
	var b bytes.Buffer
	head := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	head("From", from)
	head("To", to)
	head("Subject", mime.QEncoding.Encode("utf-8", subject))
	head("Date", now.Format(time.RFC1123Z))
	head("MIME-Version", "1.0")
	if html == "" {
		head("Content-Type", "text/plain; charset=utf-8")
		head("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		writeQP(&b, text)
		return b.Bytes()
	}
	boundary := "racion-" + randomHex(12)
	head("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", text}, {"text/html", html}} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + part.kind + "; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		writeQP(&b, part.body)
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes()
}

// writeQP — текст в quoted-printable с переводами строк CRLF, как требует почта.
func writeQP(b *bytes.Buffer, s string) {
	w := quotedprintable.NewWriter(b)
	_, _ = w.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")))
	_ = w.Close()
}

func randomHex(n int) string {
	p := make([]byte, n)
	_, _ = rand.Read(p)
	return hex.EncodeToString(p)
}
