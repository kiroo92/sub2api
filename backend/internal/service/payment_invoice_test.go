//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestInvoicePricing(t *testing.T) {
	a, b, c := 500.0, 1000.0, 2000.0
	cfg := InvoiceConfig{Enabled: true, ItemName: "技术服务费", TaxRate: 3, Tiers: []InvoiceFeeTier{{&a, "fixed", 40}, {&b, "fixed", 100}, {&c, "fixed", 120}, {nil, "percentage", 3}}}
	for _, test := range []struct{ base, fee string }{{"500", "40"}, {"500.01", "100"}, {"1000", "100"}, {"1000.01", "120"}, {"2000", "120"}, {"2000.01", "60"}, {"3000", "90"}} {
		t.Run(test.base, func(t *testing.T) {
			_, fee, total, net, tax, err := invoiceAmounts(decimal.RequireFromString(test.base), cfg)
			require.NoError(t, err)
			require.True(t, fee.Equal(decimal.RequireFromString(test.fee)))
			require.True(t, total.Equal(decimal.RequireFromString(test.base).Add(fee)))
			require.True(t, net.Add(tax).Equal(total))
		})
	}
	cfg.Tiers = []InvoiceFeeTier{{nil, "fixed", 38}}
	_, _, total, net, tax, err := invoiceAmounts(decimal.RequireFromString("311.50"), cfg)
	require.NoError(t, err)
	require.Equal(t, "349.50", total.StringFixed(2))
	require.Equal(t, "339.32", net.StringFixed(2))
	require.Equal(t, "10.18", tax.StringFixed(2))
	for _, tiers := range [][]InvoiceFeeTier{{}, {{&a, "fixed", 0}}, {{&a, "fixed", 40}}, {{nil, "fixed", 40}, {nil, "fixed", 10}}, {{&b, "fixed", 40}, {&a, "fixed", 10}, {nil, "fixed", 1}}, {{nil, "percentage", math.NaN()}}} {
		cfg.Tiers = tiers
		require.Error(t, validateInvoiceConfig(cfg))
	}
	cfg.Tiers = []InvoiceFeeTier{{nil, "percentage", 0.01}}
	_, _, _, _, _, err = invoiceAmounts(decimal.RequireFromString("0.01"), cfg)
	require.Error(t, err, "do not silently round an unpaid fee up")
}

func TestInvoiceInformationValidation(t *testing.T) {
	req := CreateInvoiceRequest{OrderIDs: []int64{2, 1}, TaxID: " 0000123456 ", Title: " Test ", Email: "billing@example.com", QuoteFingerprint: invoiceHash("quote")}
	require.NoError(t, normalizeInvoiceRequest(&req))
	require.Equal(t, []int64{1, 2}, req.OrderIDs)
	require.Equal(t, "0000123456", req.TaxID)
	require.Empty(t, req.Remarks)
	req.Email = "invalid"
	require.Error(t, normalizeInvoiceRequest(&req))
	req.Email = "billing@example.com"
	req.OrderIDs = []int64{1, 1}
	require.Error(t, normalizeInvoiceRequest(&req))
}
