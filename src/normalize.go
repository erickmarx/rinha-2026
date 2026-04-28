package src

import (
	"math"
	"time"
)

type Vector [14]float64

type Normalizer struct {
	tx *Transaction
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func clamp(v float64) float64 {
	return round4(min(max(v, 0.0), 1.0))
}

const (
	maxAmount            = 10000
	maxInstallments      = 12
	amountVSAvgRatio     = 10
	maxMinutes           = 1440
	maxKm                = 1000
	maxTxCount24h        = 20
	maxMerchantAvgAmount = 10000
)

func mccRiskValue(mcc string) (float64, int) {
	switch mcc {
	case "5411":
		return 0.15, 0
	case "5812":
		return 0.30, 0
	case "5912":
		return 0.20, 0
	case "5944":
		return 0.45, 0
	case "7801":
		return 0.80, 0
	case "7802":
		return 0.75, 0
	case "7995":
		return 0.85, 0
	case "4511":
		return 0.35, 0
	case "5311":
		return 0.25, 0
	case "5999":
		return 0.50, 0
	default:
		return 0.50, 1
	}
}

func (n *Normalizer) amount() float64 {
	return clamp(n.tx.Transaction.Amount / maxAmount)
}

func (n *Normalizer) installments() float64 {
	return clamp(float64(n.tx.Transaction.Installments) / maxInstallments)
}

func (n *Normalizer) amount_vs_avg() float64 {
	return clamp(float64((n.tx.Transaction.Amount / n.tx.Customer.AvgAmount) / amountVSAvgRatio))
}

func (n *Normalizer) hour_of_day() float64 {
	return round4(float64(n.tx.Transaction.RequestedAt.Hour()) / 23)
}

func (n *Normalizer) day_of_week() float64 {
	wd := n.tx.Transaction.RequestedAt.Weekday()

	if wd == time.Sunday {
		return 1
	}

	return round4(float64(wd-1) / 6.0)
}

func (n *Normalizer) minutes_since_last_tx() float64 {
	if n.tx.LastTransaction == nil || n.tx.LastTransaction.Timestamp == nil {
		return -1
	}

	diff := time.Since(*n.tx.LastTransaction.Timestamp)

	return clamp(diff.Minutes() / maxMinutes)
}

func (n *Normalizer) km_from_last_tx() float64 {
	if n.tx.LastTransaction == nil || n.tx.LastTransaction.KMFromCurrent == nil {
		return -1
	}

	return clamp(*n.tx.LastTransaction.KMFromCurrent / maxKm)
}

func (n *Normalizer) km_from_home() float64 {
	return clamp(n.tx.Terminal.KMFromHome / maxKm)
}

func (n *Normalizer) tx_count_24h() float64 {
	return clamp(float64(n.tx.Customer.TxCount24h) / maxTxCount24h)
}

func (n *Normalizer) is_online() float64 {
	if n.tx.Terminal.IsOnline {
		return 1
	}
	return 0
}

func (n *Normalizer) card_present() float64 {
	if n.tx.Terminal.CardPresent {
		return 1
	}
	return 0
}

func (n *Normalizer) unknown_merchant() int {
	_, known := mccRiskValue(n.tx.Merchant.MCC)
	return known
}

func (n *Normalizer) mcc_risk() float64 {
	mcc, _ := mccRiskValue(n.tx.Merchant.MCC)
	return mcc
}

func (n *Normalizer) merchant_avg_amount() float64 {
	return clamp(n.tx.Merchant.AvgAmount / maxMerchantAvgAmount)
}

func normalize(t *Transaction) Vector {
	n := Normalizer{tx: t}

	return Vector{
		n.amount(),
		n.installments(),
		n.amount_vs_avg(),
		n.hour_of_day(),
		n.day_of_week(),
		n.minutes_since_last_tx(),
		n.km_from_last_tx(),
		n.km_from_home(),
		n.tx_count_24h(),
		n.is_online(),
		n.card_present(),
		float64(n.unknown_merchant()),
		n.mcc_risk(),
		n.merchant_avg_amount(),
	}
}
