package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/google/uuid"
	pb "github.com/norun9/microservices-demo-ambient/genproto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PaymentService implements the gRPC PaymentService.
type PaymentService struct {
	pb.UnimplementedPaymentServiceServer
	tracer trace.Tracer
}

// NewPaymentService constructor.
func NewPaymentService() (*PaymentService, error) {
	return &PaymentService{
		tracer: otel.Tracer("paymentservice"),
	}, nil
}

// Charge RPC: processes a credit card charge.
func (p *PaymentService) Charge(ctx context.Context, req *pb.ChargeRequest) (*pb.ChargeResponse, error) {
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

	// Validate credit card.
	if err := p.validateCreditCard(req.CreditCard); err != nil {
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, err
	}

	// Generate transaction ID.
	transactionID := uuid.New().String()

	log.Printf("Transaction processed: %s ending %s Amount: %s%d.%02d",
		getCardType(req.CreditCard.CreditCardNumber),
		req.CreditCard.CreditCardNumber[len(req.CreditCard.CreditCardNumber)-4:],
		req.Amount.CurrencyCode,
		req.Amount.Units,
		req.Amount.Nanos/10000000)

	span.SetAttributes(attribute.String("transaction.id", transactionID))

	return &pb.ChargeResponse{
		TransactionId: transactionID,
	}, nil
}

// validateCreditCard validates the credit card.
func (p *PaymentService) validateCreditCard(creditCard *pb.CreditCardInfo) error {
	// Check card number format.
	if !isValidCardNumber(creditCard.CreditCardNumber) {
		return fmt.Errorf("credit card info is invalid")
	}

	// Check card type.
	cardType := getCardType(creditCard.CreditCardNumber)
	if cardType != "visa" && cardType != "mastercard" {
		return fmt.Errorf("sorry, we cannot process %s credit cards. Only VISA or MasterCard is accepted", cardType)
	}

	// Check expiration date.
	if err := p.validateExpiration(creditCard.CreditCardExpirationMonth, creditCard.CreditCardExpirationYear); err != nil {
		return err
	}

	return nil
}

// validateExpiration checks the expiration date.
func (p *PaymentService) validateExpiration(month, year int32) error {
	now := time.Now()
	currentMonth := int32(now.Month())
	currentYear := int32(now.Year())

	if (currentYear*12 + currentMonth) > (year*12 + month) {
		return fmt.Errorf("your credit card expired on %d/%d", month, year)
	}

	return nil
}

// isValidCardNumber checks the card number format.
func isValidCardNumber(cardNumber string) bool {
	// Basic format check (digits only, 13-19 digits).
	matched, _ := regexp.MatchString(`^\d{13,19}$`, cardNumber)
	return matched
}

// getCardType determines the card type from the card number.
func getCardType(cardNumber string) string {
	// VISA: starts with 4.
	if matched, _ := regexp.MatchString(`^4`, cardNumber); matched {
		return "visa"
	}

	// MasterCard: starts with 51-55, 2221-2720.
	if matched, _ := regexp.MatchString(`^5[1-5]`, cardNumber); matched {
		return "mastercard"
	}
	if matched, _ := regexp.MatchString(`^2[2-7][2-9][0-9]`, cardNumber); matched {
		return "mastercard"
	}

	// Other card types.
	return "unknown"
}

// maskCreditCard masks the credit card number.
func maskCreditCard(cardNumber string) string {
	if len(cardNumber) < 4 {
		return "****"
	}
	return "****" + cardNumber[len(cardNumber)-4:]
}
