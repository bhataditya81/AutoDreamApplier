// Package notification provides email delivery via AWS SES v2.
// All public methods are safe to call with a nil *Client — they become no-ops,
// which simplifies local development where SES credentials are absent.
package notification

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/rs/zerolog"
)

// Client wraps the AWS SES v2 API and provides typed email-sending methods.
type Client struct {
	ses       *sesv2.Client
	fromEmail string
	dashURL   string
	log       zerolog.Logger
}

// Config holds the values needed to build a Client.
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	FromEmail       string
	DashboardURL    string // base URL for CTA links, e.g. "https://app.autodreamapplier.com"
}

// New creates a Client connected to AWS SES v2.
// Returns nil (no-op client) when FromEmail is empty so local dev works
// without AWS credentials.
func New(ctx context.Context, cfg Config, log zerolog.Logger) (*Client, error) {
	if cfg.FromEmail == "" {
		log.Warn().Msg("notification: SES_FROM_EMAIL not set — email notifications disabled")
		return nil, nil //nolint:nilnil
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for SES: %w", err)
	}

	return &Client{
		ses:       sesv2.NewFromConfig(awsCfg),
		fromEmail: cfg.FromEmail,
		dashURL:   cfg.DashboardURL,
		log:       log,
	}, nil
}

// ─── Email payload types ──────────────────────────────────────────────────────

// ApplicationSubmittedData is the template context for the "submitted" email.
type ApplicationSubmittedData struct {
	UserName       string
	JobTitle       string
	Company        string
	ApplyURL       string
	ApplicationID  string
	DashboardURL   string
	Year           int
}

// ApplicationFailedData is the template context for the "failed" email.
type ApplicationFailedData struct {
	UserName      string
	JobTitle      string
	Company       string
	ErrorSummary  string
	DashboardURL  string
	Year          int
}

// OutcomeData is the template context for the "outcome" email (interview/offer/rejected).
type OutcomeData struct {
	UserName      string
	JobTitle      string
	Company       string
	Outcome       string // "Interview Request", "Job Offer", "Rejection"
	OutcomeEmoji  string // 🎉 / 🚀 / 😔
	DashboardURL  string
	Year          int
}

// WeeklyDigestData is the template context for the weekly summary email.
type WeeklyDigestData struct {
	UserName     string
	WeekOf       string
	Applied      int
	Interviews   int
	Offers       int
	Rejections   int
	Pending      int
	DashboardURL string
	Year         int
}

// ─── Public sending methods ───────────────────────────────────────────────────

// SendApplicationSubmitted emails the user when a job application is successfully submitted.
func (c *Client) SendApplicationSubmitted(ctx context.Context, toEmail string, data ApplicationSubmittedData) error {
	if c == nil {
		return nil
	}
	data.Year = time.Now().Year()
	data.DashboardURL = c.dashURL
	subject := fmt.Sprintf("✅ Applied to %s at %s", data.JobTitle, data.Company)
	return c.send(ctx, toEmail, subject, tmplSubmitted, data)
}

// SendApplicationFailed emails the user when an application attempt fails.
func (c *Client) SendApplicationFailed(ctx context.Context, toEmail string, data ApplicationFailedData) error {
	if c == nil {
		return nil
	}
	data.Year = time.Now().Year()
	data.DashboardURL = c.dashURL
	subject := fmt.Sprintf("⚠️ Application failed — %s at %s", data.JobTitle, data.Company)
	return c.send(ctx, toEmail, subject, tmplFailed, data)
}

// SendOutcomeUpdate emails the user when an application outcome changes (interview/offer/rejected).
func (c *Client) SendOutcomeUpdate(ctx context.Context, toEmail string, data OutcomeData) error {
	if c == nil {
		return nil
	}
	data.Year = time.Now().Year()
	data.DashboardURL = c.dashURL
	subject := fmt.Sprintf("%s %s — %s at %s", data.OutcomeEmoji, data.Outcome, data.JobTitle, data.Company)
	return c.send(ctx, toEmail, subject, tmplOutcome, data)
}

// SendWeeklyDigest emails the user a weekly summary of their application activity.
func (c *Client) SendWeeklyDigest(ctx context.Context, toEmail string, data WeeklyDigestData) error {
	if c == nil {
		return nil
	}
	data.Year = time.Now().Year()
	data.DashboardURL = c.dashURL
	subject := fmt.Sprintf("📊 Your AutoDreamApplier Weekly Digest — %s", data.WeekOf)
	return c.send(ctx, toEmail, subject, tmplWeeklyDigest, data)
}

// SendContactNotification forwards a contact form submission to the configured
// from-email address so the team receives an alert. Nil-safe — no-ops in dev.
func (c *Client) SendContactNotification(ctx context.Context, name, fromEmail, subject, message string) error {
	if c == nil {
		return nil
	}
	if subject == "" {
		subject = "New contact form submission"
	}
	body := fmt.Sprintf(
		"<p><b>From:</b> %s &lt;%s&gt;</p><p><b>Subject:</b> %s</p><p>%s</p>",
		name, fromEmail, subject, message,
	)
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(c.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{c.fromEmail}, // reply to ourselves
		},
		ReplyToAddresses: []string{fromEmail},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String("[AutoDreamApplier Contact] " + subject), Charset: aws.String("UTF-8")},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(body), Charset: aws.String("UTF-8")},
				},
			},
		},
	}
	_, err := c.ses.SendEmail(ctx, input)
	return err
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (c *Client) send(ctx context.Context, toEmail, subject string, tmpl *template.Template, data any) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render email template %q: %w", tmpl.Name(), err)
	}
	html := buf.String()

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(c.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(html),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	_, err := c.ses.SendEmail(ctx, input)
	if err != nil {
		c.log.Error().Err(err).
			Str("to", toEmail).
			Str("subject", subject).
			Msg("SES SendEmail failed")
		return fmt.Errorf("ses send email: %w", err)
	}

	c.log.Info().
		Str("to", toEmail).
		Str("subject", subject).
		Msg("email sent via SES")

	return nil
}
