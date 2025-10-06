package domain

import (
	"strings"
	"testing"
)

func TestImaginaryStep(t *testing.T) {
	tests := []struct {
		name       string
		bits       int
		graphGrade int
		kshift     string
		i          string // currentI in hex
		nextDigit  uint64
		wantRest   string
		wantHex    string // expected nextI in hex
	}{
		{
			name:       "prova sbagliato",
			bits:       66,
			graphGrade: 8,
			kshift:     "0037ef85d91755ea28",
			i:          "00fb487b807ea44256",
			nextDigit:  0,
			wantRest:   "01bf7c2ec8baaf5140",
			wantHex:    "03da43dc03f52212b0",
		},
		{
			name:       "prova sbagliato 2 ",
			bits:       66,
			graphGrade: 8,
			kshift:     "01bf7c2ec8baaf5140",
			i:          "03da43dc03f52212b0",
			nextDigit:  3,
			wantRest:   "01FBE17645D57A8A00",
			wantHex:    "02D21EE01FA9109583",
		},
		{
			name:       "prova sbagliato 3",
			bits:       66,
			graphGrade: 8,
			kshift:     "01FBE17645D57A8A00",
			i:          "02D21EE01FA9109583",
			nextDigit:  3,
			wantRest:   "03DF0BB22EABD45000",
			wantHex:    "0290F700FD4884AC1B",
		},
		{
			name:       "prova sbagliato 4-5",
			bits:       66,
			graphGrade: 8,
			kshift:     "03DF0BB22EABD45000",
			i:          "0290F700FD4884AC1B",
			nextDigit:  7,
			wantRest:   "02F85D91755EA28000",
			wantHex:    "0087B807EA442560DF",
		},
		{
			name:       "esempio con 16 bit",
			bits:       16,
			graphGrade: 8,
			kshift:     "b6c8",
			i:          "1234",
			nextDigit:  5,
			wantRest:   "b640",
			wantHex:    "91a5",
		},
		{
			name:       "esempio",
			bits:       64,
			graphGrade: 8,
			kshift:     "c2ec8baaf5140000",
			i:          "3dc03f52212b1bf7",
			nextDigit:  6,
			wantRest:   "17645d57a8a00000",
			wantHex:    "ee01fa910958dfbe",
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
			currentI, err := sp.FromHexString(tt.i)
			if err != nil {
				t.Fatalf("FromHexString(%q) failed: %v", tt.i, err)
			}
			// Parse kshift
			kshift, err := sp.FromHexString(tt.kshift)
			if err != nil {
				t.Fatalf("FromHexString(%q) failed: %v", tt.kshift, err)
			}
			// Get next digit and updated kshift
			digit, nextKshift, err := sp.NextDigitBaseK(kshift)
			if err != nil {
				t.Fatalf("FromHexString(%q) failed: %v", tt.kshift, err)
			}

			// Step 1: MulKMod
			nextI, err := sp.MulKMod(currentI)
			if err != nil {
				t.Fatalf("MulKMod error: %v", err)
			}

			// Step 2: AddMod(nextDigit)
			nextI, err = sp.AddMod(nextI, sp.FromUint64(digit))
			if err != nil {
				t.Fatalf("AddMod error: %v", err)
			}

			gotcurrentI := nextI.ToHexString(false)
			gotkshift := nextKshift.ToHexString(false)
			if gotcurrentI != strings.ToLower(tt.wantHex) {
				t.Errorf("got %s, want %s (got kshift: %s qith digit %d)", gotcurrentI, tt.wantHex, gotkshift, digit)
			}
			if gotkshift != strings.ToLower(tt.wantRest) {
				t.Errorf("got kshift %s, want %s", gotkshift, tt.wantRest)
			}
			if digit != tt.nextDigit {
				t.Errorf("got digit %d, want %d", digit, tt.nextDigit)
			}

		})
	}
}
