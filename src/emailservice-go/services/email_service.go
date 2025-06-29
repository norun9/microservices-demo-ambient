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

	"github.com/norun9/microservices-demo-ambient/src/emailservice-go/genproto/hipstershop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EmailService は gRPC の EmailService を実装します
type EmailService struct {
	hipstershop.UnimplementedEmailServiceServer
	template *template.Template
	tracer   trace.Tracer
}

// NewEmailService コンストラクタ
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

// SendOrderConfirmation RPC: 注文確認メールを送信します
func (e *EmailService) SendOrderConfirmation(ctx context.Context, req *hipstershop.SendOrderConfirmationRequest) (*hipstershop.Empty, error) {
	_, span := e.tracer.Start(ctx, "SendOrderConfirmation")
	defer span.End()

	log.Printf("Sending order confirmation email to %s", req.Email)

	span.SetAttributes(
		attribute.String("email.recipient", req.Email),
		attribute.String("order.id", req.Order.OrderId),
	)

	// メールテンプレートのレンダリング
	content, err := e.renderEmailTemplate(req.Order)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to render email template: %v", err)
		return &hipstershop.Empty{}, nil // エラーを無視して続行
	}

	// メール送信（ダミーモード）
	err = e.sendEmail(req.Email, content)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		log.Printf("Failed to send email: %v", err)
		return &hipstershop.Empty{}, nil // エラーを無視して続行
	}

	log.Printf("Order confirmation email sent successfully to %s", req.Email)
	return &hipstershop.Empty{}, nil
}

// renderEmailTemplate は注文情報を使ってメールテンプレートをレンダリングします
func (e *EmailService) renderEmailTemplate(order *hipstershop.OrderResult) (string, error) {
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

// sendEmail はメールを送信します（現在はダミー実装）
func (e *EmailService) sendEmail(emailAddress, content string) error {
	_, span := e.tracer.Start(context.Background(), "SendEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("email.recipient", emailAddress),
		attribute.Int("email.content.length", len(content)),
	)

	// ダミーモード: 実際のメール送信は行わず、ログに出力
	log.Printf("DUMMY EMAIL SENT to %s", emailAddress)
	log.Printf("Email content length: %d characters", len(content))

	return nil
}

// loadEmailTemplate はメールテンプレートを読み込みます
func loadEmailTemplate() (*template.Template, error) {
	// 初期化時の処理なので、独立したスパンとして生成
	tracer := otel.Tracer("emailservice")
	_, span := tracer.Start(context.Background(), "LoadEmailTemplate")
	defer span.End()

	templatePath := filepath.Join("templates", "confirmation.html")

	// テンプレートファイルの存在確認
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		span.SetAttributes(attribute.String("error", "template file not found"))
		return nil, fmt.Errorf("template file not found: %s", templatePath)
	}

	// カスタム関数を定義
	funcMap := template.FuncMap{
		"div": func(a, b int64) int64 {
			return a / b
		},
	}

	// テンプレートの読み込みと解析
	tmpl, err := template.New("confirmation.html").Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	span.SetAttributes(attribute.String("template.path", templatePath))
	return tmpl, nil
}
