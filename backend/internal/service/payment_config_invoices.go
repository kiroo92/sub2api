package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const SettingPaymentInvoiceConfig = "payment_invoice_config"

type InvoiceFeeTier struct {
	UpperAmount *float64 `json:"upper_amount"`
	Type        string   `json:"type"`
	Value       float64  `json:"value"`
}
type InvoiceConfig struct {
	Enabled  bool             `json:"enabled"`
	ItemName string           `json:"item_name"`
	TaxRate  float64          `json:"tax_rate"`
	Tiers    []InvoiceFeeTier `json:"tiers"`
}

func (s *PaymentConfigService) GetInvoiceConfig(ctx context.Context) (*InvoiceConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingPaymentInvoiceConfig)
	if errors.Is(err, ErrSettingNotFound) || err == nil && strings.TrimSpace(raw) == "" {
		return &InvoiceConfig{Tiers: []InvoiceFeeTier{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg InvoiceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *PaymentConfigService) SaveInvoiceConfig(ctx context.Context, cfg InvoiceConfig) (*InvoiceConfig, error) {
	cfg.ItemName = strings.TrimSpace(cfg.ItemName)
	if err := validateInvoiceConfig(cfg); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingPaymentInvoiceConfig: string(raw)}); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func invoiceMoneyValid(value float64, zero bool) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value >= 1e12 || (!zero && value == 0) {
		return false
	}
	d := decimal.NewFromFloat(value)
	return d.Equal(d.Round(2))
}
func validateInvoiceConfig(cfg InvoiceConfig) error {
	invalid := infraerrors.BadRequest("INVOICE_CONFIG_INVALID", "complete the invoice item, tax rate and ordered fee tiers")
	if utf8.RuneCountInString(cfg.ItemName) > 200 || !invoiceMoneyValid(cfg.TaxRate, true) || cfg.TaxRate > 100 {
		return invalid
	}
	if len(cfg.Tiers) == 0 {
		if cfg.Enabled {
			return invalid
		}
		return nil
	}
	if strings.TrimSpace(cfg.ItemName) == "" || len(cfg.Tiers) > 100 {
		return invalid
	}
	previous := 0.0
	for i, tier := range cfg.Tiers {
		if !invoiceMoneyValid(tier.Value, false) || (tier.Type != "fixed" && tier.Type != "percentage") || (tier.Type == "percentage" && tier.Value > 100) {
			return invalid
		}
		if tier.UpperAmount == nil {
			if i != len(cfg.Tiers)-1 {
				return invalid
			}
		} else {
			if !invoiceMoneyValid(*tier.UpperAmount, false) || *tier.UpperAmount <= previous || i == len(cfg.Tiers)-1 {
				return invalid
			}
			previous = *tier.UpperAmount
		}
	}
	return nil
}

func invoiceAmounts(base decimal.Decimal, cfg InvoiceConfig) (tier InvoiceFeeTier, fee, total, net, tax decimal.Decimal, err error) {
	if err = validateInvoiceConfig(cfg); err != nil {
		return
	}
	if !cfg.Enabled {
		err = infraerrors.Forbidden("INVOICE_DISABLED", "invoice applications are disabled")
		return
	}
	if !base.IsPositive() || base.GreaterThanOrEqual(decimal.NewFromInt(1e12)) {
		err = infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "invalid invoice amount")
		return
	}
	for _, candidate := range cfg.Tiers {
		if candidate.UpperAmount == nil || base.LessThanOrEqual(decimal.NewFromFloat(*candidate.UpperAmount)) {
			tier = candidate
			break
		}
	}
	fee = decimal.NewFromFloat(tier.Value)
	if tier.Type == "percentage" {
		fee = base.Mul(fee).Div(decimal.NewFromInt(100)).Round(2)
	}
	total = base.Add(fee)
	if !fee.IsPositive() || total.GreaterThanOrEqual(decimal.NewFromInt(1e12)) {
		err = infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "invoice fee must be at least CNY 0.01 and total within supported range")
		return
	}
	net = total.Div(decimal.NewFromInt(1).Add(decimal.NewFromFloat(cfg.TaxRate).Div(decimal.NewFromInt(100)))).Round(2)
	tax = total.Sub(net)
	return
}

func invoiceHash(value any) string {
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
