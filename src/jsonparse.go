package src

import (
	"strconv"
	"time"
)

// ParseTransactionJSON parseia o payload JSON diretamente sem reflection.
// Muito mais rapido que encoding/json para payloads fixos.
func ParseTransactionJSON(data []byte, tx *Transaction) error {
	*tx = Transaction{}

	var lastTx LastTransactionData
	var hasLastTx bool

	i := skipWhitespace(data, 0)
	if i >= len(data) || data[i] != '{' {
		return nil
	}
	i++

	for {
		i = skipWhitespace(data, i)
		if i >= len(data) || data[i] == '}' {
			break
		}

		key, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)

		switch key {
		case "id":
			tx.ID, i = parseStringValue(data, i)
		case "transaction":
			i = parseTransactionObject(data, i, tx)
		case "customer":
			i = parseCustomerObject(data, i, tx)
		case "merchant":
			i = parseMerchantObject(data, i, tx)
		case "terminal":
			i = parseTerminalObject(data, i, tx)
		case "last_transaction":
			hasLastTx, end := parseLastTransactionObject(data, i, &lastTx)
			i = end
			hasLastTx = hasLastTx
		default:
			i = skipValue(data, i)
		}

		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}

	if hasLastTx {
		tx.LastTransaction = &lastTx
	}
	return nil
}

// ---------- helpers de parsing ----------

func skipWhitespace(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\n' || data[i] == '\t' || data[i] == '\r') {
		i++
	}
	return i
}

func skipToColon(data []byte, i int) int {
	for i < len(data) && data[i] != ':' {
		i++
	}
	if i < len(data) {
		i++
	}
	return i
}

func parseStringValue(data []byte, i int) (string, int) {
	if i >= len(data) || data[i] != '"' {
		return "", i
	}
	i++
	start := i
	for i < len(data) && data[i] != '"' {
		i++
	}
	return string(data[start:i]), i + 1
}

func parseFloatValue(data []byte, i int) (float64, int, bool) {
	start := i
	if i < len(data) && data[i] == '-' {
		i++
	}
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if i < len(data) && data[i] == '.' {
		i++
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	if start == i {
		return 0, i, false
	}
	f, _ := strconv.ParseFloat(string(data[start:i]), 64)
	return f, i, true
}

func parseIntValue(data []byte, i int) (int, int, bool) {
	start := i
	if i < len(data) && data[i] == '-' {
		i++
	}
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if start == i {
		return 0, i, false
	}
	n, _ := strconv.Atoi(string(data[start:i]))
	return n, i, true
}

func parseBoolValue(data []byte, i int) (bool, int, bool) {
	if i+4 <= len(data) && data[i] == 't' {
		return true, i + 4, true
	}
	if i+5 <= len(data) && data[i] == 'f' {
		return false, i + 5, true
	}
	return false, i, false
}

func parseTimeValue(data []byte, i int) (time.Time, int, bool) {
	s, end := parseStringValue(data, i)
	if end == i {
		return time.Time{}, i, false
	}
	t, err := time.Parse(time.RFC3339, s)
	return t, end, err == nil
}

func skipValue(data []byte, i int) int {
	i = skipWhitespace(data, i)
	if i >= len(data) {
		return i
	}
	switch data[i] {
	case '"':
		_, end := parseStringValue(data, i)
		return end
	case '{':
		return skipObject(data, i)
	case '[':
		return skipArray(data, i)
	case 'n':
		if i+4 <= len(data) {
			return i + 4
		}
		return i + 1
	default:
		for i < len(data) && data[i] != ',' && data[i] != '}' && data[i] != ']' {
			i++
		}
		return i
	}
}

func skipObject(data []byte, i int) int {
	i++ // skip {
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1
		}
		_, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipValue(data, i)
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func skipArray(data []byte, i int) int {
	i++ // skip [
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ']' {
			return i + 1
		}
		i = skipValue(data, i)
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

// ---------- parsers de objetos aninhados ----------

func parseTransactionObject(data []byte, i int, tx *Transaction) int {
	i = skipWhitespace(data, i)
	if i >= len(data) || data[i] != '{' {
		return i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1
		}
		subKey, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)
		switch subKey {
		case "amount":
			tx.Transaction.Amount, i, _ = parseFloatValue(data, i)
		case "installments":
			tx.Transaction.Installments, i, _ = parseIntValue(data, i)
		case "requested_at":
			tx.Transaction.RequestedAt, i, _ = parseTimeValue(data, i)
		default:
			i = skipValue(data, i)
		}
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func parseCustomerObject(data []byte, i int, tx *Transaction) int {
	i = skipWhitespace(data, i)
	if i >= len(data) || data[i] != '{' {
		return i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1
		}
		subKey, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)
		switch subKey {
		case "avg_amount":
			tx.Customer.AvgAmount, i, _ = parseFloatValue(data, i)
		case "tx_count_24h":
			tx.Customer.TxCount24h, i, _ = parseIntValue(data, i)
		case "known_merchants":
			tx.Customer.KnownMerchants, i = parseKnownMerchants(data, i)
		default:
			i = skipValue(data, i)
		}
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func parseKnownMerchants(data []byte, i int) ([]string, int) {
	var merchants []string
	i = skipWhitespace(data, i)
	if i >= len(data) || data[i] != '[' {
		return merchants, i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ']' {
			return merchants, i + 1
		}
		s, end := parseStringValue(data, i)
		merchants = append(merchants, s)
		i = end
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func parseMerchantObject(data []byte, i int, tx *Transaction) int {
	i = skipWhitespace(data, i)
	if i >= len(data) || data[i] != '{' {
		return i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1
		}
		subKey, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)
		switch subKey {
		case "id":
			tx.Merchant.ID, i = parseStringValue(data, i)
		case "mcc":
			tx.Merchant.MCC, i = parseStringValue(data, i)
		case "avg_amount":
			tx.Merchant.AvgAmount, i, _ = parseFloatValue(data, i)
		default:
			i = skipValue(data, i)
		}
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func parseTerminalObject(data []byte, i int, tx *Transaction) int {
	i = skipWhitespace(data, i)
	if i >= len(data) || data[i] != '{' {
		return i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1
		}
		subKey, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)
		switch subKey {
		case "is_online":
			tx.Terminal.IsOnline, i, _ = parseBoolValue(data, i)
		case "card_present":
			tx.Terminal.CardPresent, i, _ = parseBoolValue(data, i)
		case "km_from_home":
			tx.Terminal.KMFromHome, i, _ = parseFloatValue(data, i)
		default:
			i = skipValue(data, i)
		}
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}

func parseLastTransactionObject(data []byte, i int, lastTx *LastTransactionData) (bool, int) {
	i = skipWhitespace(data, i)
	if i+4 <= len(data) && data[i] == 'n' {
		return false, i + 4
	}
	if i >= len(data) || data[i] != '{' {
		return false, i
	}
	i++
	for {
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == '}' {
			return true, i + 1
		}
		subKey, end := parseStringValue(data, i)
		i = end
		i = skipToColon(data, i)
		i = skipWhitespace(data, i)
		switch subKey {
		case "timestamp":
			t, end, ok := parseTimeValue(data, i)
			if ok {
				lastTx.Timestamp = &t
			}
			i = end
		case "km_from_current":
			f, end, ok := parseFloatValue(data, i)
			if ok {
				lastTx.KMFromCurrent = &f
			}
			i = end
		default:
			i = skipValue(data, i)
		}
		i = skipWhitespace(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}
}
