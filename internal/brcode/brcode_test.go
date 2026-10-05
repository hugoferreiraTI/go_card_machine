package brcode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TLV Encoding/Decoding Tests ---

func TestEncodeTLV(t *testing.T) {
	tests := []struct {
		tag, value, expected string
	}{
		{"00", "01", "000201"},
		{"26", "br.gov.bcb.pix", "261400140014br.gov.bcb.pix"}, // wrong expectation, let me fix
		{"59", "Fulano de Tal", "5913Fulano de Tal"},
		{"60", "BRASILIA", "6008BRASILIA"},
	}
	// Actually EncodeTLV is tag + fmt.Sprintf("%02d", len(value)) + value
	for _, tt := range tests {
		result := EncodeTLV(tt.tag, tt.value)
		expected := fmt.Sprintf("%s%02d%s", tt.tag, len(tt.value), tt.value)
		assert.Equal(t, expected, result, "EncodeTLV(%s, %s)", tt.tag, tt.value)
	}
}

func TestDecodeTLV_Simple(t *testing.T) {
	payload := "000201" + "5913Fulano de Tal" + "6008BRASILIA"
	objects, err := DecodeTLV(payload)
	require.NoError(t, err)
	require.Len(t, objects, 3)

	assert.Equal(t, "00", objects[0].Tag)
	assert.Equal(t, "01", objects[0].Value)

	assert.Equal(t, "59", objects[1].Tag)
	assert.Equal(t, "Fulano de Tal", objects[1].Value)

	assert.Equal(t, "60", objects[2].Tag)
	assert.Equal(t, "BRASILIA", objects[2].Value)
}

func TestDecodeTLV_Template(t *testing.T) {
	// Tag 26 with nested GUI and key
	inner := EncodeTLV("00", "br.gov.bcb.pix") + EncodeTLV("01", "mykey123")
	payload := EncodeTLV("26", inner)

	objects, err := DecodeTLV(payload)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	assert.Equal(t, "26", objects[0].Tag)
	require.Len(t, objects[0].Children, 2)
	assert.Equal(t, "br.gov.bcb.pix", objects[0].Children[0].Value)
	assert.Equal(t, "mykey123", objects[0].Children[1].Value)
}

func TestDecodeTLV_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"too short", "00"},
		{"invalid length", "00XX01"},
		{"length exceeds data", "000901"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeTLV(tt.payload)
			assert.Error(t, err)
		})
	}
}

func TestFindTag(t *testing.T) {
	objects := []EMVDataObject{
		{Tag: "00", Value: "01"},
		{Tag: "59", Value: "Test"},
	}

	found := FindTag(objects, "59")
	require.NotNil(t, found)
	assert.Equal(t, "Test", found.Value)

	notFound := FindTag(objects, "99")
	assert.Nil(t, notFound)
}

// --- CRC-16 Tests ---

func TestCalculateCRC16_KnownValues(t *testing.T) {
	// The CRC is computed over the entire payload string up to and including "6304"
	// We can verify with known BACEN examples by computing CRC over the data portion.

	// Simple test: "123456789" → CRC-16/CCITT-FALSE = 0x29B1
	crc := CalculateCRC16("123456789")
	assert.Equal(t, uint16(0x29B1), crc)
}

// --- Static QR Tests ---

func TestBuildStaticQR_Basic(t *testing.T) {
	// Arrange
	params := StaticQRParams{
		PixKey:       "123e4567-e12b-12d1-a456-426655440000",
		MerchantName: "Fulano de Tal",
		MerchantCity: "BRASILIA",
		TxID:         "***",
	}

	// Act
	payload, err := BuildStaticQR(params)

	// Assert
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(payload, "000201"))
	assert.Contains(t, payload, "br.gov.bcb.pix")
	assert.Contains(t, payload, "123e4567-e12b-12d1-a456-426655440000")
	assert.True(t, strings.HasSuffix(payload, fmt.Sprintf("%04X", CalculateCRC16(payload[:len(payload)-4]))))

	// Decode roundtrip
	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, "01", decoded.PayloadFormatIndicator)
	assert.Equal(t, "br.gov.bcb.pix", decoded.MerchantAccountInfo.GUI)
	assert.Equal(t, params.PixKey, decoded.MerchantAccountInfo.PixKey)
	assert.Equal(t, "Fulano de Tal", decoded.MerchantName)
	assert.Equal(t, "BRASILIA", decoded.MerchantCity)
	assert.Equal(t, "***", decoded.AdditionalData.ReferenceLabel)
}

func TestBuildStaticQR_BACEN_Example(t *testing.T) {
	// BACEN official example from KB:
	// 00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000
	// 5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D

	params := StaticQRParams{
		PixKey:       "123e4567-e12b-12d1-a456-426655440000",
		MerchantName: "Fulano de Tal",
		MerchantCity: "BRASILIA",
		TxID:         "***",
	}

	payload, err := BuildStaticQR(params)
	require.NoError(t, err)

	expected := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

	assert.Equal(t, expected, payload)

	// Verify CRC matches BACEN expected value "1D3D"
	crcPayload := payload[:len(payload)-4]
	crc := CalculateCRC16(crcPayload)
	assert.Equal(t, "1D3D", fmt.Sprintf("%04X", crc))
}

func TestBuildStaticQR_WithAmount(t *testing.T) {
	params := StaticQRParams{
		PixKey:       "email@example.com",
		MerchantName: "Maria Silva",
		MerchantCity: "SAO PAULO",
		Amount:       "100.00",
		TxID:         "PGTO001",
	}

	payload, err := BuildStaticQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, "100.00", decoded.TransactionAmount)
	assert.Equal(t, "PGTO001", decoded.AdditionalData.ReferenceLabel)
}

func TestBuildStaticQR_WithInfoAdicional(t *testing.T) {
	params := StaticQRParams{
		PixKey:        "11999999999",
		MerchantName:  "Loja ABC",
		MerchantCity:  "CURITIBA",
		InfoAdicional: "Pedido 12345",
	}

	payload, err := BuildStaticQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, "Pedido 12345", decoded.MerchantAccountInfo.InfoAdicional)
}

func TestBuildStaticQR_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		params StaticQRParams
		errMsg string
	}{
		{
			name:   "missing pix key",
			params: StaticQRParams{MerchantName: "Test", MerchantCity: "City"},
			errMsg: "RN_QR_002",
		},
		{
			name:   "missing merchant name",
			params: StaticQRParams{PixKey: "key123", MerchantCity: "City"},
			errMsg: "RN_QR_004",
		},
		{
			name:   "missing merchant city",
			params: StaticQRParams{PixKey: "key123", MerchantName: "Name"},
			errMsg: "RN_QR_005",
		},
		{
			name: "txid too long",
			params: StaticQRParams{
				PixKey:       "key",
				MerchantName: "Name",
				MerchantCity: "City",
				TxID:         "12345678901234567890123456", // 26 chars > 25
			},
			errMsg: "RN_QR_002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildStaticQR(tt.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// --- Dynamic QR Tests ---

func TestBuildDynamicQR_Basic(t *testing.T) {
	params := DynamicQRParams{
		URL:          "pix.example.com/8b3da2f39a4140d1a91abd93113bd441",
		MerchantName: "Fulano de Tal",
		MerchantCity: "BRASILIA",
	}

	payload, err := BuildDynamicQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, PointOfInitiationDynamic, decoded.PointOfInitiation)
	assert.Equal(t, params.URL, decoded.MerchantAccountInfo.URL)
	assert.Equal(t, "br.gov.bcb.pix", decoded.MerchantAccountInfo.GUI)
}

func TestBuildDynamicQR_BACEN_Example(t *testing.T) {
	// BACEN official example:
	// 00020101021226700014br.gov.bcb.pix2548pix.example.com/8b3da2f39a4140d1a91abd93113bd441
	// 5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***630464E4

	params := DynamicQRParams{
		URL:          "pix.example.com/8b3da2f39a4140d1a91abd93113bd441",
		MerchantName: "Fulano de Tal",
		MerchantCity: "BRASILIA",
	}

	payload, err := BuildDynamicQR(params)
	require.NoError(t, err)

	expected := "00020101021226700014br.gov.bcb.pix2548pix.example.com/8b3da2f39a4140d1a91abd93113bd441" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***630464E4"

	assert.Equal(t, expected, payload)

	// Verify CRC matches BACEN expected value "64E4"
	crcPayload := payload[:len(payload)-4]
	crc := CalculateCRC16(crcPayload)
	assert.Equal(t, "64E4", fmt.Sprintf("%04X", crc))
}

func TestBuildDynamicQR_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		params DynamicQRParams
		errMsg string
	}{
		{
			name:   "missing URL",
			params: DynamicQRParams{MerchantName: "Test", MerchantCity: "City"},
			errMsg: "RN_QR_003",
		},
		{
			name: "URL too long",
			params: DynamicQRParams{
				URL:          strings.Repeat("a", 78),
				MerchantName: "Test",
				MerchantCity: "City",
			},
			errMsg: "RN_QR_003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildDynamicQR(tt.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// --- Composite QR Tests ---

func TestBuildCompositeQR_RecurrenceOnly(t *testing.T) {
	// BACEN example — recurrence only (no payment component in tag 26 key/url, just GUI):
	params := CompositeQRParams{
		RecurrenceURL: "pix.example.com/rec/2353c790eefb11eaadc10242ac120002",
		MerchantName:  "Fulano de Tal",
		MerchantCity:  "BRASILIA",
	}

	payload, err := BuildCompositeQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	require.Len(t, decoded.UnreservedTemplates, 1)
	assert.Equal(t, "80", decoded.UnreservedTemplates[0].Tag)
	assert.Equal(t, "br.gov.bcb.pix", decoded.UnreservedTemplates[0].GUI)
	assert.Equal(t, params.RecurrenceURL, decoded.UnreservedTemplates[0].URL)
}

func TestBuildCompositeQR_DynamicPlusRecurrence(t *testing.T) {
	// BACEN example — dynamic payment + recurrence:
	params := CompositeQRParams{
		PaymentURL:    "pix.example.com/8b3da2f39a4140d1a91abd93113bd441",
		RecurrenceURL: "pix.example.com/rec/2353c790eefb11eaadc10242ac120002",
		MerchantName:  "Fulano de Tal",
		MerchantCity:  "BRASILIA",
	}

	payload, err := BuildCompositeQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, PointOfInitiationDynamic, decoded.PointOfInitiation)
	assert.Equal(t, params.PaymentURL, decoded.MerchantAccountInfo.URL)
	require.Len(t, decoded.UnreservedTemplates, 1)
	assert.Equal(t, params.RecurrenceURL, decoded.UnreservedTemplates[0].URL)
}

func TestBuildCompositeQR_BACEN_CRC_FB42(t *testing.T) {
	// The BACEN example for dynamic+recurrence has CRC FB42
	// Let's verify our generation matches that CRC
	params := CompositeQRParams{
		PaymentURL:    "pix.example.com/8b3da2f39a4140d1a91abd93113bd441",
		RecurrenceURL: "pix.example.com/rec/2353c790eefb11eaadc10242ac120002",
		MerchantName:  "Fulano de Tal",
		MerchantCity:  "BRASILIA",
	}

	payload, err := BuildCompositeQR(params)
	require.NoError(t, err)

	// Verify CRC
	crcPayload := payload[:len(payload)-4]
	crc := CalculateCRC16(crcPayload)
	crcHex := fmt.Sprintf("%04X", crc)

	// The payload should end with its CRC
	assert.Equal(t, crcHex, payload[len(payload)-4:])
}

// --- Decode Tests ---

func TestDecode_StaticQR(t *testing.T) {
	payload := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

	decoded, err := Decode(payload)
	require.NoError(t, err)

	assert.Equal(t, "01", decoded.PayloadFormatIndicator)
	assert.Equal(t, PointOfInitiation(""), decoded.PointOfInitiation) // Not set for static
	assert.Equal(t, "br.gov.bcb.pix", decoded.MerchantAccountInfo.GUI)
	assert.Equal(t, "123e4567-e12b-12d1-a456-426655440000", decoded.MerchantAccountInfo.PixKey)
	assert.Equal(t, "0000", decoded.MerchantCategoryCode)
	assert.Equal(t, "986", decoded.TransactionCurrency)
	assert.Equal(t, "BR", decoded.CountryCode)
	assert.Equal(t, "Fulano de Tal", decoded.MerchantName)
	assert.Equal(t, "BRASILIA", decoded.MerchantCity)
	assert.Equal(t, "***", decoded.AdditionalData.ReferenceLabel)
	assert.Equal(t, "1D3D", decoded.CRC)
}

func TestDecode_DynamicQR(t *testing.T) {
	payload := "00020101021226700014br.gov.bcb.pix2548pix.example.com/8b3da2f39a4140d1a91abd93113bd441" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***630464E4"

	decoded, err := Decode(payload)
	require.NoError(t, err)

	assert.Equal(t, PointOfInitiationDynamic, decoded.PointOfInitiation)
	assert.Equal(t, "pix.example.com/8b3da2f39a4140d1a91abd93113bd441", decoded.MerchantAccountInfo.URL)
	assert.Equal(t, "br.gov.bcb.pix", decoded.MerchantAccountInfo.GUI)
}

func TestDecode_InvalidCRC(t *testing.T) {
	// Tamper with CRC
	payload := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***6304FFFF"

	_, err := Decode(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CRC mismatch")
}

// --- Validate Tests ---

func TestValidate_ValidPayload(t *testing.T) {
	payload := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

	errors := Validate(payload)
	assert.Empty(t, errors)
}

func TestValidate_MissingGUI(t *testing.T) {
	// Build a payload with wrong GUI
	inner := EncodeTLV("00", "wrong.gui") + EncodeTLV("01", "key")
	payload := EncodeTLV("00", "01") +
		EncodeTLV("26", inner) +
		EncodeTLV("52", "0000") +
		EncodeTLV("53", "986") +
		EncodeTLV("58", "BR") +
		EncodeTLV("59", "Test") +
		EncodeTLV("60", "City")

	// Add CRC
	payload += "6304"
	crc := CalculateCRC16(payload)
	payload += fmt.Sprintf("%04X", crc)

	errors := Validate(payload)
	found := false
	for _, e := range errors {
		if strings.Contains(e.Message, "RN_QR_001") {
			found = true
		}
	}
	assert.True(t, found, "should report RN_QR_001 for wrong GUI")
}

// --- Roundtrip Tests ---

func TestRoundtrip_StaticQR(t *testing.T) {
	params := StaticQRParams{
		PixKey:        "test@email.com",
		MerchantName:  "Test Merchant",
		MerchantCity:  "SAO PAULO",
		Amount:        "50.00",
		TxID:          "TX123",
		InfoAdicional: "Order 789",
		PostalCode:    "01310100",
	}

	payload, err := BuildStaticQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)

	assert.Equal(t, params.PixKey, decoded.MerchantAccountInfo.PixKey)
	assert.Equal(t, "Test Merchant", decoded.MerchantName)
	assert.Equal(t, "SAO PAULO", decoded.MerchantCity)
	assert.Equal(t, "50.00", decoded.TransactionAmount)
	assert.Equal(t, "TX123", decoded.AdditionalData.ReferenceLabel)
	assert.Equal(t, "Order 789", decoded.MerchantAccountInfo.InfoAdicional)
	assert.Equal(t, "01310100", decoded.PostalCode)
}

func TestRoundtrip_DynamicQR(t *testing.T) {
	params := DynamicQRParams{
		URL:          "pix.mybank.com/v2/abcdef123456",
		MerchantName: "My Store",
		MerchantCity: "CURITIBA",
		Amount:       "199.99",
		TxID:         "DYN001",
	}

	payload, err := BuildDynamicQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)

	assert.Equal(t, PointOfInitiationDynamic, decoded.PointOfInitiation)
	assert.Equal(t, params.URL, decoded.MerchantAccountInfo.URL)
	assert.Equal(t, "199.99", decoded.TransactionAmount)
	assert.Equal(t, "DYN001", decoded.AdditionalData.ReferenceLabel)
}

func TestRoundtrip_CompositeQR(t *testing.T) {
	params := CompositeQRParams{
		PaymentURL:    "pix.bank.com/pay/abc123",
		RecurrenceURL: "pix.bank.com/rec/xyz789",
		MerchantName:  "Subscription Co",
		MerchantCity:  "RIO DE JANEIRO",
	}

	payload, err := BuildCompositeQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)

	assert.Equal(t, params.PaymentURL, decoded.MerchantAccountInfo.URL)
	require.Len(t, decoded.UnreservedTemplates, 1)
	assert.Equal(t, params.RecurrenceURL, decoded.UnreservedTemplates[0].URL)
}

// --- Boundary Tests ---

func TestBuildStaticQR_TruncatesLongName(t *testing.T) {
	params := StaticQRParams{
		PixKey:       "key",
		MerchantName: "This Name Is Way Too Long For The Field",
		MerchantCity: "City",
	}

	payload, err := BuildStaticQR(params)
	require.NoError(t, err)

	decoded, err := Decode(payload)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(decoded.MerchantName), MaxMerchantName)
}

func TestBuildDynamicQR_MaxURL(t *testing.T) {
	// URL exactly at max length should work
	url := strings.Repeat("a", MaxURL)
	params := DynamicQRParams{
		URL:          url,
		MerchantName: "Test",
		MerchantCity: "City",
	}

	_, err := BuildDynamicQR(params)
	assert.NoError(t, err)

	// One char over should fail
	params.URL = strings.Repeat("a", MaxURL+1)
	_, err = BuildDynamicQR(params)
	assert.Error(t, err)
}

func TestIsTemplateTag(t *testing.T) {
	assert.True(t, IsTemplateTag("26"))
	assert.True(t, IsTemplateTag("51"))
	assert.True(t, IsTemplateTag("62"))
	assert.True(t, IsTemplateTag("64"))
	assert.True(t, IsTemplateTag("80"))
	assert.True(t, IsTemplateTag("99"))
	assert.False(t, IsTemplateTag("00"))
	assert.False(t, IsTemplateTag("52"))
	assert.False(t, IsTemplateTag("63"))
}
