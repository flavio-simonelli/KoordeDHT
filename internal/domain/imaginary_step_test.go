package domain

import (
	"testing"
)

func TestImaginaryStep(t *testing.T) {
	tests := []struct {
		name       string
		bits       int
		graphGrade int
		currentHex string // currentI in hex
		nextDigit  uint64
		wantHex    string // expected nextI in hex
	}{
		{
			name:       "8-bit base2 simple step",
			bits:       8,
			graphGrade: 2,
			currentHex: "2a", // 42 = 00101010
			nextDigit:  1,
			// (42*2 + 1) mod 256 = 85 = 0x55
			wantHex: "55",
		},
		{
			name:       "8-bit base4 step",
			bits:       8,
			graphGrade: 4,
			currentHex: "c3", // 195 = 11000011
			nextDigit:  2,
			// (195*4 + 2) mod 256 = 782 mod 256 = 14 = 0x0e
			wantHex: "0e",
		},
		{
			name:       "16-bit base4 step with overflow",
			bits:       16,
			graphGrade: 4,
			currentHex: "abcd", // 43981
			nextDigit:  3,
			// (43981*4 + 3) mod 65536 = 175927 mod 65536 = 44655 = 0xaf37
			wantHex: "af37",
		},
		{
			name:       "12-bit base4 step non byte-aligned",
			bits:       12,
			graphGrade: 4,
			currentHex: "00f0", // 0000 1111 0000 (12 bit validi, padded)
			nextDigit:  1,
			// (240*4 + 1) = 961, mod 4096 = 961 = 0x03c1
			wantHex: "03c1",
		},
		{
			name:       "16-bit base16 step",
			bits:       32,
			graphGrade: 4,
			currentHex: "c72140bf",
			nextDigit:  4,
			wantHex:    "1c850300",
			// (3348884016*16 + 0) mod 4294967296 = 536870912 = 0x20000000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := Space{
				Bits:       tt.bits,
				ByteLen:    (tt.bits + 7) / 8,
				GraphGrade: tt.graphGrade,
			}

			// Parse currentI
			current, err := sp.FromHexString(tt.currentHex)
			if err != nil {
				t.Fatalf("FromHexString(%q) failed: %v", tt.currentHex, err)
			}

			// Step 1: MulKMod
			nextI, err := sp.MulKMod(current)
			if err != nil {
				t.Fatalf("MulKMod error: %v", err)
			}

			// Step 2: AddMod(nextDigit)
			nextI, err = sp.AddMod(nextI, sp.FromUint64(tt.nextDigit))
			if err != nil {
				t.Fatalf("AddMod error: %v", err)
			}

			got := nextI.ToHexString(false)
			if got != tt.wantHex {
				t.Errorf("got %s, want %s", got, tt.wantHex)
			}
		})
	}
}
