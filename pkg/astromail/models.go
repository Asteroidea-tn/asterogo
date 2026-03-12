package astromail

// EMAIL Module -------------------------------------------------------
type EmailConfig struct {
	EmailServer   string `json:"email_server"`   // SMTP server address
	EmailPort     *int   `json:"email_port"`     // SMTP server port
	EmailSSL      *bool  `json:"email_ssl"`      // Use SSL/TLS for SMTP connection
	EmailUsername string `json:"email_username"` // SMTP username
	EmailPassword string `json:"email_password"` // SMTP password
	EmailFrom     string `json:"from_email"`     // Sender's email address
	EmailCC       string `json:"cc_email"`       // CC email addresses (comma-separated)
	EmailObject   string `json:"email_object"`   // Email subject
	To            string `json:"to_email"`       // Recipient email addresses (comma-separated)
	Body          string `json:"email_body"`     // Email body content (HTML or plain text)
}
