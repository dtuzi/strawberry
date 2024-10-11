package state

import (
	"crypto/ed25519"
	"testing"

	"github.com/eigerco/strawberry/internal/block"
	"github.com/eigerco/strawberry/internal/common"
	"github.com/eigerco/strawberry/internal/crypto"
	"github.com/eigerco/strawberry/internal/jamtime"
	"github.com/eigerco/strawberry/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateNewTimeStateTransiton(t *testing.T) {
	header := block.Header{
		TimeSlotIndex: 2,
	}
	newTimeState := calculateNewTimeState(header)
	require.Equal(t, newTimeState, header.TimeSlotIndex)
}

func TestCalculateNewEntropyPoolWhenNewEpoch(t *testing.T) {
	entropyPool := [4]crypto.Hash{
		testutils.RandomHash(t),
		testutils.RandomHash(t),
		testutils.RandomHash(t),
		testutils.RandomHash(t),
	}
	header := block.Header{
		TimeSlotIndex: 600,
	}
	newEntropyPool, err := calculateNewEntropyPool(header, jamtime.Timeslot(599), entropyPool)
	require.NoError(t, err)
	assert.Equal(t, entropyPool[2], newEntropyPool[3])
	assert.Equal(t, entropyPool[1], newEntropyPool[2])
	assert.Equal(t, entropyPool[0], newEntropyPool[1])
}

func TestCalculateNewEntropyPoolWhenNotNewEpoch(t *testing.T) {
	timeslot := jamtime.Timeslot(600)
	entropyPool := [4]crypto.Hash{
		testutils.RandomHash(t),
		testutils.RandomHash(t),
		testutils.RandomHash(t),
		testutils.RandomHash(t),
	}
	header := block.Header{
		TimeSlotIndex: 601,
	}
	newEntropyPool, err := calculateNewEntropyPool(header, timeslot, entropyPool)
	require.NoError(t, err)
	assert.Equal(t, entropyPool[3], newEntropyPool[3])
	assert.Equal(t, entropyPool[2], newEntropyPool[2])
	assert.Equal(t, entropyPool[1], newEntropyPool[1])
}
func TestCalculateNewValidatorsWhenNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	prevNextValidators := vs.SafroleState.NextValidators
	header := block.Header{
		TimeSlotIndex: 600,
	}
	newValidators, err := calculateNewValidators(header, jamtime.Timeslot(599), vs.CurrentValidators, vs.SafroleState.NextValidators)
	require.NoError(t, err)
	require.Equal(t, prevNextValidators, newValidators)
}

func TestCalculateNewValidatorsWhenNotNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	prevValidators := vs.CurrentValidators
	header := block.Header{
		TimeSlotIndex: 2,
	}
	newValidators, err := calculateNewValidators(header, jamtime.Timeslot(1), vs.CurrentValidators, vs.SafroleState.NextValidators)
	require.Error(t, err)
	require.Equal(t, prevValidators, newValidators)
}

func TestCalcualteNewArchivedValidatorsWhenNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	prevValidators := vs.CurrentValidators
	header := block.Header{
		TimeSlotIndex: 600,
	}
	newArchivedValidators, err := calculateNewArchivedValidators(header, jamtime.Timeslot(599), vs.ArchivedValidators, vs.CurrentValidators)
	require.NoError(t, err)
	require.Equal(t, prevValidators, newArchivedValidators)
}

func TestCalcualteNewArchivedValidatorsWhenNotNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	prevArchivedValidators := vs.ArchivedValidators
	header := block.Header{
		TimeSlotIndex: 2,
	}
	newArchivedValidators, err := calculateNewArchivedValidators(header, jamtime.Timeslot(1), vs.ArchivedValidators, vs.CurrentValidators)
	require.Error(t, err)
	require.Equal(t, prevArchivedValidators, newArchivedValidators)
}

func TestCaculateNewSafroleStateWhenNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	header := block.Header{
		TimeSlotIndex: 600,
	}
	tickets := block.TicketExtrinsic{}
	expected := vs.SafroleState.NextValidators
	newSafrole, err := calculateNewSafroleState(header, jamtime.Timeslot(599), tickets, expected)
	require.NoError(t, err)
	require.Equal(t, expected, newSafrole.NextValidators)
}

func TestCaculateNewSafroleStateWhenNotNewEpoch(t *testing.T) {
	vs := setupValidatorState(t)
	header := block.Header{
		TimeSlotIndex: 1,
	}
	tickets := block.TicketExtrinsic{}
	queuedValidators := vs.QueuedValidators
	_, err := calculateNewSafroleState(header, jamtime.Timeslot(0), tickets, queuedValidators)
	require.Error(t, err)
}

func TestAddUniqueHash(t *testing.T) {
    slice := []crypto.Hash{{1}, {2}, {3}}
    
    newSlice := addUniqueHash(slice, crypto.Hash{2})
    assert.Len(t, newSlice, 3, "Slice length should remain 3 when adding existing hash")

    newSlice = addUniqueHash(slice, crypto.Hash{4})
    assert.Len(t, newSlice, 4, "Slice length should be 4 after adding new hash")
    assert.Equal(t, crypto.Hash{4}, newSlice[3], "Last element should be the newly added hash")
}

func TestAddUniqueEdPubKey(t *testing.T) {
    key1 := ed25519.PublicKey([]byte{1, 2, 3})
    key2 := ed25519.PublicKey([]byte{4, 5, 6})
    slice := []ed25519.PublicKey{key1}

    newSlice := addUniqueEdPubKey(slice, key1)
    assert.Len(t, newSlice, 1, "Slice length should remain 1 when adding existing key")

    newSlice = addUniqueEdPubKey(slice, key2)
    assert.Len(t, newSlice, 2, "Slice length should be 2 after adding new key")
    assert.Equal(t, key2, newSlice[1], "Last element should be the newly added key")
}

func TestProcessVerdictGood(t *testing.T) {
    judgements := &Judgements{}
    verdict := createVerdictWithJudgments(crypto.Hash{1}, block.ValidatorsSuperMajority)
    
    processVerdict(judgements, verdict)
    
    assert.Len(t, judgements.GoodWorkReports, 1, "Should have 1 good work report")
    assert.Equal(t, crypto.Hash{1}, judgements.GoodWorkReports[0], "Good work report should have hash {1}")
    assert.Empty(t, judgements.BadWorkReports, "Should have no bad work reports")
    assert.Empty(t, judgements.WonkyWorkReports, "Should have no wonky work reports")
}

func TestProcessVerdictBad(t *testing.T) {
    judgements := &Judgements{}
    verdict := createVerdictWithJudgments(crypto.Hash{2}, 0)
    
    processVerdict(judgements, verdict)
    
    assert.Len(t, judgements.BadWorkReports, 1, "Should have 1 bad work report")
    assert.Equal(t, crypto.Hash{2}, judgements.BadWorkReports[0], "Bad work report should have hash {2}")
    assert.Empty(t, judgements.GoodWorkReports, "Should have no good work reports")
    assert.Empty(t, judgements.WonkyWorkReports, "Should have no wonky work reports")
}

func TestProcessVerdictWonky(t *testing.T) {
    judgements := &Judgements{}
    verdict := createVerdictWithJudgments(crypto.Hash{3}, common.NumberOfValidators/3)
    
    processVerdict(judgements, verdict)
    
    assert.Len(t, judgements.WonkyWorkReports, 1, "Should have 1 wonky work report")
    assert.Equal(t, crypto.Hash{3}, judgements.WonkyWorkReports[0], "Wonky work report should have hash {3}")
    assert.Empty(t, judgements.GoodWorkReports, "Should have no good work reports")
    assert.Empty(t, judgements.BadWorkReports, "Should have no bad work reports")
}

func TestProcessVerdictMultiple(t *testing.T) {
    judgements := &Judgements{}
    
    processVerdict(judgements, createVerdictWithJudgments(crypto.Hash{1}, block.ValidatorsSuperMajority))
    processVerdict(judgements, createVerdictWithJudgments(crypto.Hash{2}, 0))
    processVerdict(judgements, createVerdictWithJudgments(crypto.Hash{3}, common.NumberOfValidators/3))
    
    assert.Len(t, judgements.GoodWorkReports, 1, "Should have 1 good work report")
    assert.Len(t, judgements.BadWorkReports, 1, "Should have 1 bad work report")
    assert.Len(t, judgements.WonkyWorkReports, 1, "Should have 1 wonky work report")
}

func TestProcessOffender(t *testing.T) {
    judgements := &Judgements{}
    key := ed25519.PublicKey([]byte{1, 2, 3})

    processOffender(judgements, key)
    assert.Len(t, judgements.OffendingValidators, 1, "Should have 1 offending validator")

    processOffender(judgements, key) // Add same key again
    assert.Len(t, judgements.OffendingValidators, 1, "Should still have 1 offending validator after adding duplicate")
}

func TestCalculateNewJudgements(t *testing.T) {
    stateJudgements := Judgements{
        BadWorkReports:  []crypto.Hash{{1}},
        GoodWorkReports: []crypto.Hash{{2}},
    }

    var judgements [block.ValidatorsSuperMajority]block.Judgement
    for i := 0; i < block.ValidatorsSuperMajority; i++ {
        judgements[i] = block.Judgement{IsValid: true, ValidatorIndex: uint16(i)}
    }

    disputes := block.DisputeExtrinsic{
        Verdicts: []block.Verdict{
            {
                ReportHash: crypto.Hash{3},
                Judgements:  judgements,
            },
        },
        Culprits: []block.Culprit{
            {ValidatorEd25519PublicKey: ed25519.PublicKey([]byte{1, 2, 3})},
        },
        Faults: []block.Fault{
            {ValidatorEd25519PublicKey: ed25519.PublicKey([]byte{4, 5, 6})},
        },
    }

    newJudgements := calculateNewJudgements(disputes, stateJudgements)

    assert.Len(t, newJudgements.BadWorkReports, 1, "Should have 1 bad work report")
    assert.Len(t, newJudgements.GoodWorkReports, 2, "Should have 2 good work reports")
    assert.Len(t, newJudgements.OffendingValidators, 2, "Should have 2 offending validators")
}

func TestCalculateIntermediateBlockState(t *testing.T) {
	header := block.Header{
		PriorStateRoot: crypto.Hash{1, 2, 3},
	}

	previousRecentBlocks := []BlockState{
		{StateRoot: crypto.Hash{4, 5, 6}},
		{StateRoot: crypto.Hash{7, 8, 9}},
	}

	expectedIntermediateBlocks := []BlockState{
		{StateRoot: crypto.Hash{4, 5, 6}},
		{StateRoot: crypto.Hash{1, 2, 3}},
	}

	intermediateBlocks := calculateIntermediateBlockState(header, previousRecentBlocks)
	require.Equal(t, expectedIntermediateBlocks, intermediateBlocks)
}

func TestCalculateIntermediateBlockStateEmpty(t *testing.T) {
	header := block.Header{
		PriorStateRoot: crypto.Hash{1, 2, 3},
	}

	previousRecentBlocks := []BlockState{}

	expectedIntermediateBlocks := []BlockState{}

	intermediateBlocks := calculateIntermediateBlockState(header, previousRecentBlocks)
	require.Equal(t, expectedIntermediateBlocks, intermediateBlocks)
}

func TestCalculateIntermediateBlockStateSingleElement(t *testing.T) {
	header := block.Header{
		PriorStateRoot: crypto.Hash{1, 2, 3},
	}

	previousRecentBlocks := []BlockState{
		{StateRoot: crypto.Hash{4, 5, 6}},
	}

	expectedIntermediateBlocks := []BlockState{
		{StateRoot: crypto.Hash{1, 2, 3}},
	}

	intermediateBlocks := calculateIntermediateBlockState(header, previousRecentBlocks)
	require.Equal(t, expectedIntermediateBlocks, intermediateBlocks)
}

func TestCalculateIntermediateServiceState(t *testing.T) {
	preimageData := []byte{1, 2, 3}
	preimageHash := crypto.HashData(preimageData)
	preimageLength := PreimageLength(len(preimageData))
	newTimeslot := jamtime.Timeslot(100)

	preimages := block.PreimageExtrinsic{
		{
			ServiceIndex: 0,
			Data:         preimageData,
		},
	}

	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				{4, 5, 6}:{7, 8, 9},
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				{4, 5, 6}: {7, 8, 9},
				preimageHash:         preimageData,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
				{Hash: preimageHash, Length: preimageLength}:             {newTimeslot},
			},
		},
	}

	newServiceState := calculateIntermediateServiceState(preimages, serviceState, newTimeslot)
	require.Equal(t, expectedServiceState, newServiceState)
}

func TestCalculateIntermediateServiceStateEmptyPreimages(t *testing.T) {
	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				{4, 5, 6}:{7, 8, 9},
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := serviceState

	newServiceState := calculateIntermediateServiceState(block.PreimageExtrinsic{}, serviceState, jamtime.Timeslot(100))
	require.Equal(t, expectedServiceState, newServiceState)
}

func TestCalculateIntermediateServiceStateNonExistentService(t *testing.T) {
	preimageData := []byte{1, 2, 3}
	newTimeslot := jamtime.Timeslot(100)

	preimages := block.PreimageExtrinsic{
		{
			ServiceIndex: 1, // Non-existent service
			Data:         preimageData,
		},
	}

	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				{4, 5, 6}:{7, 8, 9},
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := serviceState

	newServiceState := calculateIntermediateServiceState(preimages, serviceState, newTimeslot)
	require.Equal(t, expectedServiceState, newServiceState)
}

func TestCalculateIntermediateServiceStateMultiplePreimages(t *testing.T) {
	preimageData1 := []byte{1, 2, 3}
	preimageData2 := []byte{4, 5, 6}
	preimageHash1 := crypto.HashData(preimageData1)
	preimageHash2 := crypto.HashData(preimageData2)
	preimageLength1 := PreimageLength(len(preimageData1))
	preimageLength2 := PreimageLength(len(preimageData2))
	newTimeslot := jamtime.Timeslot(100)

	preimages := block.PreimageExtrinsic{
		{
			ServiceIndex: 0,
			Data:         preimageData1,
		},
		{
			ServiceIndex: 0,
			Data:         preimageData2,
		},
	}

	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{},
			PreimageMeta:   map[PreImageMetaKey]PreimageHistoricalTimeslots{},
		},
	}

	expectedServiceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				preimageHash1: preimageData1,
				preimageHash2: preimageData2,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: preimageHash1, Length: preimageLength1}: {newTimeslot},
				{Hash: preimageHash2, Length: preimageLength2}: {newTimeslot},
			},
		},
	}

	newServiceState := calculateIntermediateServiceState(preimages, serviceState, newTimeslot)
	require.Equal(t, expectedServiceState, newServiceState)
}

func TestCalculateIntermediateServiceStateExistingPreimage(t *testing.T) {
	existingPreimageData := []byte{1, 2, 3}
	existingPreimageHash := crypto.HashData(existingPreimageData)
	newPreimageData := []byte{4, 5, 6}
	newTimeslot := jamtime.Timeslot(100)

	preimages := block.PreimageExtrinsic{
		{
			ServiceIndex: 0,
			Data:         existingPreimageData,
		},
		{
			ServiceIndex: 0,
			Data:         newPreimageData,
		},
	}

	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				existingPreimageHash: existingPreimageData,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: existingPreimageHash, Length: PreimageLength(len(existingPreimageData))}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				existingPreimageHash:         existingPreimageData,
				crypto.HashData(newPreimageData): newPreimageData,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: existingPreimageHash, Length: PreimageLength(len(existingPreimageData))}: {jamtime.Timeslot(50)},
				{Hash: crypto.HashData(newPreimageData), Length: PreimageLength(len(newPreimageData))}: {newTimeslot},
			},
		},
	}

	newServiceState := calculateIntermediateServiceState(preimages, serviceState, newTimeslot)
	require.Equal(t, expectedServiceState, newServiceState)
}

func TestCalculateIntermediateServiceStateExistingMetadata(t *testing.T) {
	preimageData := []byte{1, 2, 3}
	preimageHash := crypto.HashData(preimageData)
	newTimeslot := jamtime.Timeslot(100)

	preimages := block.PreimageExtrinsic{
		{
			ServiceIndex: 0,
			Data:         preimageData,
		},
	}

	serviceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: preimageHash, Length: PreimageLength(len(preimageData))}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := serviceState // Should remain unchanged

	newServiceState := calculateIntermediateServiceState(preimages, serviceState, newTimeslot)
	require.Equal(t, expectedServiceState, newServiceState)
}

func createVerdictWithJudgments(reportHash crypto.Hash, positiveJudgments int) block.Verdict {
    verdict := block.Verdict{
        ReportHash: reportHash,
        Judgements:  [block.ValidatorsSuperMajority]block.Judgement{},
    }
    for i := 0; i < positiveJudgments; i++ {
        verdict.Judgements[i] = block.Judgement{IsValid: true}
    }
    return verdict
}