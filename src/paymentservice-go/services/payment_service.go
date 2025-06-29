package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/norun9/microservices-demo-ambient/src/paymentservice-go/genproto/hipstershop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PaymentService は gRPC の PaymentService を実装します
type PaymentService struct {
	hipstershop.UnimplementedPaymentServiceServer
	tracer trace.Tracer
}

// NewPaymentService コンストラクタ
func NewPaymentService() (*PaymentService, error) {
	return &PaymentService{
		tracer: otel.Tracer("paymentservice"),
	}, nil
}

// Charge RPC: クレジットカードの課金処理を実行します
func (p *PaymentService) Charge(ctx context.Context, req *hipstershop.ChargeRequest) (*hipstershop.ChargeResponse, error) {
	_, span := p.tracer.Start(ctx, "Charge")
	defer span.End()

	log.Printf("PaymentService#Charge invoked with request: amount=%v, credit_card_number=%s",
		req.Amount, maskCreditCard(req.CreditCard.CreditCardNumber))

	span.SetAttributes(
		attribute.String("payment.currency", req.Amount.CurrencyCode),
		attribute.Int64("payment.units", req.Amount.Units),
		attribute.Int64("payment.nanos", int64(req.Amount.Nanos)),
		attribute.String("credit_card.type", getCardType(req.CreditCard.CreditCardNumber)),
	)

	// クレジットカードの検証
	if err := p.validateCreditCard(req.CreditCard); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// トランザクションIDを生成
	transactionID := uuid.New().String()

	log.Printf("Transaction processed: %s ending %s Amount: %s%d.%02d",
		getCardType(req.CreditCard.CreditCardNumber),
		req.CreditCard.CreditCardNumber[len(req.CreditCard.CreditCardNumber)-4:],
		req.Amount.CurrencyCode,
		req.Amount.Units,
		req.Amount.Nanos/10000000)

	span.SetAttributes(attribute.String("transaction.id", transactionID))

	return &hipstershop.ChargeResponse{
		TransactionId: transactionID,
	}, nil
}

// validateCreditCard はクレジットカードの検証を行います
func (p *PaymentService) validateCreditCard(creditCard *hipstershop.CreditCardInfo) error {
	// カード番号の形式チェック
	if !isValidCardNumber(creditCard.CreditCardNumber) {
		return fmt.Errorf("credit card info is invalid")
	}

	// カードタイプのチェック
	cardType := getCardType(creditCard.CreditCardNumber)
	if cardType != "visa" && cardType != "mastercard" {
		return fmt.Errorf("sorry, we cannot process %s credit cards. Only VISA or MasterCard is accepted", cardType)
	}

	// 有効期限のチェック
	if err := p.validateExpiration(creditCard.CreditCardExpirationMonth, creditCard.CreditCardExpirationYear); err != nil {
		return err
	}

	return nil
}

// validateExpiration は有効期限をチェックします
func (p *PaymentService) validateExpiration(month, year int32) error {
	now := time.Now()
	currentMonth := int32(now.Month())
	currentYear := int32(now.Year())

	if (currentYear*12 + currentMonth) > (year*12 + month) {
		return fmt.Errorf("your credit card expired on %d/%d", month, year)
	}

	return nil
}

// isValidCardNumber はカード番号の形式をチェックします
func isValidCardNumber(cardNumber string) bool {
	// 基本的な形式チェック（数字のみ、13-19桁）
	matched, _ := regexp.MatchString(`^\d{13,19}$`, cardNumber)
	return matched
}

// getCardType はカード番号からカードタイプを判定します
func getCardType(cardNumber string) string {
	// VISA: 4で始まる
	if matched, _ := regexp.MatchString(`^4`, cardNumber); matched {
		return "visa"
	}

	// MasterCard: 51-55, 2221-2720で始まる
	if matched, _ := regexp.MatchString(`^5[1-5]`, cardNumber); matched {
		return "mastercard"
	}
	if matched, _ := regexp.MatchString(`^2[2-7][2-9][0-9]`, cardNumber); matched {
		return "mastercard"
	}

	// その他のカードタイプ
	return "unknown"
}

// maskCreditCard はクレジットカード番号をマスクします
func maskCreditCard(cardNumber string) string {
	if len(cardNumber) < 4 {
		return "****"
	}
	return "****" + cardNumber[len(cardNumber)-4:]
}
