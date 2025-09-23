package domain

import (
	"testing"
)

func TestNextDigitBaseK(t *testing.T) {
	tests := []struct {
		name       string
		bits       int
		graphGrade int
		hexID      string
		wantDigit  uint64
		wantRest   string
	}{
		{
			name:       "k=2, 8bit",
			bits:       8,
			graphGrade: 2,
			hexID:      "0xAB", // 10101011b
			wantDigit:  1,      // bit più alto
			wantRest:   "56",   // 0x56 = 01010110b = (0xAB << 1) mod 256
		},
		{
			name:       "k=4, 8bit",
			bits:       8,
			graphGrade: 4,
			hexID:      "0xAB", // 10101011b
			wantDigit:  2,      // primi 2 bit "10"
			wantRest:   "ac",   // 0xAC = (0xAB << 2) mod 256
		},
		{
			name:       "k=8, 8bit",
			bits:       8,
			graphGrade: 8,
			hexID:      "0xAB", // 10101011b
			wantDigit:  5,      // primi 3 bit "101"
			wantRest:   "58",   // 0x58 = (0xAB << 3) mod 256
		},
		{
			name:       "k=2, 13bit",
			bits:       13,
			graphGrade: 2,
			hexID:      "0x1fff", // 8191 = 0001111111111111b (13 bit usati)
			wantDigit:  1,        // bit più alto = 1
			wantRest:   "1ffe",   // (0x1fff << 1) mod 8192
		},
		{
			name:       "k=2, 8bit",
			bits:       8,
			graphGrade: 2,
			hexID:      "0xAB", // 10101011
			wantDigit:  1,
			wantRest:   "56", // 01010110
		},
		{
			name:       "k=4, 8bit",
			bits:       8,
			graphGrade: 4,
			hexID:      "0xAB", // 10101011
			wantDigit:  2,      // "10"
			wantRest:   "ac",   // 10101100
		},
		{
			name:       "k=8, 8bit",
			bits:       8,
			graphGrade: 8,
			hexID:      "0xAB", // 10101011
			wantDigit:  5,      // "101"
			wantRest:   "58",   // 01011000
		},
		{
			name:       "k=2, 8bit, id=0",
			bits:       8,
			graphGrade: 2,
			hexID:      "0x00",
			wantDigit:  0,
			wantRest:   "00",
		},
		{
			name:       "k=2, 8bit, id=max",
			bits:       8,
			graphGrade: 2,
			hexID:      "0xff", // 255
			wantDigit:  1,      // MSB=1
			wantRest:   "fe",   // 254
		},

		// --- 13 bit ---
		{
			name:       "k=2, 13bit, id=max",
			bits:       13,
			graphGrade: 2,
			hexID:      "0x1fff", // 8191
			wantDigit:  1,
			wantRest:   "1ffe", // 8190
		},
		{
			name:       "k=2, 13bit, id=0",
			bits:       13,
			graphGrade: 2,
			hexID:      "0x0000",
			wantDigit:  0,
			wantRest:   "0000",
		},
		{
			name:       "k=2, 13bit, id=0x1000",
			bits:       13,
			graphGrade: 2,
			hexID:      "0x1000", // 4096 = 1<<12
			wantDigit:  1,        // MSB=1
			wantRest:   "0000",   // shift left 1 = 8192 -> 0 mod 8192
		},
		{
			name:       "k=4, 13bit, id=0x1000",
			bits:       13,
			graphGrade: 4,
			hexID:      "0x1000", // 4096 = 1<<12
			wantDigit:  2,        // primi 2 bit "10"
			wantRest:   "0000",   // shift left 2 = 16384 -> 0 mod 8192
		},

		// --- 16 bit ---
		{
			name:       "k=2, 16bit, id=0x8000",
			bits:       16,
			graphGrade: 2,
			hexID:      "0x8000", // 1000 0000 0000 0000
			wantDigit:  1,
			wantRest:   "0000", // shift left 1 = 0 mod 2^16
		},
		{
			name:       "k=4, 16bit, id=0xc000",
			bits:       16,
			graphGrade: 4,
			hexID:      "0xc000", // 1100 0000 0000 0000
			wantDigit:  3,        // primi 2 bit = 11
			wantRest:   "0000",   // shift left 2 = 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, _ := NewSpace(tt.bits, tt.graphGrade)
			id, err := sp.FromHexString(tt.hexID)
			if err != nil {
				t.Fatalf("FromHexString failed: %v", err)
			}
			digit, rest := sp.NextDigitBaseK(id)

			if digit != tt.wantDigit {
				t.Errorf("digit = %d, want %d", digit, tt.wantDigit)
			}
			if rest.Hex() != tt.wantRest {
				t.Errorf("rest = %s, want %s", rest.Hex(), tt.wantRest)
			}
		})
	}
}
