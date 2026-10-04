package requests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepairMisencodedContent_AsciiPayloadDeclaredAsUCS2(t *testing.T) {
	garbled := "〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮"
	assert.Equal(t, "026-10-04 16:59:44 has/have been", repairMisencodedContent(garbled))
}

func TestRepairMisencodedContent_KeepsChineseBody(t *testing.T) {
	body := "祝你，2026顺顺顺，发大财"
	assert.Equal(t, body, repairMisencodedContent(body))
}

func TestRepairMisencodedContent_KeepsShortChineseBody(t *testing.T) {
	body := "好的"
	assert.Equal(t, body, repairMisencodedContent(body))
}

func TestRepairMisencodedContent_KeepsPlainASCIIBody(t *testing.T) {
	body := "hello world"
	assert.Equal(t, body, repairMisencodedContent(body))
}

func TestRepairMisencodedContent_KeepsJapaneseBody(t *testing.T) {
	body := "あいうえおかきくけこ"
	assert.Equal(t, body, repairMisencodedContent(body))
}

func TestRepairMisencodedContent_KeepsBodyWithEmoji(t *testing.T) {
	body := "祝你 🎉 愿新一年好事连连"
	assert.Equal(t, body, repairMisencodedContent(body))
}

func TestRepairMisencodedContent_KeepsShortBody(t *testing.T) {
	assert.Equal(t, "", repairMisencodedContent(""))
	assert.Equal(t, "ij", repairMisencodedContent("ij"))
}

func TestRepairMisencodedContent_IsIdempotent(t *testing.T) {
	repaired := repairMisencodedContent("〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮")
	assert.Equal(t, repaired, repairMisencodedContent(repaired))
}

func TestMessageReceiveSanitize_RepairsMisencodedContent(t *testing.T) {
	input := MessageReceive{Content: "〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮"}
	assert.Equal(t, "026-10-04 16:59:44 has/have been", input.Sanitize().Content)
}

func TestMessageReceiveSanitize_KeepsEncryptedContent(t *testing.T) {
	garbled := "〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮"
	input := MessageReceive{Content: garbled, Encrypted: true}
	assert.Equal(t, garbled, input.Sanitize().Content)
}
