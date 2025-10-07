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

// Common errors related to domain identifiers.
var (
	ErrInvalidID = errors.New("invalid id")
)

// -------------------------------
// Space
// -------------------------------

// Space defines the identifier space and routing parameters
// of the Koorde Distributed Hash Table (DHT).
//
// The identifier space is defined as the set of integers in the range
// [0, 2^Bits - 1]. Identifiers are stored in big-endian format using
// ByteLen bytes.
//
// Fields:
//
//   - Bits: total number of bits in the identifier space
//     (e.g., 160 for SHA-1; 8 or 32 for test deployments).
//
//   - ByteLen: number of bytes required to encode an identifier
//     of length Bits (computed as ceil(Bits / 8)).
//
//   - GraphGrade: the base k of the underlying de Bruijn graph
//     used for routing. For example, GraphGrade=2 yields a
//     binary de Bruijn graph; larger powers of two enable
//     base-k de Bruijn routing (see Koorde paper, Section 4.1).
//
//   - SuccListSize: number of successor nodes to maintain
//     for fault tolerance. Analogous to the successor list in
//     Chord (typically O(log n)); ensures correct lookups in
//     the presence of node failures.
//
// This struct centralizes the DHT's keyspace and routing
// parameters, allowing consistent reasoning about identifiers,
// encoding, and routing properties.
type Space struct {
	Bits         int // Number of bits in the identifier space
	ByteLen      int // Number of bytes needed to represent an identifier
	GraphGrade   int // Base k of the de Bruijn graph (must be a power of 2)
	SuccListSize int // Length of the successor list for fault tolerance
}

// NewSpace initializes a new identifier space for the Koorde DHT.
//
// Parameters:
//   - b: number of bits in the identifier space. Must be > 0.
//   - degree: base k of the de Bruijn graph used for routing.
//     Must be >= 2 and preferably a power of 2.
//   - succListSize: number of successors to maintain for fault tolerance.
//     Must be > 0 (commonly O(log n)).
//
// Returns:
//   - Space: a fully initialized Space instance with derived parameters.
//   - error: if one or more input parameters are invalid.
func NewSpace(b int, degree int, succListSize int) (Space, error) {
	if b <= 0 {
		return Space{}, fmt.Errorf("invalid identifier bits: %d (must be > 0)", b)
	}
	if degree < 2 {
		return Space{}, fmt.Errorf("invalid graph degree: %d (must be >= 2)", degree)
	}
	if degree&(degree-1) != 0 {
		return Space{}, fmt.Errorf("invalid graph degree: %d (must be a power of 2)", degree)
	}
	if succListSize <= 0 {
		return Space{}, fmt.Errorf("invalid successor list size: %d (must be > 0)", succListSize)
	}
	return Space{
		Bits:         b,
		ByteLen:      (b + 7) / 8,
		GraphGrade:   degree,
		SuccListSize: succListSize,
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

// Zero returns the all-zero identifier for this space.
func (sp Space) Zero() ID {
	return make(ID, sp.ByteLen)
}

// NewIdFromString derives a new identifier (ID) from the given string,
// within the current identifier space.
//
// Typical usage includes generating IDs from node addresses (host:port)
// or resource keys.
//
// The ID is produced as follows:
//  1. Compute the SHA-1 digest (160 bits) of the input string.
//  2. Copy the most significant bytes (big-endian order) into a buffer
//     of length sp.ByteLen.
//  3. If Bits is not a multiple of 8, mask the unused high-order bits
//     in the first byte so that the ID falls strictly within the range
//     [0, 2^Bits - 1].
//
// This ensures the generated ID is uniformly distributed and valid
// for the configured identifier space.
func (sp Space) NewIdFromString(s string) ID {
	// SHA-1 digest of the input
	h := sha1.Sum([]byte(s)) // returns [20]byte (160 bits)

	// allocate buffer of correct length and copy MSBs
	buf := make([]byte, sp.ByteLen)
	copy(buf, h[:sp.ByteLen])

	// mask unused bits if identifier length is not byte-aligned
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		buf[0] &= mask // apply mask on the first (most significant) byte
	}

	return buf
}

// IsValidID verifies whether the given byte slice represents
// a valid identifier in the current identifier space.
//
// A valid ID must satisfy:
//  1. Its length matches sp.ByteLen.
//  2. If Bits is not byte-aligned, the unused high-order bits
//     in the first byte must be zero (i.e., ID < 2^Bits).
//
// Returns:
//   - nil if the ID is valid.
//   - ErrInvalidID if the ID is out of range or has invalid length.
func (sp Space) IsValidID(id []byte) error {
	// Check byte length
	if len(id) != sp.ByteLen {
		return ErrInvalidID
	}

	// Check unused bits in the most significant byte
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		// Mask to isolate the unused bits
		mask := byte(0xFF << (8 - extraBits))
		if id[0]&mask != 0 {
			return ErrInvalidID
		}
	}

	return nil
}

// ToHexString returns the identifier as a lowercase hexadecimal string.
//
// Options:
//   - If prefix is true, the string is returned with "0x" prefix.
//   - If the ID is nil, the string "<nil>" is returned instead.
//
// This method is intended for debugging, logging, and user-friendly output.
func (x ID) ToHexString(prefix bool) string {
	if x == nil {
		return "<nil>"
	}
	hexStr := hex.EncodeToString(x)
	if prefix {
		return "0x" + hexStr
	}
	return hexStr
}

// ToBigInt converts the identifier into a non-negative integer.
// The ID is interpreted as a big-endian unsigned integer.
//
// Returns:
//   - *big.Int representing the numeric value of the ID.
//   - nil if the ID is nil.
func (x ID) ToBigInt() *big.Int {
	if x == nil {
		return nil
	}
	return new(big.Int).SetBytes(x)
}

// ToBinaryString returns the binary representation of the ID
// as a string of length len(x)*8. Leading zeros are preserved.
//
// If withPrefix is true, the string is returned with "0b" prefix.
// If x is nil, the string "<nil>" is returned instead.
func (x ID) ToBinaryString(withPrefix bool) string {
	if x == nil {
		return "<nil>"
	}

	var sb strings.Builder
	for _, b := range x {
		sb.WriteString(fmt.Sprintf("%08b", b))
	}

	if withPrefix {
		return "0b" + sb.String()
	}
	return sb.String()
}

// FromHexString parses a hexadecimal string into an ID, accepting
// leading zero padding but rejecting any value that exceeds the
// current identifier space (i.e., value >= 2^Bits).
//
// Rules:
//   - The input may optionally start with "0x" or "0X".
//   - If the decoded value has more bytes than ByteLen, all the extra
//     most-significant bytes must be zero (pure padding). Otherwise: error.
//   - If shorter, it's left-padded with zeros (valid).
//   - If Bits is not a multiple of 8, the unused high-order bits in the
//     first byte must be zero; if any of those bits is 1: error.
//   - Empty or non-hex input: error.
func (sp Space) FromHexString(s string) (ID, error) {
	str := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if str == "" {
		return nil, fmt.Errorf("invalid hex string: empty input")
	}

	// Decode the hex string.
	bt, err := hex.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string %q: %w", s, err)
	}

	// If longer than ByteLen, ensure extra leading bytes are all zero (pure padding).
	if len(bt) > sp.ByteLen {
		leading := bt[:len(bt)-sp.ByteLen]
		for _, b := range leading {
			if b != 0 {
				return nil, fmt.Errorf("value exceeds %d-bit space (non-zero leading bytes)", sp.Bits)
			}
		}
		// Keep least significant sp.ByteLen bytes.
		bt = bt[len(bt)-sp.ByteLen:]
	}

	// Prepare the ID buffer with left padding if needed.
	id := make(ID, sp.ByteLen)
	copy(id[sp.ByteLen-len(bt):], bt)

	// If Bits is not byte-aligned, the high-order "extraBits" in the first byte must be zero.
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		topMask := byte(0xFF << (8 - extraBits)) // bits that are *outside* the allowed range
		if id[0]&topMask != 0 {
			return nil, fmt.Errorf("value exceeds %d-bit space (non-zero in top %d unused bits)", sp.Bits, extraBits)
		}
	}

	return id, nil
}

// FromUint64 converts a uint64 value into an identifier (ID)
// in the current identifier space.
//
// Behavior:
//   - The value is truncated to fit into sp.Bits bits
//     (i.e., only the least significant sp.Bits are kept).
//   - The result is returned as a big-endian byte slice of length sp.ByteLen.
//   - If Bits is not a multiple of 8, unused high-order bits in the
//     first byte are masked to zero.
//
// Typical usage: embedding small integers into the identifier space,
// e.g., when computing de Bruijn digits (NextDigitBaseK) or applying
// modular arithmetic on IDs (AddMod, MulKMod, etc.).
func (sp Space) FromUint64(x uint64) ID {
	id := make(ID, sp.ByteLen)

	// Fill buffer from least significant byte, big-endian order
	for i := sp.ByteLen - 1; i >= 0 && x > 0; i-- {
		id[i] = byte(x & 0xFF)
		x >>= 8
	}

	// Mask unused high-order bits if identifier is not byte-aligned
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		id[0] &= mask
	}

	return id
}

// Cmp compares two identifiers in big-endian order.
//
// Returns:
//
//	-1 if x < b
//	 0 if x == b
//	+1 if x > b
//
// Note: comparison is purely byte-wise (big-endian), so IDs are
// treated as unsigned integers in the identifier space.
func (x ID) Cmp(b ID) int {
	return bytes.Compare(x, b)
}

// Equal reports whether two identifiers are exactly the same,
// comparing all bytes.
//
// Returns true if x and b have identical length and contents.
func (x ID) Equal(b ID) bool {
	return bytes.Equal(x, b)
}

// Between reports whether the identifier x lies in the circular interval (a, b].
//
// Identifiers are compared in big-endian order using Cmp, so they are
// treated as unsigned integers in the identifier space of size 2^Bits.
//
// Interval semantics:
//   - If a == b: the interval (a, a] covers the entire ring, so the
//     method always returns true.
//   - If a < b: the interval is linear (a, b], i.e. strictly greater
//     than a and less than or equal to b.
//   - If a > b: the interval wraps around zero and includes all IDs
//     greater than a or less than or equal to b.
func (x ID) Between(a, b ID) bool {
	// Precompute comparisons
	acmp := a.Cmp(x)  // a vs x
	xbcmp := x.Cmp(b) // x vs b
	abcmp := a.Cmp(b) // a vs b

	if abcmp == 0 {
		// (a, a] means the whole ring
		return true
	}
	if abcmp < 0 {
		// Linear case: a < b → (a, b]
		return acmp < 0 && xbcmp <= 0
	}
	// Wrap-around case: a > b
	return acmp < 0 || xbcmp <= 0
}

// MulKMod computes (GraphGrade * a) modulo 2^Bits,
// where GraphGrade is the base k of the de Bruijn graph.
//
// Behavior:
//   - The input ID `a` must be a valid identifier of length sp.ByteLen.
//   - Multiplication is performed in big-endian order using
//     per-byte arithmetic with carry propagation.
//   - Any overflow beyond sp.Bits is discarded (modular arithmetic).
//   - If Bits is not a multiple of 8, the unused high-order bits
//     of the most significant byte are masked to zero.
//
// Panics if the length of `a` is not sp.ByteLen.
func (sp Space) MulKMod(a ID) (ID, error) {
	if err := sp.IsValidID(a); err != nil {
		return nil, err
	}
	res := make(ID, sp.ByteLen)
	carry := uint64(0)
	k := uint64(sp.GraphGrade)
	// Multiply each byte (big-endian order)
	for i := sp.ByteLen - 1; i >= 0; i-- {
		prod := uint64(a[i])*k + carry
		res[i] = byte(prod & 0xFF)
		carry = prod >> 8
	}
	// Apply mask if identifier size is not byte-aligned
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}
	// carry is ignored (mod 2^Bits)
	return res, nil
}

// AddMod computes (a + b) modulo 2^Bits.
//
// Both inputs must be valid IDs of length sp.ByteLen, interpreted
// as big-endian unsigned integers. Addition is performed with
// per-byte carry propagation.
//
// Behavior:
//   - Overflow beyond sp.Bits is discarded (arithmetic modulo 2^Bits).
//   - If Bits is not a multiple of 8, the unused high-order bits in
//     the most significant byte are masked to zero.
//   - Returns an error if either input is not a valid ID.
func (sp Space) AddMod(a, b ID) (ID, error) {
	if err := sp.IsValidID(a); err != nil {
		return nil, fmt.Errorf("invalid ID a: %w", err)
	}
	if err := sp.IsValidID(b); err != nil {
		return nil, fmt.Errorf("invalid ID b: %w", err)
	}

	res := make(ID, sp.ByteLen)
	carry := 0

	// Add from least significant to most significant byte
	for i := sp.ByteLen - 1; i >= 0; i-- {
		sum := int(a[i]) + int(b[i]) + carry
		res[i] = byte(sum & 0xFF)
		carry = sum >> 8
	}

	// Mask unused bits if identifier size is not byte-aligned
	extraBits := sp.ByteLen*8 - sp.Bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		res[0] &= mask
	}

	return res, nil
}

// NextDigitBaseK extracts the most significant digit of x in base-k,
// where k = sp.GraphGrade (must be a power of 2).
//
// Behavior:
//  1. Compute how many padding bits are in the first byte (extraBits).
//  2. Let r = log2(k). Extract the r most significant bits of x
//     (after skipping extraBits).
//  3. Left-shift the entire ID by r bits.
//  4. Mask the extraBits in the most significant byte to ensure
//     the result lies strictly in [0, 2^Bits).
//
// Returns:
//   - digit: the extracted base-k digit as uint64.
//   - rest:  the remaining ID after shifting left by r bits.
//   - error: if x is invalid or GraphGrade is not a power of 2.
func (sp Space) NextDigitBaseK(x ID) (digit uint64, rest ID, err error) {
	if err := sp.IsValidID(x); err != nil {
		return 0, nil, fmt.Errorf("NextDigitBaseK: invalid ID: %w", err)
	}

	// Number of unused MSBs in the first byte
	extraBits := sp.ByteLen*8 - sp.Bits

	// r = log2(k), i.e. number of bits per digit
	r := bits.TrailingZeros(uint(sp.GraphGrade))

	// extract the most significant r bits
	bitPos := extraBits // skip the padding bits
	digit = 0
	for i := 0; i < r; i++ {
		byteIndex := (bitPos + i) / 8
		bitIndex := 7 - ((bitPos + i) % 8) // MSB-first order
		bit := (x[byteIndex] >> bitIndex) & 1
		digit = (digit << 1) | uint64(bit)
	}

	// shift left by r bits
	rest = make(ID, sp.ByteLen)
	carry := byte(0)
	for i := sp.ByteLen - 1; i >= 0; i-- {
		val := x[i]
		rest[i] = (val << r) | carry
		carry = val >> (8 - r)
	}

	// mask unused high-order bits
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits)
		rest[0] &= mask
	}

	return digit, rest, nil
}

func (sp Space) BestImaginarySimple(self, succ, target ID) (currentI, kshift ID, err error) {
	base, err := sp.AddMod(self, sp.FromUint64(1))
	if err != nil {
		return nil, nil, fmt.Errorf("BestImaginarySimple: AddMod failed: %w", err)
	}
	currentI = base
	kshift = target
	return currentI, kshift, nil
}
