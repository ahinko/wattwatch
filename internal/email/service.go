package email

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"sync"
	"wattwatch/internal/config"
)

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendVerificationEmail(to, username, token string) error
	SendPasswordResetEmail(to, username, token string) error
}

// Service implements the EmailSender interface
type Service struct {
	config config.EmailConfig
	client *smtp.Client
	mu     sync.Mutex
}

func NewService(cfg config.EmailConfig) *Service {
	return &Service{
		config: cfg,
		client: nil,
	}
}

// dialSMTP establishes an SMTP connection
func (s *Service) dialSMTP() (*smtp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse existing connection if it's still alive
	if s.client != nil {
		if err := s.client.Noop(); err == nil {
			return s.client, nil
		}
		// Connection is dead, close it
		s.client.Close()
		s.client = nil
	}

	// Create new connection
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SMTP server: %w", err)
	}

	if err := client.Auth(smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to authenticate with SMTP server: %w", err)
	}

	s.client = client
	return client, nil
}

// sendMail sends an email using a pooled SMTP connection
func (s *Service) sendMail(to []string, msg []byte) error {
	client, err := s.dialSMTP()
	if err != nil {
		return err
	}

	if err := client.Mail(s.config.SMTPUsername); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to create message writer: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close message writer: %w", err)
	}

	return nil
}

// Close closes the SMTP connection
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		err := s.client.Quit()
		s.client = nil
		return err
	}
	return nil
}

func (s *Service) SendVerificationEmail(to, username, token string) error {
	// Validate configuration
	if s.config.SMTPHost == "" || s.config.SMTPPort == 0 || s.config.SMTPUsername == "" ||
		s.config.SMTPPassword == "" || s.config.FromAddress == "" || s.config.AppURL == "" {
		return fmt.Errorf("incomplete email configuration")
	}

	subject := "Verify Your Email Address"
	verificationURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", s.config.AppURL, token)

	tmpl, err := template.New("verification").Parse(`
		<h2>Hello {{.Username}},</h2>
		<p>Please verify your email address by clicking the link below:</p>
		<p><a href="{{.URL}}">Verify Email Address</a></p>
		<p>This link will expire in 24 hours.</p>
		<p>If you did not create an account, no further action is required.</p>
	`)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, map[string]string{
		"Username": username,
		"URL":      verificationURL,
	}); err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	msg := fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", to, s.config.FromAddress, subject, body.String())

	log.Printf("Sending verification email to %s via SMTP server %s:%d", to, s.config.SMTPHost, s.config.SMTPPort)
	if err := s.sendMail([]string{to}, []byte(msg)); err != nil {
		log.Printf("SMTP error details: %+v", err)
		return fmt.Errorf("failed to send verification email: %w", err)
	}
	return nil
}

func (s *Service) SendPasswordResetEmail(to, username, token string) error {
	// Validate configuration
	if s.config.SMTPHost == "" || s.config.SMTPPort == 0 || s.config.SMTPUsername == "" ||
		s.config.SMTPPassword == "" || s.config.FromAddress == "" || s.config.AppURL == "" {
		return fmt.Errorf("incomplete email configuration")
	}

	subject := "Reset Your Password"
	resetURL := fmt.Sprintf("%s/api/v1/auth/reset-password?token=%s", s.config.AppURL, token)

	tmpl, err := template.New("reset").Parse(`
		<h2>Hello {{.Username}},</h2>
		<p>You have requested to reset your password. Click the link below to proceed:</p>
		<p><a href="{{.URL}}">Reset Password</a></p>
		<p>This link will expire in 1 hour.</p>
		<p>If you did not request a password reset, please ignore this email.</p>
	`)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, map[string]string{
		"Username": username,
		"URL":      resetURL,
	}); err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}

	msg := fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", to, s.config.FromAddress, subject, body.String())

	if err := s.sendMail([]string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}
