package data

import "testing"

func TestDecodeSponsorInfoRejectsMissingKeyAndCorruptPayloadWithoutPanicking(t *testing.T) {
	tests := []struct {
		name string
		code string
		key  string
	}{
		{name: "missing key", code: "0011", key: ""},
		{name: "invalid key size", code: "0011", key: "00"},
		{name: "corrupt encrypted payload", code: "00112233445566778899aabbccddeeff", key: "00112233445566778899aabbccddeeff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeSponsorInfo(tt.code, tt.key); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}
