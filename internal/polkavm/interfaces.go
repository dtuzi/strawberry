package polkavm

import (
	"math"
)

type Module interface {
	AddHostFunc(string, HostFunc)
	Run(symbol string, gasLimit int64, args ...uint32) (result uint32, gasRemaining int64, err error)
	Instantiate(instructionOffset uint32, gasLimit int64) Instance
}

type Instance interface {
	GetReg(Reg) uint32
	SetReg(Reg, uint32)

	NextInstruction() (Instruction, error)
	NextOffsets()
	GetInstructionOffset() uint32
	SetInstructionOffset(target uint32)
	StartBasicBlock(program *Program)

	GasRemaining() int64
	DeductGas(cost int64)

	GetMemory(mm *MemoryMap, address uint32, length int) ([]byte, error)
	SetMemory(mm *MemoryMap, address uint32, data []byte) error
	Sbrk(mm *MemoryMap, size uint32) (uint32, error)
}

type HostFunc func(instance Instance) (uint32, error)

type Mutator interface {
	Trap() error
	Fallthrough()
	Sbrk(dst Reg, size Reg)
	MoveReg(d Reg, s Reg)
	BranchEq(s1 Reg, s2 Reg, target uint32)
	BranchEqImm(s1 Reg, s2 uint32, target uint32)
	BranchNotEq(s1 Reg, s2 Reg, target uint32)
	BranchNotEqImm(s1 Reg, s2 uint32, target uint32)
	BranchLessUnsigned(s1 Reg, s2 Reg, target uint32)
	BranchLessUnsignedImm(s1 Reg, s2 uint32, target uint32)
	BranchLessSigned(s1 Reg, s2 Reg, target uint32)
	BranchLessSignedImm(s1 Reg, s2 uint32, target uint32)
	BranchGreaterOrEqualUnsigned(s1 Reg, s2 Reg, target uint32)
	BranchGreaterOrEqualUnsignedImm(s1 Reg, s2 uint32, target uint32)
	BranchGreaterOrEqualSigned(s1 Reg, s2 Reg, target uint32)
	BranchGreaterOrEqualSignedImm(s1 Reg, s2 uint32, target uint32)
	BranchLessOrEqualUnsignedImm(s1 Reg, s2 uint32, target uint32)
	BranchLessOrEqualSignedImm(s1 Reg, s2 uint32, target uint32)
	BranchGreaterUnsignedImm(s1 Reg, s2 uint32, target uint32)
	BranchGreaterSignedImm(s1 Reg, s2 uint32, target uint32)
	SetLessThanUnsignedImm(d Reg, s1 Reg, s2 uint32)
	SetLessThanSignedImm(d Reg, s1 Reg, s2 uint32)
	ShiftLogicalLeftImm(d Reg, s1 Reg, s2 uint32)
	ShiftArithmeticRightImm(d Reg, s1 Reg, s2 uint32)
	ShiftArithmeticRightImmAlt(d Reg, s1 Reg, s2 uint32)
	NegateAndAddImm(d Reg, s1 Reg, s2 uint32)
	SetGreaterThanUnsignedImm(d Reg, s1 Reg, s2 uint32)
	SetGreaterThanSignedImm(d Reg, s1 Reg, s2 uint32)
	ShiftLogicalRightImmAlt(d Reg, s1 Reg, s2 uint32)
	ShiftLogicalLeftImmAlt(d Reg, s1 Reg, s2 uint32)
	Add(d Reg, s1, s2 Reg)
	AddImm(d Reg, s1 Reg, s2 uint32)
	Sub(d Reg, s1, s2 Reg)
	And(d Reg, s1, s2 Reg)
	AndImm(d Reg, s1 Reg, s2 uint32)
	Xor(d Reg, s1, s2 Reg)
	XorImm(d Reg, s1 Reg, s2 uint32)
	Or(d Reg, s1, s2 Reg)
	OrImm(d Reg, s1 Reg, s2 uint32)
	Mul(d Reg, s1, s2 Reg)
	MulImm(d Reg, s1 Reg, s2 uint32)
	MulUpperSignedSigned(d Reg, s1, s2 Reg)
	MulUpperSignedSignedImm(d Reg, s1 Reg, s2 uint32)
	MulUpperUnsignedUnsigned(d Reg, s1, s2 Reg)
	MulUpperUnsignedUnsignedImm(d Reg, s1 Reg, s2 uint32)
	MulUpperSignedUnsigned(d Reg, s1, s2 Reg)
	SetLessThanUnsigned(d Reg, s1, s2 Reg)
	SetLessThanSigned(d Reg, s1, s2 Reg)
	ShiftLogicalLeft(d Reg, s1, s2 Reg)
	ShiftLogicalRight(d Reg, s1, s2 Reg)
	ShiftLogicalRightImm(d Reg, s1 Reg, s2 uint32)
	ShiftArithmeticRight(d Reg, s1, s2 Reg)
	DivUnsigned(d Reg, s1, s2 Reg)
	DivSigned(d Reg, s1, s2 Reg)
	RemUnsigned(d Reg, s1, s2 Reg)
	RemSigned(d Reg, s1, s2 Reg)
	CmovIfZero(d Reg, s, c Reg)
	CmovIfZeroImm(d Reg, c Reg, s uint32)
	CmovIfNotZero(d Reg, s, c Reg)
	CmovIfNotZeroImm(d Reg, c Reg, s uint32)
	Ecalli(imm uint32) HostCallResult
	StoreU8(src Reg, offset uint32) error
	StoreU16(src Reg, offset uint32) error
	StoreU32(src Reg, offset uint32) error
	StoreImmU8(offset uint32, value uint32) error
	StoreImmU16(offset uint32, value uint32) error
	StoreImmU32(offset uint32, value uint32) error
	StoreImmIndirectU8(base Reg, offset uint32, value uint32) error
	StoreImmIndirectU16(base Reg, offset uint32, value uint32) error
	StoreImmIndirectU32(base Reg, offset uint32, value uint32) error
	StoreIndirectU8(src Reg, base Reg, offset uint32) error
	StoreIndirectU16(src Reg, base Reg, offset uint32) error
	StoreIndirectU32(src Reg, base Reg, offset uint32) error
	LoadU8(dst Reg, offset uint32) error
	LoadI8(dst Reg, offset uint32) error
	LoadU16(dst Reg, offset uint32) error
	LoadI16(dst Reg, offset uint32) error
	LoadU32(dst Reg, offset uint32) error
	LoadIndirectU8(dst Reg, base Reg, offset uint32) error
	LoadIndirectI8(dst Reg, base Reg, offset uint32) error
	LoadIndirectU16(dst Reg, base Reg, offset uint32) error
	LoadIndirectI16(dst Reg, base Reg, offset uint32) error
	LoadIndirectU32(dst Reg, base Reg, offset uint32) error
	LoadImm(dst Reg, imm uint32)
	LoadImmAndJump(ra Reg, value uint32, target uint32)
	LoadImmAndJumpIndirect(ra Reg, base Reg, value, offset uint32) error
	Jump(target uint32)
	JumpIndirect(base Reg, offset uint32) error
}

type HostCallResult struct {
	Code      HostCallCode
	InnerCode HostCallInnerCode
	Msg       string
}

type HostCallCode uint32

const (
	HostCallResultNone HostCallCode = math.MaxUint32
	HostCallResultWhat HostCallCode = math.MaxUint32 - 1
	HostCallResultOob  HostCallCode = math.MaxUint32 - 2
	HostCallResultWho  HostCallCode = math.MaxUint32 - 3
	HostCallResultFull HostCallCode = math.MaxUint32 - 4
	HostCallResultCore HostCallCode = math.MaxUint32 - 5
	HostCallResultCash HostCallCode = math.MaxUint32 - 6
	HostCallResultLow  HostCallCode = math.MaxUint32 - 7
	HostCallResultHigh HostCallCode = math.MaxUint32 - 8
	HostCallResultHuh  HostCallCode = math.MaxUint32 - 9
	HostCallResultOk   HostCallCode = 0
)

type HostCallInnerCode uint32

const (
	HostCallInnerCodeHalt  HostCallInnerCode = 0
	HostCallInnerCodePanic HostCallInnerCode = 1
	HostCallInnerCodeFault HostCallInnerCode = 2
	HostCallInnerCodeHost  HostCallInnerCode = 3
)

func (r HostCallCode) String() string {
	switch r {
	case HostCallResultNone:
		return "item does not exist"
	case HostCallResultWhat:
		return "name unknown"
	case HostCallResultOob:
		return "the return value for memory index provided is not accessible"
	case HostCallResultWho:
		return "index unknown"
	case HostCallResultFull:
		return "storage full"
	case HostCallResultCore:
		return "core index unknown"
	case HostCallResultCash:
		return "insufficient funds"
	case HostCallResultLow:
		return "gas limit too low"
	case HostCallResultHigh:
		return "gas limit too high"
	case HostCallResultHuh:
		return "the item is already solicited or cannot be forgotten"
	case HostCallResultOk:
		return "success"
	}
	return "unknown"
}
