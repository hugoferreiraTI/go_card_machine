package brcode

// PointOfInitiation defines the EMV Point of Initiation Method.
// "12" identifies the dynamic QR flow supported by this package.
type PointOfInitiation string

const PointOfInitiationDynamic PointOfInitiation = "12"

// --- EMV Tag Constants ---

const (
	TagPayloadFormatIndicator  = "00"
	TagPointOfInitiationMethod = "01"
	TagMerchantAccountInfo     = "26" // Default PIX MAI template (26-51 allowed)
	TagMerchantCategoryCode    = "52"
	TagTransactionCurrency     = "53"
	TagTransactionAmount       = "54"
	TagCountryCode             = "58"
	TagMerchantName            = "59"
	TagMerchantCity            = "60"
	TagPostalCode              = "61"
	TagAdditionalDataField     = "62"
	TagCRC                     = "63"
)

// Subtags within Merchant Account Information (tag 26-51)
const (
	SubTagGUI = "00"
	SubTagURL = "25"
)

// Subtags within Additional Data Field (tag 62)
const (
	SubTagReferenceLabel = "05"
)

// --- BR Code Fixed Values ---

const (
	PayloadFormatIndicator = "01"
	PIXGUI                 = "br.gov.bcb.pix"
	MerchantCategoryCode   = "0000"
	CurrencyBRL            = "986"
	CountryBR              = "BR"
)

// --- Field Length Constraints ---

const (
	MaxMerchantName = 25
	MaxMerchantCity = 15
	MaxPostalCode   = 8
	MaxDynamicTxID  = 35 // Advisory limit; BuildDynamicQR does not enforce it.
	MaxURL          = 77
)

// DynamicQRParams contains parameters for generating a dynamic BR Code QR.
type DynamicQRParams struct {
	URL          string // Required: payload URL without protocol, max 77 chars
	MerchantName string // Required: max 25 chars
	MerchantCity string // Required: max 15 chars
	Amount       string // Optional: decimal string
	TxID         string // Optional: default "***"
	PostalCode   string // Optional: max 8 chars
}

// BRCodeData represents a fully decoded BR Code payload.
type BRCodeData struct {
	PayloadFormatIndicator string
	PointOfInitiation      PointOfInitiation
	MerchantAccountInfo    *MerchantAccountInfo
	MerchantCategoryCode   string
	TransactionCurrency    string
	TransactionAmount      string
	CountryCode            string
	MerchantName           string
	MerchantCity           string	
	PostalCode             string
	AdditionalData         *AdditionalData
	CRC                    string
}

// MerchantAccountInfo contains the decoded tag 26 (or 27-51) template.
type MerchantAccountInfo struct {
	GUI string // "br.gov.bcb.pix"
	URL string // Dynamic QR: payload URL
}

// AdditionalData contains the decoded tag 62 template.
type AdditionalData struct {
	ReferenceLabel string // tag 62.05: txid
}

// ValidationError represents a BR Code validation issue.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
