package brcode

import (
	"fmt"
	"strings"
)

// BuildStaticQR generates a complete BR Code payload for a static PIX QR code.
// Static QR codes can be reused and contain the PIX key directly.
//
// Business rules:
//   - RN_QR_001: GUI must be "br.gov.bcb.pix"
//   - RN_QR_002: PIX key required, txID <= 25 chars
//   - RN_QR_004: Merchant name <= 25 chars
//   - RN_QR_005: Merchant city <= 15 chars
//   - RN_QR_006: Merchant Account Info total <= 99 chars
func BuildStaticQR(params StaticQRParams) (string, error) {
	if params.PixKey == "" {
		return "", fmt.Errorf("RN_QR_002: PIX key (chave) is required for static QR")
	}
	if params.MerchantName == "" {
		return "", fmt.Errorf("RN_QR_004: merchant name is required")
	}
	if params.MerchantCity == "" {
		return "", fmt.Errorf("RN_QR_005: merchant city is required")
	}

	name := truncate(params.MerchantName, MaxMerchantName)
	city := truncate(params.MerchantCity, MaxMerchantCity)

	txid := params.TxID
	if txid == "" {
		txid = "***"
	}
	if len(txid) > MaxStaticTxID {
		return "", fmt.Errorf("RN_QR_002: static QR txID must be at most %d chars, got %d", MaxStaticTxID, len(txid))
	}

	// Build Merchant Account Information (tag 26)
	maiChildren := []EMVDataObject{
		{Tag: SubTagGUI, Value: PIXGUI},
		{Tag: SubTagPixKey, Value: params.PixKey},
	}
	if params.InfoAdicional != "" {
		info := truncate(params.InfoAdicional, MaxInfoAdicional)
		maiChildren = append(maiChildren, EMVDataObject{Tag: SubTagInfoAdicional, Value: info})
	}
	if params.FSS != "" {
		maiChildren = append(maiChildren, EMVDataObject{Tag: SubTagFSS, Value: params.FSS})
	}

	// RN_QR_006: Validate MAI total length
	maiPayload := ""
	for _, child := range maiChildren {
		maiPayload += EncodeTLV(child.Tag, child.Value)
	}
	if len(maiPayload) > MaxMerchantAccountInfo {
		return "", fmt.Errorf("RN_QR_006: merchant account info exceeds %d chars (got %d)", MaxMerchantAccountInfo, len(maiPayload))
	}

	// Build full payload
	var b strings.Builder

	b.WriteString(EncodeTLV(TagPayloadFormatIndicator, PayloadFormatIndicator))
	// Static QR: no Point of Initiation Method (tag 01) → defaults to "11" (reusable)
	b.WriteString(EncodeTemplateTLV(TagMerchantAccountInfo, maiChildren))
	b.WriteString(EncodeTLV(TagMerchantCategoryCode, MerchantCategoryCode))
	b.WriteString(EncodeTLV(TagTransactionCurrency, CurrencyBRL))

	if params.Amount != "" {
		b.WriteString(EncodeTLV(TagTransactionAmount, params.Amount))
	}

	b.WriteString(EncodeTLV(TagCountryCode, CountryBR))
	b.WriteString(EncodeTLV(TagMerchantName, name))
	b.WriteString(EncodeTLV(TagMerchantCity, city))

	if params.PostalCode != "" {
		pc := truncate(params.PostalCode, MaxPostalCode)
		b.WriteString(EncodeTLV(TagPostalCode, pc))
	}

	// Additional Data Field (tag 62)
	adChildren := []EMVDataObject{
		{Tag: SubTagReferenceLabel, Value: txid},
	}
	b.WriteString(EncodeTemplateTLV(TagAdditionalDataField, adChildren))

	// CRC placeholder: tag 63, length 04, then compute CRC over everything including "6304"
	b.WriteString(TagCRC + "04")
	crc := CalculateCRC16(b.String())
	b.WriteString(fmt.Sprintf("%04X", crc))

	return b.String(), nil
}

// BuildDynamicQR generates a complete BR Code payload for a dynamic PIX QR code.
// Dynamic QR codes contain a URL pointing to the payment payload and are single-use.
//
// Business rules:
//   - RN_QR_003: URL required, max 77 chars, no protocol prefix
//   - RN_QR_004: Merchant name <= 25 chars
//   - RN_QR_005: Merchant city <= 15 chars
func BuildDynamicQR(params DynamicQRParams) (string, error) {
	if params.URL == "" {
		return "", fmt.Errorf("RN_QR_003: URL is required for dynamic QR")
	}
	if len(params.URL) > MaxURL {
		return "", fmt.Errorf("RN_QR_003: dynamic QR URL must be at most %d chars, got %d", MaxURL, len(params.URL))
	}
	if params.MerchantName == "" {
		return "", fmt.Errorf("RN_QR_004: merchant name is required")
	}
	if params.MerchantCity == "" {
		return "", fmt.Errorf("RN_QR_005: merchant city is required")
	}

	name := truncate(params.MerchantName, MaxMerchantName)
	city := truncate(params.MerchantCity, MaxMerchantCity)

	txid := params.TxID
	if txid == "" {
		txid = "***"
	}

	// Build Merchant Account Information (tag 26) with URL
	maiChildren := []EMVDataObject{
		{Tag: SubTagGUI, Value: PIXGUI},
		{Tag: SubTagURL, Value: params.URL},
	}

	var b strings.Builder

	b.WriteString(EncodeTLV(TagPayloadFormatIndicator, PayloadFormatIndicator))
	b.WriteString(EncodeTLV(TagPointOfInitiationMethod, string(PointOfInitiationDynamic)))
	b.WriteString(EncodeTemplateTLV(TagMerchantAccountInfo, maiChildren))
	b.WriteString(EncodeTLV(TagMerchantCategoryCode, MerchantCategoryCode))
	b.WriteString(EncodeTLV(TagTransactionCurrency, CurrencyBRL))

	if params.Amount != "" {
		b.WriteString(EncodeTLV(TagTransactionAmount, params.Amount))
	}

	b.WriteString(EncodeTLV(TagCountryCode, CountryBR))
	b.WriteString(EncodeTLV(TagMerchantName, name))
	b.WriteString(EncodeTLV(TagMerchantCity, city))

	if params.PostalCode != "" {
		pc := truncate(params.PostalCode, MaxPostalCode)
		b.WriteString(EncodeTLV(TagPostalCode, pc))
	}

	adChildren := []EMVDataObject{
		{Tag: SubTagReferenceLabel, Value: txid},
	}
	b.WriteString(EncodeTemplateTLV(TagAdditionalDataField, adChildren))

	b.WriteString(TagCRC + "04")
	crc := CalculateCRC16(b.String())
	b.WriteString(fmt.Sprintf("%04X", crc))

	return b.String(), nil
}

// BuildCompositeQR generates a composite BR Code QR containing both payment (tag 26)
// and recurrence (tag 80) information.
// Used for Pix Automatico jornadas 2-4.
func BuildCompositeQR(params CompositeQRParams) (string, error) {
	if params.RecurrenceURL == "" {
		return "", fmt.Errorf("recurrence URL is required for composite QR")
	}
	if len(params.RecurrenceURL) > MaxURL {
		return "", fmt.Errorf("recurrence URL must be at most %d chars, got %d", MaxURL, len(params.RecurrenceURL))
	}
	if params.MerchantName == "" {
		return "", fmt.Errorf("RN_QR_004: merchant name is required")
	}
	if params.MerchantCity == "" {
		return "", fmt.Errorf("RN_QR_005: merchant city is required")
	}

	name := truncate(params.MerchantName, MaxMerchantName)
	city := truncate(params.MerchantCity, MaxMerchantCity)

	txid := params.TxID
	if txid == "" {
		txid = "***"
	}

	var b strings.Builder

	b.WriteString(EncodeTLV(TagPayloadFormatIndicator, PayloadFormatIndicator))

	// If there's a payment component (dynamic URL or static key), set point of initiation
	hasPayment := params.PaymentURL != "" || params.PixKey != ""
	if hasPayment && params.PaymentURL != "" {
		b.WriteString(EncodeTLV(TagPointOfInitiationMethod, string(PointOfInitiationDynamic)))
	}

	// Build Merchant Account Information (tag 26) for payment
	var maiChildren []EMVDataObject
	maiChildren = append(maiChildren, EMVDataObject{Tag: SubTagGUI, Value: PIXGUI})
	if params.PaymentURL != "" {
		if len(params.PaymentURL) > MaxURL {
			return "", fmt.Errorf("payment URL must be at most %d chars", MaxURL)
		}
		maiChildren = append(maiChildren, EMVDataObject{Tag: SubTagURL, Value: params.PaymentURL})
	} else if params.PixKey != "" {
		maiChildren = append(maiChildren, EMVDataObject{Tag: SubTagPixKey, Value: params.PixKey})
	}

	b.WriteString(EncodeTemplateTLV(TagMerchantAccountInfo, maiChildren))
	b.WriteString(EncodeTLV(TagMerchantCategoryCode, MerchantCategoryCode))
	b.WriteString(EncodeTLV(TagTransactionCurrency, CurrencyBRL))

	if params.Amount != "" {
		b.WriteString(EncodeTLV(TagTransactionAmount, params.Amount))
	}

	b.WriteString(EncodeTLV(TagCountryCode, CountryBR))
	b.WriteString(EncodeTLV(TagMerchantName, name))
	b.WriteString(EncodeTLV(TagMerchantCity, city))

	if params.PostalCode != "" {
		pc := truncate(params.PostalCode, MaxPostalCode)
		b.WriteString(EncodeTLV(TagPostalCode, pc))
	}

	adChildren := []EMVDataObject{
		{Tag: SubTagReferenceLabel, Value: txid},
	}
	b.WriteString(EncodeTemplateTLV(TagAdditionalDataField, adChildren))

	// Unreserved template tag 80: recurrence URL
	recChildren := []EMVDataObject{
		{Tag: SubTagGUI, Value: PIXGUI},
		{Tag: SubTagURL, Value: params.RecurrenceURL},
	}
	b.WriteString(EncodeTemplateTLV(TagUnreservedTemplate80, recChildren))

	b.WriteString(TagCRC + "04")
	crc := CalculateCRC16(b.String())
	b.WriteString(fmt.Sprintf("%04X", crc))

	return b.String(), nil
}

// CalculateCRC16 computes CRC-16/CCITT-FALSE for BR Code verification.
// Polynomial: 0x1021, Initial value: 0xFFFF.
func CalculateCRC16(data string) uint16 {
	var crc uint16 = 0xFFFF
	for i := 0; i < len(data); i++ {
		crc ^= uint16(data[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// truncate truncates a string to the specified maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
