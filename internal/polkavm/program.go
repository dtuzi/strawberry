package polkavm

import (
	"bytes"
	"fmt"

	"github.com/eigerco/strawberry/pkg/serialization/codec/jam"
)

const BitmaskMax = 24

type ProgramMemorySizes struct {
	RODataSize       uint32 `jam:"length=3"`
	RWDataSize       uint32 `jam:"length=3"`
	InitialHeapPages uint16 `jam:"length=2"`
	StackSize        uint32 `jam:"length=3"`
}

type Program struct {
	ProgramMemorySizes ProgramMemorySizes
	ROData             []byte
	RWData             []byte
	CodeAndJumpTable   []byte
}

// ParseBlob let E3(|o|) ⌢ E3(|w|) ⌢ E2(z) ⌢ E3(s) ⌢ o ⌢ w ⌢ E4(|c|) ⌢ c = p (eq. A.32)
func ParseBlob(data []byte) (program *Program, err error) {
	memorySizes := ProgramMemorySizes{}
	buff := bytes.NewBuffer(data)
	dec := jam.NewDecoder(buff)
	if err := dec.Decode(&memorySizes); err != nil {
		return nil, err
	}
	program = &Program{ProgramMemorySizes: memorySizes}
	if err := dec.DecodeFixedLength(&program.ROData, uint(memorySizes.RODataSize)); err != nil {
		return nil, err
	}
	if int(memorySizes.RODataSize) != len(program.ROData) {
		return nil, fmt.Errorf("ro data size mismatch")
	}
	if int(memorySizes.RWDataSize) != len(program.RWData) {
		return nil, fmt.Errorf("rw data size mismatch")
	}
	if err := dec.DecodeFixedLength(&program.RWData, uint(memorySizes.RWDataSize)); err != nil {
		return nil, err
	}
	var codeSize uint32
	if err := dec.Decode(&codeSize); err != nil {
		return nil, err
	}
	if len(data[memorySizes.RWDataSize+4:]) != int(codeSize) {
		return nil, fmt.Errorf("code size mismatch")
	}

	program.CodeAndJumpTable = buff.Bytes()
	return program, nil
}

type CodeAndJumpTableLengths struct {
	JumpTableEntryCount uint
	JumpTableEntrySize  byte
	CodeLength          uint
}

// Deblob p = Ε(|j|) ⌢ E1(z) ⌢ E(|c|) ⌢ E_z(j) ⌢ E(c) ⌢ E(k), |k| = |c| (A.2 v6.0.2)
func Deblob(bytecode []byte) ([]byte, jam.BitSequence, []uint64, error) {
	sizes := &CodeAndJumpTableLengths{}

	buff := bytes.NewBuffer(bytecode)
	dec := jam.NewDecoder(buff)
	// Ε(|j|) ⌢ E1(z) ⌢ E(|c|)
	if err := dec.Decode(sizes); err != nil {
		return nil, nil, nil, err
	}

	// E_z(j)
	jumpTable := make([]uint64, sizes.JumpTableEntryCount)
	for i := range jumpTable {
		if err := dec.DecodeFixedLength(&jumpTable[i], uint(sizes.JumpTableEntrySize)); err != nil {
			return nil, nil, nil, err
		}
	}
	// E(c)
	code := make([]byte, sizes.CodeLength)
	if err := dec.DecodeFixedLength(&code, sizes.CodeLength); err != nil {
		return nil, nil, nil, err
	}

	var bitmask = jam.BitSequence{}
	// E(k)
	if err := dec.DecodeFixedLength(&bitmask, uint(buff.Len())); err != nil {
		return nil, nil, nil, err
	}

	bitmask = bitmask[:sizes.CodeLength] // |k| = |c|

	return code, bitmask, jumpTable, nil
}

// Skip skip(i N) → N (A.2)
func Skip(instructionOffset uint64, bitmask []bool) uint64 {
	for i, b := range bitmask[instructionOffset+1:] {
		if i > BitmaskMax {
			return BitmaskMax
		}
		if b {
			return uint64(i)
		}
	}
	return 0
}
