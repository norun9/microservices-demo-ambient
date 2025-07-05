// emailservice-go/services/email_service.go

package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EmailService implements the gRPC EmailService.
type EmailService struct {
	pb.UnimplementedEmailServiceServer
	template *template.Template
	tracer   trace.Tracer
}

// NewEmailService constructor.
func NewEmailService() (*EmailService, error) {
	tmpl, err := loadEmailTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to load email template: %w", err)
	}

	return &EmailService{
		template: tmpl,
		tracer:   otel.Tracer("emailservice"),
	}, nil
}

// SendOrderConfirmation RPC: sends an order confirmation email.
func (e *EmailService) SendOrderConfirmation(ctx context.Context, req *pb.SendOrderConfirmationRequest) (*pb.Empty, error) {
	_, span := e.tracer.Start(ctx, "SendOrderConfirmation")
	defer span.End()

	log.Printf("Sending order confirmation email to %s", req.Email)

	span.SetAttributes(
		attribute.String("email.recipient", req.Email),
		attribute.String("order.id", req.Order.OrderId),
	)

	// Render email template.
	content, err := e.renderEmailTemplate(req.Order)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to render email template: %v", err)
		return &pb.Empty{}, nil // Ignore the error and continue.
	}

	// Send email (dummy mode).
	err = e.sendEmail(req.Email, content)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to send email: %v", err)
		return &pb.Empty{}, nil // Ignore the error and continue.
	}

	log.Printf("Order confirmation email sent successfully to %s", req.Email)
	return &pb.Empty{}, nil
}

// renderEmailTemplate renders the email template with order information.
func (e *EmailService) renderEmailTemplate(order *pb.OrderResult) (string, error) {
	_, span := e.tracer.Start(context.Background(), "RenderEmailTemplate")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", order.OrderId),
		attribute.Int("order.items.count", len(order.Items)),
	)

	var buf bytes.Buffer
	err := e.template.ExecuteTemplate(&buf, "confirmation.html", order)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// sendEmail sends an email (currently a dummy implementation).
func (e *EmailService) sendEmail(emailAddress, content string) error {
	_, span := e.tracer.Start(context.Background(), "SendEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("email.recipient", emailAddress),
		attribute.Int("email.content.length", len(content)),
	)

	// Dummy mode: Do not send actual emails, just log them.
	log.Printf("DUMMY EMAIL SENT to %s", emailAddress)
	log.Printf("Email content length: %d characters", len(content))

	return nil
}

// loadEmailTemplate loads the email template.
func loadEmailTemplate() (*template.Template, error) {
	// This is an initialization process, so create it as an independent span.
	tracer := otel.Tracer("emailservice")
	_, span := tracer.Start(context.Background(), "LoadEmailTemplate")
	defer span.End()

	templatePath := filepath.Join("templates", "confirmation.html")

	// Check if the template file exists.
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		span.SetAttributes(attribute.String("error", "template file not found"))
		return nil, fmt.Errorf("template file not found: %s", templatePath)
	}

	// Define custom functions.
	funcMap := template.FuncMap{
		"div": func(a, b int64) int64 {
			return a / b
		},
	}

	// Load and parse the template.
	tmpl, err := template.New("confirmation.html").Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	span.SetAttributes(attribute.String("template.path", templatePath))
	return tmpl, nil
}
