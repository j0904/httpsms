package requests

import (
	"strings"
	"unicode/utf16"
)

const (
	minRepairLength      = 4
	minRepairTextRatio   = 0.9
	forbiddenRepairChars = "]`{}|\\~^<>"
)

// repairMisencodedContent restores an SMS body that was decoded with the wrong alphabet.
// Some aggregators put GSM/ASCII bytes on the wire while declaring UCS-2 (DCS 0x08), so the
// platform pairs those bytes into UTF-16 code units and the body arrives as CJK noise.
// It only rewrites a body whose every code unit maps onto printable ASCII and which recovers
// into text, so normal Chinese/Japanese/Korean bodies are left untouched.
func repairMisencodedContent(body string) string {
	units := utf16.Encode([]rune(body))
	if len(units) < minRepairLength {
		return body
	}

	payload := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		upper := byte(unit >> 8)
		lower := byte(unit & 0xff)
		if !isPayloadByte(upper) || !isPayloadByte(lower) {
			return body
		}
		payload = append(payload, upper, lower)
	}

	candidate := string(payload)
	if strings.ContainsAny(candidate, forbiddenRepairChars) {
		return body
	}
	if textRatio(candidate) < minRepairTextRatio {
		return body
	}
	if !hasLongRun(candidate) {
		return body
	}
	return candidate
}

func isPayloadByte(value byte) bool {
	return value >= 0x09 && value <= 0x0d || value >= 0x20 && value <= 0x7e
}

func isTextByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == ' ' || value == '\t' ||
		strings.ContainsRune(".,:;!?/'\"-/%", rune(value))
}

func textRatio(candidate string) float64 {
	var text int
	for i := 0; i < len(candidate); i++ {
		if isTextByte(candidate[i]) {
			text++
		}
	}
	return float64(text) / float64(len(candidate))
}

func hasLongRun(candidate string) bool {
	letters, digits := 0, 0
	for i := 0; i < len(candidate); i++ {
		switch value := candidate[i]; {
		case value >= 'a' && value <= 'z', value >= 'A' && value <= 'Z':
			letters++
			digits = 0
		case value >= '0' && value <= '9':
			digits++
			letters = 0
		default:
			letters = 0
			digits = 0
		}
		if letters >= 3 || digits >= 3 {
			return true
		}
	}
	return false
}
