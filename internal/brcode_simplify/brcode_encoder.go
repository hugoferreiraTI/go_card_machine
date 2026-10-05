package brcode

import (
	"fmt"
	"strings"
)

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
