package domain

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"strings"
)

var (
	InvalidHexString = errors.New("invalid hex string")
	InvalidDegree    = errors.New("invalid graph degree")
	InvalidIDBits    = errors.New("invalid ID bits")
)

// Big Endian
type ID []byte

// Space rappresenta lo spazio degli ID con b bit (0..2^b-1).
type Space struct {
	Bits       int
	ByteLen    int
	GraphGrade int
}

// NewSpace crea uno spazio di identificatori con b bit.
func NewSpace(b int, degree int) (*Space, error) {
	if b <= 0 {
		return nil, fmt.Errorf("invalid ID bits: %d", b)
	}
	if degree < 2 {
		return nil, fmt.Errorf("invalid graph degree: %d", degree)
	}
	return &Space{
		Bits:       b,
		ByteLen:    (b + 7) / 8,
		GraphGrade: degree,
	}, nil
}

// NewIdFromAddr genera un ID a partire da ip:port nello spazio definito da sp.
// L'ID è ottenuto tramite SHA-1, troncato/paddato a sp.Bits.
func (sp *Space) NewIdFromAddr(addr string) ID {
	h := sha1.Sum([]byte(addr)) // [20]byte (160 bit)
	// Alloca buffer della lunghezza corretta
	buf := make([]byte, sp.ByteLen)
	// Copia i byte più significativi (big-endian)
	copy(buf, h[:sp.ByteLen])
	// Maschera eventuali bit extra nell'ultimo byte (se Bits non è multiplo di 8)
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		buf[0] &= mask
	}
	return buf
}

// ToHexString restituisce l'id trasformato in una stringa esadecimale
func (x ID) Hex() string {
	if x == nil {
		return "<nil>"
	}
	return hex.EncodeToString(x)
}

// String implementa fmt.Stringer con prefisso 0x.
func (x ID) String() string {
	if x == nil {
		return "<nil>"
	}
	return "0x" + hex.EncodeToString(x)
}

// FromHexString trasforma una stringa esadecimale in un ID nello spazio sp.
// La stringa può opzionalmente iniziare con 0x o 0X.
// Se rappresenta un numero più grande di 2^sp.Bits-1, vengono presi gli ultimi sp.Bits.
// Se più corta, viene fatto il padding a sinistra.
// Se vuota o invalida, restituisce InvalidHexString.
func (sp *Space) FromHexString(s string) (ID, error) {
	str := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if str == "" {
		return nil, InvalidHexString
	}
	// decode
	bt, err := hex.DecodeString(str)
	if err != nil {
		return nil, InvalidHexString
	}
	id := make(ID, sp.ByteLen)
	if len(bt) >= sp.ByteLen {
		// prendi gli ultimi sp.ByteLen byte (meno significativi)
		copy(id, bt[len(bt)-sp.ByteLen:])
	} else {
		// padding a sinistra
		copy(id[sp.ByteLen-len(bt):], bt)
	}
	// maschera bit extra (se Bits non è multiplo di 8, big-endian)
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		id[0] &= mask
	}
	return id, nil
}

// Cmp confronta due ID come big-endian.
// Restituisce:
//
//	-1 se a < b
//	 0 se a == b
//	+1 se a > b
func (x ID) Cmp(b ID) int {
	return bytes.Compare(x, b)
}

// Equal restituisce true se due ID sono identici byte per byte.
func (x ID) Equal(b ID) bool {
	return bytes.Equal(x, b)
}

// Between restituisce true se x ∈ (a, b] nello spazio circolare degli ID.
// Gli ID sono confrontati come big-endian (Cmp).
// se a == b, l'intervallo è considerato l'intero anello e quindi ritorna sempre true.
func (x ID) Between(a, b ID) bool {
	acmp := a.Cmp(x)  // confronto a vs x
	xbcmp := x.Cmp(b) // confronto x vs b
	abcmp := a.Cmp(b) // confronto a vs b
	if abcmp == 0 {
		// Intervallo (a, a] = tutto l'anello
		return true
	}
	if abcmp < 0 {
		// Caso lineare: a < b → (a, b]
		return acmp < 0 && xbcmp <= 0
	}
	// Caso wrap-around: a > b
	return acmp < 0 || xbcmp <= 0
}

// MulKMod calcola (GraphGrade * a) mod 2^Bits.
// a è un ID big-endian della lunghezza fissa sp.ByteLen.
func (sp *Space) MulKMod(a ID) ID {
	if len(a) != sp.ByteLen {
		panic("MulKMod: ID length mismatch") //TODO: possiamo toglierlo
	}
	res := make(ID, sp.ByteLen)
	carry := uint64(0)
	k := uint64(sp.GraphGrade)
	for i := sp.ByteLen - 1; i >= 0; i-- {
		prod := uint64(a[i])*k + carry
		res[i] = byte(prod & 0xFF)
		carry = prod >> 8
	}
	// carry extra viene scartato: lavoriamo mod 2^(8*ByteLen)
	// maschera bit extra se Bits non è multiplo di 8 (big-endian)
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}
	return res
}

// AddMod calcola (a + b) mod 2^Bits.
// Entrambi gli ID devono avere lunghezza sp.ByteLen e big-endian.
func (sp *Space) AddMod(a, b ID) ID {
	if len(a) != sp.ByteLen || len(b) != sp.ByteLen {
		panic("AddMod: ID length mismatch") //TODO: possiamo toglierlo
	}
	res := make(ID, sp.ByteLen)
	carry := 0
	// somma dal byte meno significativo (a destra) verso quello più significativo
	for i := sp.ByteLen - 1; i >= 0; i-- {
		sum := int(a[i]) + int(b[i]) + carry
		res[i] = byte(sum & 0xFF)
		carry = sum >> 8
	}
	// carry extra viene scartato: risultato è modulo 2^(8*ByteLen)
	// maschera i bit extra se Bits non è multiplo di 8 (big-endian)
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}
	return res
}

// NextDigitBaseK estrae la cifra più significativa di x in base-k,
// dove k = sp.GraphGrade (deve essere una potenza di 2).
// Restituisce (digit, resto).
func (sp *Space) NextDigitBaseK(x ID) (digit uint64, rest ID) {
	if len(x) != sp.ByteLen {
		panic("NextDigitBaseK: ID length mismatch")
	}
	if (sp.GraphGrade & (sp.GraphGrade - 1)) != 0 {
		panic("NextDigitBaseK: GraphGrade deve essere una potenza di 2")
	}

	// r = log2(k)
	r := bits.TrailingZeros(uint(sp.GraphGrade))

	// 1. Converto l'ID in un big.Int
	val := new(big.Int).SetBytes(x)

	// 2. digit = primi r bit → val >> (Bits - r)
	shift := uint(sp.Bits - r)
	digitBig := new(big.Int).Rsh(val, shift)
	digit = digitBig.Uint64()

	// 3. rest = (val << r) mod 2^Bits
	restBig := new(big.Int).Lsh(val, uint(r))
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(sp.Bits))
	restBig.Mod(restBig, modulus)

	// 4. Converto restBig in []byte della lunghezza corretta
	restBytes := restBig.Bytes()
	if len(restBytes) < sp.ByteLen {
		padding := make([]byte, sp.ByteLen-len(restBytes))
		restBytes = append(padding, restBytes...)
	}
	rest = ID(restBytes)

	return digit, rest
}
