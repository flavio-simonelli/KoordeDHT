package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
)

var (
	InvalidHexString = errors.New("invalid hex string")
	InvalidDegree    = errors.New("invalid graph degree")
)

type ID []byte

// NewIdFromAddr genera un Id a partire da ip:port nel range 2^b
func NewIdFromAddr(addr string, b int) ID {
	bytes := (b + 7) / 8
	h := sha1.Sum([]byte(addr)) // 160 bit
	buf := make([]byte, bytes)
	copy(buf, h[:bytes])
	return buf
}

// ToHexString restituisce l'id trasformato in una stringa esadecimale
func (id ID) ToHexString() string {
	return hex.EncodeToString(id[:])
}

// FromHexString trasforma una stringa esadecimale (con o senza prefisso 0x)
// in un ID lungo b bit. Se la stringa è più corta di ceil(b/4) caratteri
// restituisce InvalidHexString. Se è più lunga, vengono usati solo gli
// ultimi b bit.
func FromHexString(s string, b int) (ID, error) {
	str := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	// controllo lunghezza minima
	if str == "" || len(str) < (b+3)/4 {
		return ID{}, InvalidHexString
	}
	// decode stringa hex
	bt, err := hex.DecodeString(str)
	if err != nil {
		return ID{}, InvalidHexString
	}
	// numero di byte necessari per rappresentare b bit
	nbytes := (b + 7) / 8
	// se la stringa è troppo lunga, prendo solo gli ultimi nbytes
	if len(bt) > nbytes {
		bt = bt[len(bt)-nbytes:]
	}
	id := make(ID, nbytes)
	copy(id, bt)
	// maschera i bit extra se b non è multiplo di 8
	extraBits := nbytes*8 - b
	if extraBits > 0 {
		mask := byte(0xFF >> extraBits) // es: se b=13 → extra=3 → mask=00011111
		id[0] &= mask
	}
	return id, nil
}

// Less restituisce true se id1 < id2
func (id1 ID) Less(id2 ID) bool {
	// normalizza lunghezze
	if len(id1) < len(id2) {
		diff := len(id2) - len(id1)
		id1 = append(make([]byte, diff), id1...)
	} else if len(id2) < len(id1) {
		diff := len(id1) - len(id2)
		id2 = append(make([]byte, diff), id2...)
	}
	// confronto byte per byte
	for i := 0; i < len(id1); i++ {
		if id1[i] < id2[i] {
			return true
		}
		if id1[i] > id2[i] {
			return false
		}
	}
	return false // uguali
}

// Greater restituisce true se id1 > id2
func (id1 ID) Greater(id2 ID) bool {
	return id2.Less(id1)
}

// Equal restituisce true se id1 == id2
func (id1 ID) Equal(id2 ID) bool {
	return !id1.Less(id2) && !id1.Greater(id2)
}

// InCO restituisce true se x ∈ [a, b) modulo 2^128
func (x ID) InCO(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return (a.Equal(x) || a.Less(x)) && x.Less(b)
	}
	// caso wrap-around: intervallo da a → max e da 0 → b
	return a.Equal(x) || a.Less(x) || x.Less(b)
}

// InOC restituisce true se x ∈ (a, b] modulo 2^128
func (x ID) InOC(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return a.Less(x) && (x.Equal(b) || x.Less(b))
	}
	// caso wrap-around
	return x.Less(b) || (a.Less(x) && !x.Equal(a)) || x.Equal(b)
}

// InOO restituisce true se x ∈ (a, b) modulo 2^128
func (x ID) InOO(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return a.Less(x) && x.Less(b)
	}
	// caso wrap-around: intervallo (a → max] ∪ [0 → b)
	return a.Less(x) || x.Less(b)
}

// Next restituisce l'identificatore successivo a id modulo 2^128.
func (id ID) Next() ID {
	next := make(ID, len(id))
	copy(next, id)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 { // no overflow → stop
			break
		}
	}
	return next
}

// Prev restituisce l'identificatore precedente a id modulo 2^b.
func (id ID) Prev() ID {
	prev := make(ID, len(id))
	copy(prev, id)
	for i := len(prev) - 1; i >= 0; i-- {
		prev[i]--
		if prev[i] != 0xFF { // no borrow → stop
			break
		}
	}
	return prev
}

// DeBruijnNext calcola (k*m + digit) mod 2^b.
// m è l'ID corrente, k il grado del grafo, digit ∈ [0, k-1].
func (m ID) DeBruijnNext(k, digit, b int) (ID, error) {
	if digit < 0 || digit >= k {
		return ID{}, InvalidDegree
	}
	nbytes := (b + 7) / 8
	// Converti ID in intero grande
	val := new(big.Int).SetBytes(m)
	// Calcola next = (k*m + digit) mod 2^b
	mod := new(big.Int).Lsh(big.NewInt(1), uint(b)) // 2^b
	next := new(big.Int).Mul(val, big.NewInt(int64(k)))
	next.Add(next, big.NewInt(int64(digit)))
	next.Mod(next, mod)
	// Rendi next come slice di byte lungo nbytes
	res := make(ID, nbytes)
	nb := next.Bytes()
	copy(res[nbytes-len(nb):], nb)
	return res, nil
}

// Bytes restituisce una copia del contenuto dell'ID come slice di byte.
// La copia garantisce che le modifiche allo slice risultante non alterino
// l'ID originale.
func (id ID) Bytes() []byte {
	out := make([]byte, len(id))
	copy(out, id)
	return out
}
