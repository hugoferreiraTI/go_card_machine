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
		{"25", "pix.example.com/pay/123", "2523pix.example.com/pay/123"},
		{"59", "Fulano de Tal", "5913Fulano de Tal"},
		{"60", "BRASILIA", "6008BRASILIA"},
	}
	for _, tt := range tests {
		result := EncodeTLV(tt.tag, tt.value)
		assert.Equal(t, tt.expected, result, "EncodeTLV(%s, %s)", tt.tag, tt.value)
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
	// Tag 26 with nested GUI and dynamic URL
	inner := EncodeTLV("00", "br.gov.bcb.pix") + EncodeTLV("25", "pix.example.com/pay/123")
	payload := EncodeTLV("26", inner)

	objects, err := DecodeTLV(payload)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	assert.Equal(t, "26", objects[0].Tag)
	require.Len(t, objects[0].Children, 2)
	assert.Equal(t, "br.gov.bcb.pix", objects[0].Children[0].Value)
	assert.Equal(t, "pix.example.com/pay/123", objects[0].Children[1].Value)
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

// --- Decode Tests ---

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
	payload := "00020101021226700014br.gov.bcb.pix2548pix.example.com/8b3da2f39a4140d1a91abd93113bd441" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***6304FFFF"

	_, err := Decode(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CRC mismatch")
}

// --- Validate Tests ---

func TestValidate_ValidPayload(t *testing.T) {
	payload := "00020101021226700014br.gov.bcb.pix2548pix.example.com/8b3da2f39a4140d1a91abd93113bd441" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***630464E4"

	errors := Validate(payload)
	assert.Empty(t, errors)
}

func TestValidate_MissingGUI(t *testing.T) {
	// Build a payload with wrong GUI
	inner := EncodeTLV("00", "wrong.gui") + EncodeTLV("25", "pix.example.com/pay/123")
	payload := EncodeTLV("00", "01") +
		EncodeTLV("01", "12") +
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

// --- Boundary Tests ---

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

func TestBuildDynamicQR_MissingMerchantFields(t *testing.T) {
	for _, tc := range []struct {
		name, merchantName, merchantCity, rule string
	}{
		{"name", "", "City", "RN_QR_004"},
		{"city", "Name", "", "RN_QR_005"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := BuildDynamicQR(DynamicQRParams{
				URL: "pix.example.com/pay/123", MerchantName: tc.merchantName, MerchantCity: tc.merchantCity,
			})
			require.Error(t, err)
			assert.Empty(t, payload)
			assert.Contains(t, err.Error(), tc.rule)
		})
	}
}

func TestBuildDynamicQR_TruncationAndOptionalFields(t *testing.T) {
	params := DynamicQRParams{
		URL:          "pix.example.com/pay/123",
		MerchantName: strings.Repeat("N", MaxMerchantName+1),
		MerchantCity: strings.Repeat("C", MaxMerchantCity+1),
		PostalCode:   "123456789", Amount: "25.90", TxID: "VENDA123",
	}
	payload, err := BuildDynamicQR(params)
	require.NoError(t, err)
	assert.Empty(t, Validate(payload))
	data, err := Decode(payload)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("N", MaxMerchantName), data.MerchantName)
	assert.Equal(t, strings.Repeat("C", MaxMerchantCity), data.MerchantCity)
	assert.Equal(t, "12345678", data.PostalCode)
	assert.Equal(t, "25.90", data.TransactionAmount)
	require.NotNil(t, data.AdditionalData)
	assert.Equal(t, "VENDA123", data.AdditionalData.ReferenceLabel)

	params.PostalCode, params.Amount, params.TxID = "", "", ""
	payload, err = BuildDynamicQR(params)
	require.NoError(t, err)
	objects, err := DecodeTLV(payload)
	require.NoError(t, err)
	assert.Nil(t, FindTag(objects, TagPostalCode))
	assert.Nil(t, FindTag(objects, TagTransactionAmount))
	data, err = Decode(payload)
	require.NoError(t, err)
	require.NotNil(t, data.AdditionalData)
	assert.Equal(t, "***", data.AdditionalData.ReferenceLabel)
}

func TestValidate_InvalidDynamicPayload(t *testing.T) {
	payload, err := BuildDynamicQR(DynamicQRParams{
		URL: "pix.example.com/pay/123", MerchantName: "Name", MerchantCity: "City",
	})
	require.NoError(t, err)
	objects, err := DecodeTLV(payload)
	require.NoError(t, err)
	for _, tag := range []string{TagPayloadFormatIndicator, TagMerchantAccountInfo,
		TagMerchantCategoryCode, TagTransactionCurrency, TagCountryCode, TagMerchantName, TagMerchantCity} {
		t.Run("missing_"+tag, func(t *testing.T) {
			var b strings.Builder
			for _, obj := range objects {
				if obj.Tag != tag && obj.Tag != TagCRC {
					b.WriteString(EncodeTLV(obj.Tag, obj.Value))
				}
			}
			b.WriteString("6304")
			invalid := b.String() + fmt.Sprintf("%04X", CalculateCRC16(b.String()))
			errs := Validate(invalid)
			require.NotEmpty(t, errs)
			assert.Equal(t, "tag"+tag+map[string]string{TagMerchantAccountInfo: ".00"}[tag], errs[0].Field)
		})
	}
	assert.NotEmpty(t, Validate(payload[:len(payload)-4]+"FFFF"))
	assert.NotEmpty(t, Validate("00"))
	assert.NotEmpty(t, Validate("00XX016304FFFF"))
}
