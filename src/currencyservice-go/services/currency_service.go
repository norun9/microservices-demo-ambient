// currencyservice-go/services/currency_service.go

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/norun9/microservices-demo-ambient/src/currencyservice-go/genproto/hipstershop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CurrencyService は gRPC の CurrencyService を実装します
type CurrencyService struct {
	hipstershop.UnimplementedCurrencyServiceServer
	currencyData map[string]float64
	tracer       trace.Tracer
}

// NewCurrencyService コンストラクタ
func NewCurrencyService() (*CurrencyService, error) {
	currencyData, err := loadCurrencyData()
	if err != nil {
		return nil, fmt.Errorf("failed to load currency data: %w", err)
	}

	return &CurrencyService{
		currencyData: currencyData,
		tracer:       otel.Tracer("currencyservice"),
	}, nil
}

// GetSupportedCurrencies RPC: サポートされている通貨のリストを返します
func (c *CurrencyService) GetSupportedCurrencies(ctx context.Context, req *hipstershop.Empty) (*hipstershop.GetSupportedCurrenciesResponse, error) {
	// 呼び出し元のコンテキストをそのまま使用（親スパンは呼び出し元が生成）
	_, span := c.tracer.Start(ctx, "GetSupportedCurrencies")
	defer span.End()

	log.Println("Getting supported currencies...")

	currencyCodes := make([]string, 0, len(c.currencyData))
	for code := range c.currencyData {
		currencyCodes = append(currencyCodes, code)
	}

	span.SetAttributes(
		attribute.Int("supported.currencies.count", len(currencyCodes)),
	)

	return &hipstershop.GetSupportedCurrenciesResponse{
		CurrencyCodes: currencyCodes,
	}, nil
}

// Convert RPC: 通貨変換を実行します
func (c *CurrencyService) Convert(ctx context.Context, req *hipstershop.CurrencyConversionRequest) (*hipstershop.Money, error) {
	_, span := c.tracer.Start(ctx, "ConvertCurrency")
	defer span.End()

	log.Printf("Converting %v %s to %s", req.From.Units, req.From.CurrencyCode, req.ToCode)

	fromRate, ok1 := c.currencyData[req.From.CurrencyCode]
	toRate, ok2 := c.currencyData[req.ToCode]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("unsupported currency: %s or %s", req.From.CurrencyCode, req.ToCode)
	}
	span.SetAttributes(
		attribute.Float64("rate.from", fromRate),
		attribute.Float64("rate.to", toRate),
	)

	// from → EUR → to
	totalFrom := float64(req.From.Units) + float64(req.From.Nanos)/1e9
	euros := totalFrom / fromRate
	resultAmt := euros * toRate
	resultUnits := int64(math.Floor(resultAmt))
	resultNanos := int32(math.Round((resultAmt - float64(resultUnits)) * 1e9))
	// nanos 範囲チェック
	if resultNanos >= 1e9 {
		resultUnits += int64(resultNanos / 1e9)
		resultNanos %= 1e9
	}

	log.Println("Conversion request successful")

	return &hipstershop.Money{
		CurrencyCode: req.ToCode,
		Units:        resultUnits,
		Nanos:        resultNanos,
	}, nil
}

// loadCurrencyData は通貨変換データをJSONファイルから読み込みます
func loadCurrencyData() (map[string]float64, error) {
	// 初期化時の処理なので、独立したスパンとして生成
	// tracer := otel.Tracer("currencyservice")
	// _, span := tracer.Start(context.Background(), "LoadCurrencyData")
	// defer span.End()

	dataPath := filepath.Join("data", "currency_conversion.json")

	data, err := os.ReadFile(dataPath)
	if err != nil {
		// span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read currency data file: %w", err)
	}

	var rawData map[string]string
	if err := json.Unmarshal(data, &rawData); err != nil {
		// span.SetAttributes(attribute.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse currency data JSON: %w", err)
	}

	currencyData := make(map[string]float64)
	for code, rateStr := range rawData {
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			// span.SetAttributes(
			// 	attribute.String("error.currency", code),
			// 	attribute.String("error.rate", rateStr),
			// )
			return nil, fmt.Errorf("failed to parse rate for currency %s: %w", code, err)
		}
		currencyData[code] = rate
	}

	// span.SetAttributes(attribute.Int("total.currencies", len(currencyData)))

	return currencyData, nil
}
