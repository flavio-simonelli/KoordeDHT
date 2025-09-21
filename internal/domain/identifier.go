package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
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
func (id ID) Less(id2 ID) bool {
	// normalizza lunghezze
	if len(id) < len(id2) {
		diff := len(id2) - len(id)
		id = append(make([]byte, diff), id...)
	} else if len(id2) < len(id) {
		diff := len(id) - len(id2)
		id2 = append(make([]byte, diff), id2...)
	}
	// confronto byte per byte
	for i := 0; i < len(id); i++ {
		if id[i] < id2[i] {
			return true
		}
		if id[i] > id2[i] {
			return false
		}
	}
	return false // uguali
}

// Greater restituisce true se id1 > id2
func (id ID) Greater(id2 ID) bool {
	return id2.Less(id)
}

// Equal restituisce true se id1 == id2
func (id ID) Equal(id2 ID) bool {
	return !id.Less(id2) && !id.Greater(id2)
}

// InCO restituisce true se x ∈ [a, b) modulo 2^128
func (id ID) InCO(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return (a.Equal(id) || a.Less(id)) && id.Less(b)
	}
	// caso wrap-around: intervallo da a → max e da 0 → b
	return a.Equal(id) || a.Less(id) || id.Less(b)
}

// InOC restituisce true se x ∈ (a, b] modulo 2^128
func (id ID) InOC(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return a.Less(id) && (id.Equal(b) || id.Less(b))
	}
	// caso wrap-around
	return id.Less(b) || (a.Less(id) && !id.Equal(a)) || id.Equal(b)
}

// InOO restituisce true se x ∈ (a, b) modulo 2^128
func (id ID) InOO(a, b ID) bool {
	if a.Less(b) {
		// caso normale: a < b
		return a.Less(id) && id.Less(b)
	}
	// caso wrap-around: intervallo (a → max] ∪ [0 → b)
	return a.Less(id) || id.Less(b)
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

// Bytes restituisce una copia del contenuto dell'ID come slice di byte.
// La copia garantisce che le modifiche allo slice risultante non alterino
// l'ID originale.
func (id ID) Bytes() []byte {
	out := make([]byte, len(id))
	copy(out, id)
	return out
}

// AdvanceDeBruijn calcola il prossimo nodo immaginario i*d
// in un grafo de Bruijn di base k (k deve essere potenza di 2).
//
// Parametri:
//   - id: identificatore corrente come slice di byte (big-endian).
//   - d: cifra da aggiungere (0 <= d < k).
//   - k: base del grafo (es. 2, 4, 16, 256).
//
// Restituisce:
//   - un nuovo identificatore ID dopo lo shift e append.
//
// Nota: la lunghezza di id rimane invariata (overflow scarta i bit più alti).
func (id ID) AdvanceDeBruijn(d, k int) ID {
	if (k & (k - 1)) != 0 {
		panic("DeBruijnStepBytes: k deve essere una potenza di 2") //TODO: chageme con logger
	}
	if d < 0 || d >= k {
		panic("DeBruijnStepBytes: cifra d fuori dall'intervallo [0, k)") //TODO: chageme con logger
	}
	shiftBits := 0
	for x := k; x > 1; x >>= 1 {
		shiftBits++
	}
	out := make([]byte, len(id))
	carry := byte(d)
	for i := len(id) - 1; i >= 0; i-- {
		// prendi i bit alti che andranno come carry al byte successivo
		newCarry := id[i] >> (8 - shiftBits)
		// shift a sinistra e aggiungi il carry corrente nei bit bassi
		out[i] = (id[i]<<shiftBits | carry) & 0xFF
		// aggiorna carry per il prossimo ciclo
		carry = newCarry
	}
	return out
}

// TopDigit estrae la cifra più significativa dell'ID
// in base k (dove k deve essere potenza di 2).
func (id ID) TopDigit(k int) (int, error) {
	if len(id) == 0 {
		return 0, fmt.Errorf("TopDigit: ID vuoto")
	}
	if (k & (k - 1)) != 0 {
		return 0, fmt.Errorf("TopDigit: k deve essere potenza di 2")
	}
	// numero di bit da estrarre
	shiftBits := 0
	for x := k; x > 1; x >>= 1 {
		shiftBits++
	}
	// prendiamo i primi shiftBits bit del primo byte
	firstByte := id[0]
	mask := byte((1 << shiftBits) - 1)
	top := (firstByte >> (8 - shiftBits)) & mask
	return int(top), nil
}

// ShiftLeftDigit ritorna un nuovo ID ottenuto spostando a sinistra di log₂(k) bit.
// Questo equivale a "consumare" la TopDigit in base-k.
// k deve essere potenza di 2.
func (id ID) ShiftLeftDigit(k int) (ID, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("ShiftLeftDigit: ID vuoto")
	}
	if (k & (k - 1)) != 0 {
		return nil, fmt.Errorf("ShiftLeftDigit: k deve essere potenza di 2")
	}
	// calcola quanti bit spostare
	shiftBits := 0
	for x := k; x > 1; x >>= 1 {
		shiftBits++
	}
	out := make(ID, len(id))
	carry := byte(0)
	for i := len(id) - 1; i >= 0; i-- {
		cur := id[i]
		// salva i bit più alti che devono scivolare nel byte successivo
		newCarry := cur >> (8 - shiftBits)
		// shift e inserisci carry dai bit meno significativi
		out[i] = (cur << shiftBits) | carry
		carry = newCarry
	}
	return out, nil
}
