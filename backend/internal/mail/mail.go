// Package mail — отправка писем через SMTP (наш docker-mailserver или любой релей). Без настроек
// (MAIL_HOST пуст) письма не уходят, а пишутся в лог: так работает локальный стенд.
package mail

import (
	"crypto/tls"
	"fmt"
	"mime"
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
func (m *Mailer) Send(to, subject, text string) error {
	if !m.Enabled() {
		m.log.Info("mail (not configured, logged only)", zap.String("to", to), zap.String("subject", subject), zap.String("text", text))
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
	msg := strings.Join([]string{
		"From: " + encodeName(from),
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		text,
	}, "\r\n")

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
	if _, err := w.Write([]byte(msg)); err != nil {
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
