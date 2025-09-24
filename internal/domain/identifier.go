package domain

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/bits"
	"strings"
)

// -------------------------------
// Space
// -------------------------------

// Space represents the identifier space of the Koorde DHT.
//
// The identifier space is defined as the set of all integers in the range
// [0, 2^b - 1], where b = Bits. Each identifier is encoded in big-endian
// format using ByteLen bytes.
//
// Fields:
//   - Bits: the number of bits in the identifier space (e.g., 160 for SHA-1).
//   - ByteLen: the number of bytes required to store an identifier of Bits length.
//   - GraphGrade: the base k of the underlying de Bruijn graph used for routing.
//     For example, GraphGrade=2 yields a binary de Bruijn graph, while larger
//     values allow base-k de Bruijn routing (see Koorde paper, Section 4.1).
//
// This struct provides the parameters required to reason about identifiers,
// their encoding, and the routing properties of the Koorde DHT.
type Space struct {
	Bits       int
	ByteLen    int
	GraphGrade int
}

// NewSpace creates a new identifier space for the Koorde DHT.
//
// The identifier space consists of all integers in the range [0, 2^b - 1],
// where b is the number of bits. Identifiers are stored in big-endian
// format using (b+7)/8 bytes.
//
// Arguments:
//   - b: number of bits of the identifier space. Must be > 0.
//   - degree: base k of the de Bruijn graph used for routing. Must be >= 2.
//
// Returns:
//   - Space: a new Space instance with computed parameters.
//   - error: non-nil if the parameters are invalid.
func NewSpace(b int, degree int) (Space, error) {
	if b <= 0 {
		return Space{}, fmt.Errorf("invalid ID bits: %d", b)
	}
	if degree < 2 {
		return Space{}, fmt.Errorf("invalid graph degree: %d", degree)
	}
	return Space{
		Bits:       b,
		ByteLen:    (b + 7) / 8,
		GraphGrade: degree,
	}, nil
}

// -------------------------------
// ID type and methods
// -------------------------------

// ID represents a unique identifier in the Koorde DHT.
//
// Identifiers are stored as a byte slice in **big-endian** format,
// meaning the most significant byte is at the lowest memory index.
// This choice ensures consistent ordering when comparing IDs as
// numbers, and aligns with the arithmetic operations described
// in the Koorde and Chord papers (e.g., successor and de Bruijn
// graph calculations).
type ID []byte

// NewIdFromAddr generates a new identifier (ID) for a given network address
// (e.g., "ip:port") within the current identifier space.
//
// The ID is derived by computing the SHA-1 hash of the address string.
// The resulting 160-bit digest is then truncated or padded to match
// the bit-length specified by the Space.
//
// Steps:
//  1. Compute SHA-1 of the input address.
//  2. Copy the most significant bytes (big-endian order) into a buffer
//     of length sp.ByteLen.
//  3. If Bits is not a multiple of 8, mask the unused high-order bits
//     in the first byte to ensure the ID fits exactly into [0, 2^Bits - 1].
func (sp Space) NewIdFromAddr(addr string) ID {
	h := sha1.Sum([]byte(addr)) // [20]byte (160 bits)
	// Allocate buffer of correct length
	buf := make([]byte, sp.ByteLen)
	// Copy the most significant bytes (big-endian)
	copy(buf, h[:sp.ByteLen])
	// Mask unused high-order bits if Bits is not a multiple of 8
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		buf[0] &= mask // max one byte to mask
	}
	return buf
}

// Hex returns the identifier as a lowercase hexadecimal string.
//
// If the ID is nil, the string "<nil>" is returned instead. This is
// mainly useful for debugging and logging purposes.
func (x ID) Hex() string {
	if x == nil {
		return "<nil>"
	}
	return hex.EncodeToString(x)
}

// String implements fmt.Stringer with 0x prefix.
func (x ID) String() string {
	if x == nil {
		return "<nil>"
	}
	return "0x" + hex.EncodeToString(x)
}

// FromHexString parses a hexadecimal string into an ID within the given Space.
//
// The input string may optionally start with "0x" or "0X". The resulting ID
// is normalized to exactly sp.Bits bits, stored in big-endian format.
//
// Rules:
//   - If the hex string encodes more than sp.Bits, only the least significant
//     sp.Bits are kept (rightmost bytes).
//   - If shorter than sp.Bits, the value is left-padded with zeros.
//   - If empty or invalid, an error is returned.
//   - If Bits is not a multiple of 8, unused high-order bits in the first byte
//     are masked off.
func (sp Space) FromHexString(s string) (ID, error) {
	str := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if str == "" {
		return nil, fmt.Errorf("invalid hex string: empty")
	}
	// Decode hex string into raw bytes
	bt, err := hex.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %s", str)
	}
	id := make(ID, sp.ByteLen)
	if len(bt) >= sp.ByteLen {
		// Keep only the least significant sp.ByteLen bytes
		copy(id, bt[len(bt)-sp.ByteLen:])
	} else {
		// Left-pad with zeros
		copy(id[sp.ByteLen-len(bt):], bt)
	}
	// Mask off unused high-order bits (if Bits is not multiple of 8)
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		id[0] &= mask
	}
	return id, nil
}

// FromUint64 converts a uint64 value into an identifier (ID) within the given Space.
//
// The returned ID has exactly sp.ByteLen bytes and is interpreted as a big-endian
// integer. The conversion masks off any unused high-order bits if sp.Bits is not
// a multiple of 8, ensuring that the ID is always a valid element of the identifier
// space defined by sp.
//
// This function is typically used when a numeric digit (e.g., from NextDigitBaseK)
// must be combined with other IDs through modular arithmetic (AddMod, MulKMod, etc.).
func (sp Space) FromUint64(x uint64) ID {
	id := make(ID, sp.ByteLen)
	for i := sp.ByteLen - 1; i >= 0 && x > 0; i-- {
		id[i] = byte(x & 0xFF)
		x >>= 8
	}
	// mask unused high-order bits
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		id[0] &= mask
	}
	return id
}

// Cmp compare two ID in big-endian order.
// returns:
//
//	-1 se a < b
//	 0 se a == b
//	+1 se a > b
func (x ID) Cmp(b ID) int {
	return bytes.Compare(x, b)
}

// Equal returns true if the two ID are the same byte to byte.
func (x ID) Equal(b ID) bool {
	return bytes.Equal(x, b)
}

// Between reports whether the identifier x lies in the circular interval (a, b].
//
// Identifiers are compared in big-endian order using the Cmp method.
// The interval is defined on the identifier ring of size 2^Bits, so it
// correctly handles wrap-around cases.
//
// Rules:
//   - If a == b, the interval (a, a] is interpreted as the entire ring,
//     and the method always returns true.
//   - If a < b, the interval is linear: (a, b].
//   - If a > b, the interval wraps around zero and includes all IDs
//     greater than a or less than or equal to b.
func (x ID) Between(a, b ID) bool {
	acmp := a.Cmp(x)  // compare a vs x
	xbcmp := x.Cmp(b) // compare x vs b
	abcmp := a.Cmp(b) // compare a vs b
	if abcmp == 0 {
		// Interval (a, a] = full ring
		return true
	}
	if abcmp < 0 {
		// Linear case: a < b → (a, b]
		return acmp < 0 && xbcmp <= 0
	}
	// Wrap-around case: a > b
	return acmp < 0 || xbcmp <= 0
}

// MulKMod computes (GraphGrade * a) modulo 2^Bits, where GraphGrade is the
// base k of the de Bruijn graph.
//
// The input ID `a` must have exactly sp.ByteLen bytes and is interpreted
// as a big-endian integer. Multiplication is performed using 64-bit
// arithmetic on each byte with carry propagation. Any overflow beyond
// sp.Bits is discarded (i.e., arithmetic is performed modulo 2^Bits).
//
// If Bits is not a multiple of 8, the high-order unused bits of the most
// significant byte are masked off to ensure the result lies within the
// valid identifier space.
//
// Panics if the input ID has a length different from sp.ByteLen.
func (sp Space) MulKMod(a ID) ID {
	if len(a) != sp.ByteLen {
		panic("MulKMod: ID length mismatch") // TODO: vediamo cosa fare qua
	}
	res := make(ID, sp.ByteLen)
	carry := uint64(0)
	k := uint64(sp.GraphGrade)
	// Process bytes from least significant (end) to most significant (front)
	for i := sp.ByteLen - 1; i >= 0; i-- {
		prod := uint64(a[i])*k + carry
		res[i] = byte(prod & 0xFF)
		carry = prod >> 8
	}
	// Discard carry beyond sp.Bits (mod 2^(8*ByteLen))
	// Mask unused high-order bits if Bits is not a multiple of 8
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}
	return res
}

// AddMod computes (a + b) modulo 2^Bits.
//
// Both input IDs must have exactly sp.ByteLen bytes and are interpreted
// as big-endian integers. Addition is performed byte by byte starting
// from the least significant end, with carry propagation.
//
// Any overflow beyond sp.Bits is discarded (i.e., arithmetic is performed
// modulo 2^Bits). If Bits is not a multiple of 8, the unused high-order
// bits of the most significant byte are masked off.
//
// Panics if the input IDs have a length different from sp.ByteLen.
func (sp Space) AddMod(a, b ID) ID {
	if len(a) != sp.ByteLen || len(b) != sp.ByteLen {
		panic("AddMod: ID length mismatch") // TODO: vediamo cosa fare qua
	}
	res := make(ID, sp.ByteLen)
	carry := 0
	// Sum from least significant byte to most significant
	for i := sp.ByteLen - 1; i >= 0; i-- {
		sum := int(a[i]) + int(b[i]) + carry
		res[i] = byte(sum & 0xFF)
		carry = sum >> 8
	}
	// Discard any overflow beyond Bits
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}
	return res
}

// NextDigitBaseK extracts the most significant digit of x in base-k,
// where k = sp.GraphGrade (must be a power of 2).
//
// The operation is done entirely with bit-level arithmetic:
//  1. Compute how many padding bits are in the first byte (extraBits).
//  2. Compute x = log2(GraphGrade).
//  3. Extract the top x bits of the ID, skipping the extraBits.
//  4. Left-shift the entire ID by x bits.
//  5. Mask out the extraBits at the top so they are always zero.
//
// Returns (digit, rest).
func (sp Space) NextDigitBaseK(x ID) (digit uint64, rest ID) {
	if len(x) != sp.ByteLen {
		panic("NextDigitBaseK: ID length mismatch")
	}
	if (sp.GraphGrade & (sp.GraphGrade - 1)) != 0 {
		panic("NextDigitBaseK: GraphGrade must be a power of 2")
	}
	// Extra bits at the top (padding in the first byte)
	extraBits := sp.ByteLen*8 - sp.Bits
	// x = log2(k)
	r := bits.TrailingZeros(uint(sp.GraphGrade))
	// Extract top r bits after skipping extraBits
	bitPos := extraBits // starting bit position in the stream
	digit = 0
	for i := 0; i < r; i++ {
		byteIndex := (bitPos + i) / 8
		bitIndex := 7 - ((bitPos + i) % 8) // MSB-first
		bit := (x[byteIndex] >> bitIndex) & 1
		digit = (digit << 1) | uint64(bit)
	}
	// Shift the ID left by r bits
	rest = make(ID, sp.ByteLen)
	carry := byte(0)
	for i := sp.ByteLen - 1; i >= 0; i-- {
		val := x[i]
		rest[i] = (val << r) | carry
		carry = val >> (8 - r)
	}
	// Mask unused high-order bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		rest[0] &= mask
	}
	return digit, rest
}
