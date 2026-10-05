package brcode

import (
	"fmt"
	"strconv"
)

// EMVDataObject represents a single TLV (Tag-Length-Value) data object
// as defined by EMV-QRCPS-MPM v1.1.
type EMVDataObject struct {
	Tag      string          // 2-character tag identifier (00-99)
	Value    string          // The data value
	Children []EMVDataObject // Nested TLV objects for template tags
}

// templateTags are EMV tags that contain nested TLV structures.
// Tags 26-51: Merchant Account Information templates
// Tag 62: Additional Data Field template
// Tag 64: Merchant Information — Language template
// Tags 80-99: Unreserved Templates
var templateTags = map[string]bool{}

func init() {
	for i := 26; i <= 51; i++ {
		templateTags[fmt.Sprintf("%02d", i)] = true
	}
	templateTags["62"] = true
	templateTags["64"] = true
	for i := 80; i <= 99; i++ {
		templateTags[fmt.Sprintf("%02d", i)] = true
	}
}

// IsTemplateTag returns true if the tag contains nested TLV data objects.
func IsTemplateTag(tag string) bool {
	return templateTags[tag]
}

// EncodeTLV encodes a single tag-length-value triplet.
// Format: Tag(2) + Length(2, zero-padded) + Value
func EncodeTLV(tag, value string) string {
	return fmt.Sprintf("%s%02d%s", tag, len(value), value)
}

// EncodeTemplateTLV encodes a template tag containing nested TLV children.
// The children are encoded first, then wrapped in the parent tag.
func EncodeTemplateTLV(tag string, children []EMVDataObject) string {
	inner := ""
	for _, child := range children {
		if len(child.Children) > 0 {
			inner += EncodeTemplateTLV(child.Tag, child.Children)
		} else {
			inner += EncodeTLV(child.Tag, child.Value)
		}
	}
	return EncodeTLV(tag, inner)
}

// DecodeTLV parses an EMV TLV payload string into structured data objects.
// Template tags (26-51, 62, 64, 80-99) are recursively decoded.
func DecodeTLV(payload string) ([]EMVDataObject, error) {
	var objects []EMVDataObject
	pos := 0

	for pos < len(payload) {
		// Need at least 4 chars: tag(2) + length(2)
		if pos+4 > len(payload) {
			return nil, fmt.Errorf("malformed TLV at position %d: insufficient data for tag+length", pos)
		}

		tag := payload[pos : pos+2]
		lengthStr := payload[pos+2 : pos+4]
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return nil, fmt.Errorf("malformed TLV at position %d: invalid length %q", pos, lengthStr)
		}

		pos += 4 // advance past tag + length

		if pos+length > len(payload) {
			return nil, fmt.Errorf("malformed TLV at position %d: value length %d exceeds remaining data", pos-4, length)
		}

		value := payload[pos : pos+length]
		pos += length

		obj := EMVDataObject{Tag: tag, Value: value}

		// Recursively parse template tags
		if IsTemplateTag(tag) {
			children, err := DecodeTLV(value)
			if err != nil {
				return nil, fmt.Errorf("malformed template tag %s: %w", tag, err)
			}
			obj.Children = children
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// FindTag searches a slice of EMVDataObject for a specific tag.
// Returns nil if not found.
func FindTag(objects []EMVDataObject, tag string) *EMVDataObject {
	for i := range objects {
		if objects[i].Tag == tag {
			return &objects[i]
		}
	}
	return nil
}
