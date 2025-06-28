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
)

// CurrencyService は gRPC の CurrencyService を実装します
type CurrencyService struct {
	hipstershop.UnimplementedCurrencyServiceServer
	currencyData map[string]float64
}

// NewCurrencyService コンストラクタ
func NewCurrencyService() (*CurrencyService, error) {
	currencyData, err := loadCurrencyData()
	if err != nil {
		return nil, fmt.Errorf("failed to load currency data: %w", err)
	}

	return &CurrencyService{
		currencyData: currencyData,
	}, nil
}

// GetSupportedCurrencies RPC: サポートされている通貨のリストを返します
func (c *CurrencyService) GetSupportedCurrencies(ctx context.Context, req *hipstershop.Empty) (*hipstershop.GetSupportedCurrenciesResponse, error) {
	log.Println("Getting supported currencies...")

	currencyCodes := make([]string, 0, len(c.currencyData))
	for code := range c.currencyData {
		currencyCodes = append(currencyCodes, code)
	}

	return &hipstershop.GetSupportedCurrenciesResponse{
		CurrencyCodes: currencyCodes,
	}, nil
}

// Convert RPC: 通貨変換を実行します
func (c *CurrencyService) Convert(ctx context.Context, req *hipstershop.CurrencyConversionRequest) (*hipstershop.Money, error) {
	log.Printf("Converting %v %s to %s", req.From.Units, req.From.CurrencyCode, req.ToCode)

	// 入力通貨のレートを取得
	fromRate, exists := c.currencyData[req.From.CurrencyCode]
	if !exists {
		return nil, fmt.Errorf("unsupported currency: %s", req.From.CurrencyCode)
	}

	// 出力通貨のレートを取得
	toRate, exists := c.currencyData[req.ToCode]
	if !exists {
		return nil, fmt.Errorf("unsupported currency: %s", req.ToCode)
	}

	// 変換: from_currency --> EUR --> to_currency
	// まず EUR に変換
	fromUnits := float64(req.From.Units)
	fromNanos := float64(req.From.Nanos) / 1e9 // nanos を units に変換

	totalFromAmount := fromUnits + fromNanos
	euros := totalFromAmount / fromRate

	// EUR から目標通貨に変換
	resultAmount := euros * toRate

	// 結果を units と nanos に分割
	resultUnits := int64(math.Floor(resultAmount))
	resultNanos := int32(math.Round((resultAmount - float64(resultUnits)) * 1e9))

	// nanos の範囲チェック
	if resultNanos >= 1e9 {
		resultUnits += int64(resultNanos / 1e9)
		resultNanos = resultNanos % 1e9
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
	dataPath := filepath.Join("data", "currency_conversion.json")
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read currency data file: %w", err)
	}

	var rawData map[string]string
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse currency data JSON: %w", err)
	}

	currencyData := make(map[string]float64)
	for code, rateStr := range rawData {
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rate for currency %s: %w", code, err)
		}
		currencyData[code] = rate
	}

	return currencyData, nil
}
