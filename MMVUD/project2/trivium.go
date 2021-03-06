package main

import "fmt"

const (
	wordSize uint64 = 1 << 6       // 0000...0100 0000 = 64
	mask     uint64 = (1 << 6) - 1 // 0000...0011 1111 = 63
)

// 288-bit internal state
type Trivium struct {
	state [5]uint64 // 320-bit (but only 288 bits are used)
}

// Index returns the array index of bit i, using uint64 = 2^6 as the backing array
func (t Trivium) ArrayIdxContainsState(i uint64) uint64 {
	if i < 0 || i > 288 {
		panic("invalid access trivium index")
	}

	var index uint64 = (i - 1) >> 6

	if index > 5 {
		panic(fmt.Sprintf("index > 4. %d - %d", i, index))
	}

	return index
}

// State returns the state at index i
func (t Trivium) State(i uint64) uint64 {
	var (
		shift = uint64(mask - uint64(i-1)&mask) // number of shifts to get to the right bit
		res   uint64
	)

	res |= (t.state[t.ArrayIdxContainsState(i)] >> shift) // i'th bit

	return res
}

func (t *Trivium) NextBit() uint64 {
	var bitmask uint64 = (1 << 1) - 1 // 0000...0001
	// t1 ← s66 + s93
	t1 := t.State(66) ^ t.State(93)
	// t2 ← s162 + s177
	t2 := t.State(162) ^ t.State(177)
	// t3 ← s243 + s288
	t3 := t.State(243) ^ t.State(288)

	// zi ← t1 + t2 + t3
	z := (t1 ^ t2 ^ t3) & bitmask // stores the n last bits

	// t1 ← t1 + s91 · s92 + s171
	t1 ^= ((t.State(91) & t.State(92)) ^ t.State(171))
	t1 &= bitmask

	// t2 ← t2 + s175 · s176 + s264
	t2 ^= ((t.State(175) & t.State(176)) ^ t.State(264))
	t2 &= bitmask

	// t3 ← t3 + s286 · s287 + s69
	t3 ^= ((t.State(286) & t.State(287)) ^ t.State(69))
	t3 &= bitmask

	// shift -> (s1, s2, . . . , s93) ← (t3, s1, . . . , s92)
	t.state[4] = (t.state[4] >> 1) | (t.state[3] << (wordSize - 1))
	t.state[3] = (t.state[3] >> 1) | (t.state[2] << (wordSize - 1))
	t.state[2] = (t.state[2] >> 1) | (t.state[1] << (wordSize - 1))
	t.state[1] = (t.state[1] >> 1) | (t.state[0] << (wordSize - 1))
	t.state[0] = (t.state[0] >> 1) | (t3 << (wordSize - 1))

	// (s94, s95, . . . , s177) ← (t1, s94, . . . , s176)
	n94 := uint64(93)               // real index
	nidx94 := n94 >> 6              // array index
	nshift94 := mask - (n94 & mask) // shift to get to the 94th bit

	// insert
	t.state[nidx94] = t.state[nidx94] &^ (bitmask << nshift94) // clear
	t.state[nidx94] |= t1 << nshift94                          // insert

	// (s178, s279, . . . , s288) ← (t2, s178, . . . , s287)
	n178 := uint64(177)               // real index
	nidx178 := n178 >> 6              // array index
	nshift178 := mask - (n178 & mask) // shift to get to the 178th bit
	// insert
	t.state[nidx178] = t.state[nidx178] &^ (bitmask << nshift178) // clear
	t.state[nidx178] |= t2 << nshift178                           // insert

	return z
}

func (t *Trivium) NextByte() byte {
	var next uint8

	for i := 0; i < 8; i++ {
		nextBit := t.NextBit()
		next |= uint8(nextBit) << i
	}

	return byte(next)
}

// InitTrivium 80 bits key length and initial valuesu
func InitTrivium(key, iv [10]byte) *Trivium {
	var state [5]uint64

	// load the 80-bit key into 0 to 80 states - (s1, s2, . . . , s93) ← (K1, . . . , K80, 0, . . . , 0)
	state[0] |= (uint64((key[0])) << 56) // 1 - 8
	state[0] |= (uint64((key[1])) << 48) // 9 - 16
	state[0] |= (uint64((key[2])) << 40) // 17 - 24
	state[0] |= (uint64((key[3])) << 32) // 25 - 32

	state[0] |= (uint64((key[4])) << 24) // 33 - 40
	state[0] |= (uint64((key[5])) << 16) // 41 - 48
	state[0] |= (uint64((key[6])) << 8)  // 49 - 56
	state[0] |= (uint64((key[7])))       // 57 - 64

	state[1] |= (uint64((key[8])) << 56) // 65 - 72 (1 - 8)
	state[1] |= (uint64((key[9])) << 48) // 73 - 81 (9 - 16)

	// load the 80-bit initial value into 94 - 174 states - (s94, s95, . . . , s177) ← (IV1, . . . , IV80, 0, . . . , 0)
	state[1] |= (uint64((iv[0])) << 27) // 94 - 102 (29 - 37)
	state[1] |= (uint64((iv[1])) << 19) // 102 - 110 (37 - 45)
	state[1] |= (uint64((iv[2])) << 11) // 111 - 119 (45 - 53)
	state[1] |= (uint64((iv[3])) << 3)  // 120 - 128 (53 - 61)
	state[1] |= (uint64((iv[4])) >> 5)  // 129 - 136 (61 - 64)

	state[2] |= (uint64((iv[4])) << 59) // 137 - 141 (1 - 5)
	state[2] |= (uint64((iv[5])) << 51) // 142 - 149 (6 - 13)
	state[2] |= (uint64((iv[6])) << 43) // 149 - 156 (14 - 21)
	state[2] |= (uint64((iv[7])) << 35) // 156 - 163 (22 - 29)
	state[2] |= (uint64((iv[8])) << 27) // 164 - 171 (30 - 37)
	state[2] |= (uint64((iv[9])) << 19) // 172 - 173 (38 - 45)

	// set state 286 - 288 to 1 - (s178, s279, . . . , s288) ← (0, . . . , 0, 1, 1, 1)
	state[4] = uint64(7) << 32 // 286 - 288

	trivium := Trivium{state: state}

	// warm-up
	for i := 0; i < 4*288; i++ {
		trivium.NextBit()
	}

	return &trivium
}
