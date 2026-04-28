package src

import (
	"errors"
	"time"
)

//	{
//	  "id": "tx-3576980410",
//	  "transaction": {
//	    "amount": 384.88,
//	    "installments": 3,
//	    "requested_at": "2026-03-11T20:23:35Z"
//	  },
//	  "customer": {
//	    "avg_amount": 769.76,
//	    "tx_count_24h": 3,
//	    "known_merchants": ["MERC-009", "MERC-001", "MERC-001"]
//	  },
//	  "merchant": {
//	    "id": "MERC-001",
//	    "mcc": "5912",
//	    "avg_amount": 298.95
//	  },
//	  "terminal": {
//	    "is_online": false,
//	    "card_present": true,
//	    "km_from_home": 13.7090520965
//	  },
//	  "last_transaction": {
//	    "timestamp": "2026-03-11T14:58:35Z",
//	    "km_from_current": 18.8626479774
//	  }
//	}
type Transaction struct {
	ID              string               `json:"id"`
	Transaction     TransactionData      `json:"transaction"`
	Customer        CustomerData         `json:"customer"`
	Merchant        MerchantData         `json:"merchant"`
	Terminal        TerminalData         `json:"terminal"`
	LastTransaction *LastTransactionData `json:"last_transaction"`
}

type TransactionData struct {
	Amount       float64   `json:"amount"`
	Installments int       `json:"installments"`
	RequestedAt  time.Time `json:"requested_at"`
}

type CustomerData struct {
	AvgAmount      float64  `json:"avg_amount"`
	TxCount24h     int      `json:"tx_count_24h"`
	KnownMerchants []string `json:"known_merchants"`
}

type MerchantData struct {
	ID        string  `json:"id"`
	MCC       string  `json:"mcc"`
	AvgAmount float64 `json:"avg_amount"`
}

type TerminalData struct {
	IsOnline    bool    `json:"is_online"`
	CardPresent bool    `json:"card_present"`
	KMFromHome  float64 `json:"km_from_home"`
}

type LastTransactionData struct {
	Timestamp     *time.Time `json:"timestamp"`
	KMFromCurrent *float64   `json:"km_from_current"`
}

func (td *TransactionData) Validate() error {
	if td.Amount <= 0 {
		return errors.New("transaction amount must be positive")
	}
	if td.Installments < 1 {
		return errors.New("installments must be at least 1")
	}
	if td.RequestedAt.After(time.Now()) {
		return errors.New("requested_at must not be in the future")
	}
	return nil
}

func (c *CustomerData) Validate() error {
	if c.AvgAmount < 0 {
		return errors.New("customer avg_amount must not be negative")
	}
	if c.TxCount24h < 0 {
		return errors.New("customer tx_count_24h must not be negative")
	}
	return nil
}

func (m *MerchantData) Validate() error {
	if m.ID == "" {
		return errors.New("merchant id is required")
	}
	if m.MCC == "" {
		return errors.New("merchant mcc is required")
	}
	if m.AvgAmount < 0 {
		return errors.New("merchant avg_amount must not be negative")
	}
	return nil
}

func (t *TerminalData) Validate() error {
	if t.KMFromHome < 0 {
		return errors.New("terminal km_from_home must not be negative")
	}
	return nil
}

func (ltd *LastTransactionData) Validate() error {
	if ltd.Timestamp != nil && ltd.Timestamp.After(time.Now()) {
		return errors.New("last_transaction timestamp must not be in the future")
	}
	if ltd.KMFromCurrent != nil && *ltd.KMFromCurrent < 0 {
		return errors.New("last_transaction km_from_current must not be negative")
	}
	return nil
}

func (tx *Transaction) Validate() error {
	if tx.ID == "" {
		return errors.New("transaction id is required")
	}
	if err := tx.Transaction.Validate(); err != nil {
		return err
	}
	if err := tx.Customer.Validate(); err != nil {
		return err
	}
	if err := tx.Merchant.Validate(); err != nil {
		return err
	}
	if err := tx.Terminal.Validate(); err != nil {
		return err
	}
	if tx.LastTransaction != nil {
		if err := tx.LastTransaction.Validate(); err != nil {
			return err
		}
	}
	return nil
}
