package astromail

import (
	"crypto/tls"
	"fmt"
	"strings"

	"gopkg.in/gomail.v2"
)

// SendEmail sends an email using the provided configuration
func SendEmail(smtpData *EmailConfig) error {
	// Validate required fields
	if smtpData == nil {
		return fmt.Errorf("Email configuration is nil")
	}

	if smtpData.EmailServer == "" || smtpData.EmailPort == nil ||
		smtpData.EmailUsername == "" || smtpData.EmailPassword == "" {
		return fmt.Errorf("Incomplete email configuration")
	}

	// Set up message
	message := gomail.NewMessage()
	message.SetHeader("From", smtpData.EmailFrom)

	// Handle multiple recipients if needed
	recipients := strings.Split(smtpData.To, ",")
	message.SetHeader("To", recipients...)

	// Add CC if specified
	if smtpData.EmailCC != "" {
		ccRecipients := strings.Split(smtpData.EmailCC, ",")
		message.SetHeader("Cc", ccRecipients...)
	}

	message.SetHeader("Subject", smtpData.EmailObject)
	message.SetBody("text/html", smtpData.Body)

	// Configure dialer with SSL/TLS settings
	dialer := gomail.NewDialer(
		smtpData.EmailServer,
		*smtpData.EmailPort,
		smtpData.EmailUsername,
		smtpData.EmailPassword,
	)

	// Set SSL based on configuration (default to true if not specified)
	sslEnabled := true
	if smtpData.EmailSSL != nil {
		sslEnabled = *smtpData.EmailSSL
	}
	dialer.SSL = sslEnabled

	// Skip certificate validation ( Only for non-production use)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Send the email
	if err := dialer.DialAndSend(message); err != nil {
		return err
	}

	return nil
}
