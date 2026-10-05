package brcode

// PointOfInitiation defines the EMV Point of Initiation Method.
// "11" = static (reusable), "12" = dynamic (single-use).
type PointOfInitiation string

const (
	PointOfInitiationStatic  PointOfInitiation = "11"
	PointOfInitiationDynamic PointOfInitiation = "12"
)

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
	TagUnreservedTemplate80    = "80" // First unreserved template (used for recurrence)
)

// Subtags within Merchant Account Information (tag 26-51)
const (
	SubTagGUI           = "00"
	SubTagPixKey        = "01"
	SubTagInfoAdicional = "02"
	SubTagFSS           = "03"
	SubTagURL           = "25"
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
	MaxMerchantName        = 25
	MaxMerchantCity        = 15
	MaxPostalCode          = 8
	MaxStaticTxID          = 25
	MaxDynamicTxID         = 35
	MaxURL                 = 77
	MaxInfoAdicional       = 72
	MaxMerchantAccountInfo = 99
)

// StaticQRParams contains parameters for generating a static BR Code QR.
type StaticQRParams struct {
	PixKey        string // Required: PIX key (chave)
	MerchantName  string // Required: max 25 chars
	MerchantCity  string // Required: max 15 chars
	Amount        string // Optional: decimal string "123.45" (empty = open value)
	TxID          string // Optional: max 25 chars (default "***")
	InfoAdicional string // Optional: max 72 chars
	PostalCode    string // Optional: max 8 chars (CEP)
	FSS           string // Optional: Facilitador de Servico de Saque
}

// DynamicQRParams contains parameters for generating a dynamic BR Code QR.
type DynamicQRParams struct {
	URL          string // Required: payload URL without protocol, max 77 chars
	MerchantName string // Required: max 25 chars
	MerchantCity string // Required: max 15 chars
	Amount       string // Optional: decimal string
	TxID         string // Optional: default "***"
	PostalCode   string // Optional: max 8 chars
}

// CompositeQRParams contains parameters for generating a composite BR Code QR.
// Composite QR codes combine payment (tag 26) with recurrence (tag 80).
type CompositeQRParams struct {
	// Payment component (optional — omit for recurrence-only)
	PaymentURL string // URL for dynamic payment in tag 26 (max 77 chars)
	PixKey     string // PIX key for static payment in tag 26

	// Recurrence component (required)
	RecurrenceURL string // URL for recurrence payload in tag 80 (max 77 chars)

	// Shared fields
	MerchantName string // Required: max 25 chars
	MerchantCity string // Required: max 15 chars
	Amount       string // Optional
	TxID         string // Optional
	PostalCode   string // Optional
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
	UnreservedTemplates    []UnreservedTemplate
	CRC                    string
}

// MerchantAccountInfo contains the decoded tag 26 (or 27-51) template.
type MerchantAccountInfo struct {
	GUI           string // "br.gov.bcb.pix"
	PixKey        string // Static QR: chave
	InfoAdicional string // Static QR: info adicional
	FSS           string // Facilitador de Servico de Saque
	URL           string // Dynamic QR: payload URL
}

// AdditionalData contains the decoded tag 62 template.
type AdditionalData struct {
	ReferenceLabel string // tag 62.05: txid
}

// UnreservedTemplate contains a decoded unreserved template (tags 80-99).
type UnreservedTemplate struct {
	Tag string
	GUI string // tag XX.00
	URL string // tag XX.25
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
