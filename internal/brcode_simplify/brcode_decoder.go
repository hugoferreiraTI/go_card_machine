package brcode

import (
	"fmt"
	"strings"
)

// Decode parses a BR Code payload string into a structured BRCodeData.
// Validates the CRC-16 checksum and extracts all recognized fields.
func Decode(payload string) (*BRCodeData, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("payload too short: minimum 8 chars required")
	}

	// Verify CRC-16 before decoding
	errs := validateCRC(payload)
	if len(errs) > 0 {
		return nil, errs[0]
	}

	objects, err := DecodeTLV(payload)
	if err != nil {
		return nil, fmt.Errorf("decode TLV: %w", err)
	}

	data := &BRCodeData{}

	for _, obj := range objects {
		switch obj.Tag {
		case TagPayloadFormatIndicator:
			data.PayloadFormatIndicator = obj.Value
		case TagPointOfInitiationMethod:
			data.PointOfInitiation = PointOfInitiation(obj.Value)
		case TagMerchantCategoryCode:
			data.MerchantCategoryCode = obj.Value
		case TagTransactionCurrency:
			data.TransactionCurrency = obj.Value
		case TagTransactionAmount:
			data.TransactionAmount = obj.Value
		case TagCountryCode:
			data.CountryCode = obj.Value
		case TagMerchantName:
			data.MerchantName = obj.Value
		case TagMerchantCity:
			data.MerchantCity = obj.Value
		case TagPostalCode:
			data.PostalCode = obj.Value
		case TagCRC:
			data.CRC = obj.Value
		}

		// Merchant Account Information (tags 26-51)
		if isMAITag(obj.Tag) && len(obj.Children) > 0 {
			mai := &MerchantAccountInfo{}
			for _, child := range obj.Children {
				switch child.Tag {
				case SubTagGUI:
					mai.GUI = child.Value
				case SubTagURL:
					mai.URL = child.Value
				}
			}
			data.MerchantAccountInfo = mai
		}

		// Additional Data Field (tag 62)
		if obj.Tag == TagAdditionalDataField && len(obj.Children) > 0 {
			ad := &AdditionalData{}
			for _, child := range obj.Children {
				if child.Tag == SubTagReferenceLabel {
					ad.ReferenceLabel = child.Value
				}
			}
			data.AdditionalData = ad
		}

	}

	return data, nil
}

// Validate checks a BR Code payload for correctness.
// Returns a list of validation errors (empty if valid).
func Validate(payload string) []ValidationError {
	var errors []ValidationError

	if len(payload) < 8 {
		errors = append(errors, ValidationError{Field: "payload", Message: "too short"})
		return errors
	}

	// CRC validation
	crcErrors := validateCRC(payload)
	errors = append(errors, crcErrors...)

	// Parse TLV
	objects, err := DecodeTLV(payload)
	if err != nil {
		errors = append(errors, ValidationError{Field: "tlv", Message: err.Error()})
		return errors
	}

	// Mandatory tag 00 (Payload Format Indicator)
	if tag00 := FindTag(objects, TagPayloadFormatIndicator); tag00 == nil {
		errors = append(errors, ValidationError{Field: "tag00", Message: "payload format indicator is required"})
	} else if tag00.Value != PayloadFormatIndicator {
		errors = append(errors, ValidationError{Field: "tag00", Message: fmt.Sprintf("expected %q, got %q", PayloadFormatIndicator, tag00.Value)})
	}

	// At least one MAI tag (26-51) must have GUI = br.gov.bcb.pix
	foundGUI := false
	for _, obj := range objects {
		if isMAITag(obj.Tag) {
			for _, child := range obj.Children {
				if child.Tag == SubTagGUI && strings.EqualFold(child.Value, PIXGUI) {
					foundGUI = true
					break
				}
			}
		}
	}
	if !foundGUI {
		errors = append(errors, ValidationError{
			Field:   "tag26.00",
			Message: fmt.Sprintf("RN_QR_001: GUI must be %q", PIXGUI),
		})
	}

	// Mandatory tag 52 (MCC)
	if FindTag(objects, TagMerchantCategoryCode) == nil {
		errors = append(errors, ValidationError{Field: "tag52", Message: "merchant category code is required"})
	}

	// Mandatory tag 53 (Currency)
	if tag53 := FindTag(objects, TagTransactionCurrency); tag53 == nil {
		errors = append(errors, ValidationError{Field: "tag53", Message: "transaction currency is required"})
	} else if tag53.Value != CurrencyBRL {
		errors = append(errors, ValidationError{Field: "tag53", Message: fmt.Sprintf("expected %q (BRL), got %q", CurrencyBRL, tag53.Value)})
	}

	// Mandatory tag 58 (Country)
	if tag58 := FindTag(objects, TagCountryCode); tag58 == nil {
		errors = append(errors, ValidationError{Field: "tag58", Message: "country code is required"})
	} else if tag58.Value != CountryBR {
		errors = append(errors, ValidationError{Field: "tag58", Message: fmt.Sprintf("expected %q, got %q", CountryBR, tag58.Value)})
	}

	// Mandatory tag 59 (Merchant Name)
	if FindTag(objects, TagMerchantName) == nil {
		errors = append(errors, ValidationError{Field: "tag59", Message: "merchant name is required"})
	}

	// Mandatory tag 60 (Merchant City)
	if FindTag(objects, TagMerchantCity) == nil {
		errors = append(errors, ValidationError{Field: "tag60", Message: "merchant city is required"})
	}

	// Mandatory tag 63 (CRC)
	if FindTag(objects, TagCRC) == nil {
		errors = append(errors, ValidationError{Field: "tag63", Message: "CRC is required"})
	}

	return errors
}

// validateCRC checks the CRC-16 checksum of a BR Code payload.
func validateCRC(payload string) []ValidationError {
	if len(payload) < 8 {
		return []ValidationError{{Field: "crc", Message: "payload too short for CRC validation"}}
	}

	// CRC covers everything except the 4-char CRC value itself
	// The payload ends with "6304XXXX" where XXXX is the CRC
	dataForCRC := payload[:len(payload)-4]
	expectedCRC := payload[len(payload)-4:]

	computed := CalculateCRC16(dataForCRC)
	computedHex := fmt.Sprintf("%04X", computed)

	if !strings.EqualFold(computedHex, expectedCRC) {
		return []ValidationError{{
			Field:   "tag63",
			Message: fmt.Sprintf("CRC mismatch: computed %s, found %s", computedHex, expectedCRC),
		}}
	}

	return nil
}

// isMAITag returns true if the tag is in the Merchant Account Information range (26-51).
func isMAITag(tag string) bool {
	if len(tag) != 2 {
		return false
	}
	n := (tag[0]-'0')*10 + (tag[1] - '0')
	return n >= 26 && n <= 51
}
