package state

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/eigerco/strawberry/internal/block"
	"github.com/eigerco/strawberry/internal/common"
	"github.com/eigerco/strawberry/internal/crypto"
	"github.com/eigerco/strawberry/internal/jamtime"
	"github.com/eigerco/strawberry/internal/safrole"
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
	verdict := createVerdictWithJudgments(crypto.Hash{1}, common.ValidatorsSuperMajority)

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

	processVerdict(judgements, createVerdictWithJudgments(crypto.Hash{1}, common.ValidatorsSuperMajority))
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

	var judgements [common.ValidatorsSuperMajority]block.Judgement
	for i := 0; i < common.ValidatorsSuperMajority; i++ {
		judgements[i] = block.Judgement{IsValid: true, ValidatorIndex: uint16(i)}
	}

	disputes := block.DisputeExtrinsic{
		Verdicts: []block.Verdict{
			{
				ReportHash: crypto.Hash{3},
				Judgements: judgements,
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
				{4, 5, 6}: {7, 8, 9},
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
			},
		},
	}

	expectedServiceState := ServiceState{
		block.ServiceId(0): {
			PreimageLookup: map[crypto.Hash][]byte{
				{4, 5, 6}:    {7, 8, 9},
				preimageHash: preimageData,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: crypto.Hash{4, 5, 6}, Length: PreimageLength(3)}: {jamtime.Timeslot(50)},
				{Hash: preimageHash, Length: preimageLength}:            {newTimeslot},
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
				{4, 5, 6}: {7, 8, 9},
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
				{4, 5, 6}: {7, 8, 9},
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
				existingPreimageHash:             existingPreimageData,
				crypto.HashData(newPreimageData): newPreimageData,
			},
			PreimageMeta: map[PreImageMetaKey]PreimageHistoricalTimeslots{
				{Hash: existingPreimageHash, Length: PreimageLength(len(existingPreimageData))}:        {jamtime.Timeslot(50)},
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

func TestCalculateIntermediateCoreAssignmentsFromExtrinsics(t *testing.T) {
	// Create WorkReports with known hashes
	workReport1 := &block.WorkReport{CoreIndex: 0}
	workReport2 := &block.WorkReport{CoreIndex: 1}

	hash1, _ := workReport1.Hash()
	hash2, _ := workReport2.Hash()

	coreAssignments := CoreAssignments{
		{WorkReport: workReport1},
		{WorkReport: workReport2},
	}

	disputes := block.DisputeExtrinsic{
		Verdicts: []block.Verdict{
			createVerdictWithJudgments(hash1, common.ValidatorsSuperMajority-1),
			createVerdictWithJudgments(hash2, common.ValidatorsSuperMajority),
		},
	}

	expectedAssignments := CoreAssignments{
		{WorkReport: nil}, // Cleared due to less than super majority
		{WorkReport: workReport2},
	}

	newAssignments := calculateIntermediateCoreAssignmentsFromExtrinsics(disputes, coreAssignments)
	require.Equal(t, expectedAssignments, newAssignments)
}

func TestCalculateIntermediateCoreAssignmentsFromAvailability(t *testing.T) {
	testCases := []struct {
		name           string
		availableCores uint16
		validators     uint16
		expectedKept   uint16
	}{
		{"No Cores Available, All Validators", 0, common.NumberOfValidators, 0},
		{"No Cores Available, No Validators", 0, 0, 0},
		{"Half Cores Available, No Validators", common.TotalNumberOfCores / 2, 0, 0},
		{"Half Cores Available, 2/3 Validators", common.TotalNumberOfCores / 2, (2 * common.NumberOfValidators / 3), 0},
		{"Half Cores Available, 2/3+1 Validators", common.TotalNumberOfCores / 2, (2 * common.NumberOfValidators / 3) + 1, common.TotalNumberOfCores / 2},
		{"Half Cores Available, All Validators", common.TotalNumberOfCores / 2, common.NumberOfValidators, common.TotalNumberOfCores / 2},
		{"All Cores Available, All Validators", common.TotalNumberOfCores, common.NumberOfValidators, common.TotalNumberOfCores},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assurances := createAssuranceExtrinsic(tc.availableCores, tc.validators)
			initialAssignments := createInitialAssignments()
			newAssignments := calculateIntermediateCoreAssignmentsFromAvailability(assurances, initialAssignments)

			keptCount := uint16(0)
			for _, assignment := range newAssignments {
				if assignment.WorkReport != nil {
					keptCount++
				}
			}
			require.Equal(t, tc.expectedKept, keptCount, "Number of kept assignments should match expected")
		})
	}
}

func TestCalculateNewCoreAssignments(t *testing.T) {
	t.Run("valid guarantees within rotation period", func(t *testing.T) {
		pubKey1, prvKey1, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		pubKey2, prvKey2, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		validatorKeys := make([]crypto.ValidatorKey, 2)
		validatorKeys[0].Ed25519 = pubKey1
		validatorKeys[1].Ed25519 = pubKey2
		validatorsData := safrole.ValidatorsData{
			{Ed25519: validatorKeys[0].Ed25519},
			{Ed25519: validatorKeys[1].Ed25519},
		}

		validatorState := ValidatorState{
			CurrentValidators:  validatorsData,
			ArchivedValidators: validatorsData,
		}
		currentTimeslot := jamtime.Timeslot(100)

		workReport := block.WorkReport{
			CoreIndex: 0,
		}

		// Marshal and hash the work report for signing
		reportBytes, err := json.Marshal(workReport)
		require.NoError(t, err)
		reportHash := crypto.HashData(reportBytes)
		message := append([]byte(signatureContextGuarantee), reportHash[:]...)
		signature1 := ed25519.Sign(prvKey1, message)
		signature2 := ed25519.Sign(prvKey2, message)

		// Create credentials with valid signatures
		credentials := []block.CredentialSignature{
			{
				ValidatorIndex: 0,
				Signature:      [64]byte(signature1),
			},
			{
				ValidatorIndex: 1,
				Signature:      [64]byte(signature2),
			},
		}

		guarantees := block.GuaranteesExtrinsic{
			Guarantees: []block.Guarantee{
				{
					WorkReport:  workReport,
					Timeslot:    currentTimeslot - 1,
					Credentials: credentials,
				},
			},
		}

		intermediateAssignments := CoreAssignments{}

		newAssignments := calculateNewCoreAssignments(
			guarantees,
			intermediateAssignments,
			validatorState,
			currentTimeslot,
		)

		// Assert
		require.NotNil(t, newAssignments[0].WorkReport)
		require.Equal(t, workReport, *newAssignments[0].WorkReport)
		require.Equal(t, currentTimeslot, newAssignments[0].Time)
	})

	t.Run("invalid guarantee due to timeslot too old", func(t *testing.T) {
		pubKey1, prvKey1, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		pubKey2, prvKey2, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		validatorKeys := make([]crypto.ValidatorKey, 2)
		validatorKeys[0].Ed25519 = pubKey1
		validatorKeys[1].Ed25519 = pubKey2
		validatorsData := safrole.ValidatorsData{
			{Ed25519: validatorKeys[0].Ed25519},
			{Ed25519: validatorKeys[1].Ed25519},
		}

		validatorState := ValidatorState{
			CurrentValidators:  validatorsData,
			ArchivedValidators: validatorsData,
		}
		currentTimeslot := jamtime.Timeslot(100)

		workReport := block.WorkReport{
			CoreIndex: 0,
		}

		reportBytes, err := json.Marshal(workReport)
		require.NoError(t, err)
		reportHash := crypto.HashData(reportBytes)
		message := append([]byte(signatureContextGuarantee), reportHash[:]...)
		signature1 := ed25519.Sign(prvKey1, message)
		signature2 := ed25519.Sign(prvKey2, message)

		credentials := []block.CredentialSignature{
			{
				ValidatorIndex: 0,
				Signature:      [64]byte(signature1),
			},
			{
				ValidatorIndex: 1,
				Signature:      [64]byte(signature2),
			},
		}

		// Create guarantee with timeslot outside valid range
		oldTimeslot := currentTimeslot - common.ValidatorRotationPeriod*2
		guarantees := block.GuaranteesExtrinsic{
			Guarantees: []block.Guarantee{
				{
					WorkReport:  workReport,
					Timeslot:    oldTimeslot,
					Credentials: credentials,
				},
			},
		}

		intermediateAssignments := CoreAssignments{}

		newAssignments := calculateNewCoreAssignments(
			guarantees,
			intermediateAssignments,
			validatorState,
			currentTimeslot,
		)

		require.Nil(t, newAssignments[0].WorkReport)
	})

	t.Run("invalid guarantee due to unordered credentials", func(t *testing.T) {
		pubKey1, prvKey1, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		pubKey2, prvKey2, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		validatorKeys := make([]crypto.ValidatorKey, 2)
		validatorKeys[0].Ed25519 = pubKey1
		validatorKeys[1].Ed25519 = pubKey2
		validatorsData := safrole.ValidatorsData{
			{Ed25519: validatorKeys[0].Ed25519},
			{Ed25519: validatorKeys[1].Ed25519},
		}

		validatorState := ValidatorState{
			CurrentValidators:  validatorsData,
			ArchivedValidators: validatorsData,
		}
		currentTimeslot := jamtime.Timeslot(100)

		workReport := block.WorkReport{
			CoreIndex: 0,
		}

		reportBytes, err := json.Marshal(workReport)
		require.NoError(t, err)
		reportHash := crypto.HashData(reportBytes)
		message := append([]byte(signatureContextGuarantee), reportHash[:]...)
		signature1 := ed25519.Sign(prvKey1, message)
		signature2 := ed25519.Sign(prvKey2, message)

		// Create credentials with unordered validator indices
		credentials := []block.CredentialSignature{
			{
				ValidatorIndex: 1,
				Signature:      [64]byte(signature2),
			},
			{
				ValidatorIndex: 0,
				Signature:      [64]byte(signature1),
			},
		}

		guarantees := block.GuaranteesExtrinsic{
			Guarantees: []block.Guarantee{
				{
					WorkReport:  workReport,
					Timeslot:    currentTimeslot - 1,
					Credentials: credentials,
				},
			},
		}

		intermediateAssignments := CoreAssignments{}

		newAssignments := calculateNewCoreAssignments(
			guarantees,
			intermediateAssignments,
			validatorState,
			currentTimeslot,
		)

		require.Nil(t, newAssignments[0].WorkReport)
	})

	t.Run("invalid guarantee due to wrong signature", func(t *testing.T) {
		pubKey1, _, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		pubKey2, prvKey2, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)
		_, wrongPrvKey, err := testutils.RandomED25519Keys(t)
		require.NoError(t, err)

		validatorKeys := make([]crypto.ValidatorKey, 2)
		validatorKeys[0].Ed25519 = pubKey1
		validatorKeys[1].Ed25519 = pubKey2
		validatorsData := safrole.ValidatorsData{
			{Ed25519: validatorKeys[0].Ed25519},
			{Ed25519: validatorKeys[1].Ed25519},
		}

		validatorState := ValidatorState{
			CurrentValidators:  validatorsData,
			ArchivedValidators: validatorsData,
		}
		currentTimeslot := jamtime.Timeslot(100)

		workReport := block.WorkReport{
			CoreIndex: 0,
		}

		reportBytes, err := json.Marshal(workReport)
		require.NoError(t, err)
		reportHash := crypto.HashData(reportBytes)
		message := append([]byte(signatureContextGuarantee), reportHash[:]...)
		wrongSignature := ed25519.Sign(wrongPrvKey, message)
		signature2 := ed25519.Sign(prvKey2, message)

		credentials := []block.CredentialSignature{
			{
				ValidatorIndex: 0,
				Signature:      [64]byte(wrongSignature), // Wrong signature for validator 0
			},
			{
				ValidatorIndex: 1,
				Signature:      [64]byte(signature2),
			},
		}

		guarantees := block.GuaranteesExtrinsic{
			Guarantees: []block.Guarantee{
				{
					WorkReport:  workReport,
					Timeslot:    currentTimeslot - 1,
					Credentials: credentials,
				},
			},
		}

		intermediateAssignments := CoreAssignments{}

		newAssignments := calculateNewCoreAssignments(
			guarantees,
			intermediateAssignments,
			validatorState,
			currentTimeslot,
		)

		require.Nil(t, newAssignments[0].WorkReport)
	})
}

func TestCalculateNewCoreAuthorizations(t *testing.T) {
	t.Run("add new authorizer to empty pool", func(t *testing.T) {
		header := block.Header{
			TimeSlotIndex: 1,
		}
		pendingAuths := PendingAuthorizersQueues{}
		currentAuths := CoreAuthorizersPool{}

		// Set up a pending authorizer for core 0
		newAuth := testutils.RandomHash(t)
		pendingAuths[0][1] = newAuth // At index 1 (matching TimeSlotIndex)

		newAuths := calculateNewCoreAuthorizations(header, block.GuaranteesExtrinsic{}, pendingAuths, currentAuths)

		require.Len(t, newAuths[0], 1)
		assert.Equal(t, newAuth, newAuths[0][0])
	})

	t.Run("remove used authorizer and add new one", func(t *testing.T) {
		header := block.Header{
			TimeSlotIndex: 1,
		}

		// Create a guarantee that uses an authorizer
		usedAuth := testutils.RandomHash(t)
		workReport := block.WorkReport{
			CoreIndex:      0,
			AuthorizerHash: usedAuth,
		}
		guarantees := block.GuaranteesExtrinsic{
			Guarantees: []block.Guarantee{
				{WorkReport: workReport},
			},
		}

		// Set up current authorizations with the used authorizer
		currentAuths := CoreAuthorizersPool{}
		currentAuths[0] = []crypto.Hash{usedAuth}

		// Set up pending authorizations with new authorizer
		pendingAuths := PendingAuthorizersQueues{}
		newAuth := testutils.RandomHash(t)
		pendingAuths[0][1] = newAuth // At index 1 (matching TimeSlotIndex)

		newAuths := calculateNewCoreAuthorizations(header, guarantees, pendingAuths, currentAuths)

		require.Len(t, newAuths[0], 1)
		assert.Equal(t, newAuth, newAuths[0][0])
		assert.NotContains(t, newAuths[0], usedAuth)
	})

	t.Run("maintain max size limit", func(t *testing.T) {
		header := block.Header{
			TimeSlotIndex: 1,
		}

		// Fill current authorizations to max size
		currentAuths := CoreAuthorizersPool{}
		for i := 0; i < MaxAuthorizersPerCore; i++ {
			currentAuths[0] = append(currentAuths[0], testutils.RandomHash(t))
		}

		// Set up new pending authorizer
		pendingAuths := PendingAuthorizersQueues{}
		newAuth := testutils.RandomHash(t)
		pendingAuths[0][1] = newAuth

		newAuths := calculateNewCoreAuthorizations(header, block.GuaranteesExtrinsic{}, pendingAuths, currentAuths)

		// Check that size limit is maintained and oldest auth was removed
		require.Len(t, newAuths[0], MaxAuthorizersPerCore)
		assert.Equal(t, newAuth, newAuths[0][MaxAuthorizersPerCore-1])
		assert.NotEqual(t, currentAuths[0][0], newAuths[0][0])
	})

	t.Run("handle empty pending authorization", func(t *testing.T) {
		header := block.Header{
			TimeSlotIndex: 1,
		}

		currentAuths := CoreAuthorizersPool{}
		existingAuth := testutils.RandomHash(t)
		currentAuths[0] = []crypto.Hash{existingAuth}

		// Empty pending authorizations
		pendingAuths := PendingAuthorizersQueues{}

		newAuths := calculateNewCoreAuthorizations(header, block.GuaranteesExtrinsic{}, pendingAuths, currentAuths)

		// Should keep existing authorizations unchanged
		require.Len(t, newAuths[0], 1)
		assert.Equal(t, existingAuth, newAuths[0][0])
	})

	t.Run("handle multiple cores", func(t *testing.T) {
		header := block.Header{
			TimeSlotIndex: 1,
		}

		// Set up authorizations for two cores
		currentAuths := CoreAuthorizersPool{}
		existingAuth0 := testutils.RandomHash(t)
		existingAuth1 := testutils.RandomHash(t)
		currentAuths[0] = []crypto.Hash{existingAuth0}
		currentAuths[1] = []crypto.Hash{existingAuth1}

		// Set up new pending authorizations
		pendingAuths := PendingAuthorizersQueues{}
		newAuth0 := testutils.RandomHash(t)
		newAuth1 := testutils.RandomHash(t)
		pendingAuths[0][1] = newAuth0
		pendingAuths[1][1] = newAuth1

		newAuths := calculateNewCoreAuthorizations(header, block.GuaranteesExtrinsic{}, pendingAuths, currentAuths)

		require.Len(t, newAuths[0], 2)
		require.Len(t, newAuths[1], 2)
		assert.Contains(t, newAuths[0], existingAuth0)
		assert.Contains(t, newAuths[0], newAuth0)
		assert.Contains(t, newAuths[1], existingAuth1)
		assert.Contains(t, newAuths[1], newAuth1)
	})
}

func TestCalculateNewValidatorStatistics(t *testing.T) {
    t.Run("new epoch transition", func(t *testing.T) {
        // Initial state with some existing stats
        initialStats := ValidatorStatisticsState{
            0: [1023]ValidatorStatistics{
                0: {NumOfBlocks: 5},
                1: {NumOfTickets: 3},
            },
            1: [1023]ValidatorStatistics{
                0: {NumOfBlocks: 10},
                1: {NumOfTickets: 6},
            },
        }

        block := block.Block{
            Header: block.Header{
                TimeSlotIndex: jamtime.Timeslot(600), // First slot in new epoch
				BlockAuthorIndex: 2,
            },
        }

        newStats := calculateNewValidatorStatistics(block, jamtime.Timeslot(600), initialStats)

        // Check that stats were rotated correctly
        assert.Equal(t, uint32(10), newStats[0][0].NumOfBlocks, "Previous current stats should become history")
        assert.Equal(t, uint64(6), newStats[0][1].NumOfTickets, "Previous current stats should become history")
        assert.Equal(t, uint32(0), newStats[1][0].NumOfBlocks, "Current stats should be reset")
        assert.Equal(t, uint64(0), newStats[1][1].NumOfTickets, "Current stats should be reset")
    })

    t.Run("block author statistics", func(t *testing.T) {
        initialStats := ValidatorStatisticsState{
            1: [1023]ValidatorStatistics{}, // Current epoch stats
        }

        block := block.Block{
            Header: block.Header{
                TimeSlotIndex: jamtime.Timeslot(5),
                BlockAuthorIndex: 1,
            },
            Extrinsic: block.Extrinsic{
                ET: block.TicketExtrinsic{
                    TicketProofs: []block.TicketProof{{}, {}, {}}, // 3 tickets
                },
                EP: block.PreimageExtrinsic{
                    {Data: []byte("test1")},
                    {Data: []byte("test2")},
                },
            },
        }

        newStats := calculateNewValidatorStatistics(block, jamtime.Timeslot(5), initialStats)

        // Check block author stats
        assert.Equal(t, uint32(1), newStats[1][1].NumOfBlocks, "Block count should increment")
        assert.Equal(t, uint64(3), newStats[1][1].NumOfTickets, "Ticket count should match")
        assert.Equal(t, uint64(2), newStats[1][1].NumOfPreimages, "Preimage count should match")
        assert.Equal(t, uint64(10), newStats[1][1].NumOfBytesAllPreimages, "Preimage bytes should match")
        
        // Check non-author stats remained zero
        assert.Equal(t, uint32(0), newStats[1][0].NumOfBlocks, "Non-author stats should remain zero")
    })

    t.Run("guarantees and assurances", func(t *testing.T) {
        initialStats := ValidatorStatisticsState{
            1: [1023]ValidatorStatistics{}, // Current epoch stats
        }

        block := block.Block{
            Header: block.Header{
                TimeSlotIndex: jamtime.Timeslot(5),
            },
            Extrinsic: block.Extrinsic{
                EG: block.GuaranteesExtrinsic{
                    Guarantees: []block.Guarantee{
                        {
                            Credentials: []block.CredentialSignature{
                                {ValidatorIndex: 0},
                                {ValidatorIndex: 1},
                            },
                        },
                        {
                            Credentials: []block.CredentialSignature{
                                {ValidatorIndex: 0},
                            },
                        },
                    },
                },
                EA: block.AssurancesExtrinsic{
                    {ValidatorIndex: 0},
                    {ValidatorIndex: 1},
                },
            },
        }

        newStats := calculateNewValidatorStatistics(block, jamtime.Timeslot(5), initialStats)

        // Check guarantees and assurances
        assert.Equal(t, uint64(2), newStats[1][0].NumOfGuaranteedReports, "Should count all guarantees for validator 0")
        assert.Equal(t, uint64(1), newStats[1][1].NumOfGuaranteedReports, "Should count all guarantees for validator 1")
        assert.Equal(t, uint64(1), newStats[1][0].NumOfAvailabilityAssurances, "Should count assurance for validator 0")
        assert.Equal(t, uint64(1), newStats[1][1].NumOfAvailabilityAssurances, "Should count assurance for validator 1")
    })

    t.Run("full block processing", func(t *testing.T) {
        initialStats := ValidatorStatisticsState{
            1: [1023]ValidatorStatistics{
                1: {
                    NumOfBlocks: 5,
                    NumOfTickets: 10,
                    NumOfPreimages: 2,
                    NumOfBytesAllPreimages: 100,
                    NumOfGuaranteedReports: 3,
                    NumOfAvailabilityAssurances: 2,
                },
            },
        }

        block := block.Block{
            Header: block.Header{
                TimeSlotIndex: jamtime.Timeslot(5),
                BlockAuthorIndex: 1,
            },
            Extrinsic: block.Extrinsic{
                ET: block.TicketExtrinsic{
                    TicketProofs: []block.TicketProof{{}, {}}, // 2 tickets
                },
                EP: block.PreimageExtrinsic{
                    {Data: []byte("test")}, // 4 bytes
                },
                EG: block.GuaranteesExtrinsic{
                    Guarantees: []block.Guarantee{
                        {
                            Credentials: []block.CredentialSignature{
                                {ValidatorIndex: 1},
                            },
                        },
                    },
                },
                EA: block.AssurancesExtrinsic{
                    {ValidatorIndex: 1},
                },
            },
        }

        newStats := calculateNewValidatorStatistics(block, jamtime.Timeslot(5), initialStats)

        expected := ValidatorStatistics{
            NumOfBlocks: 6,
            NumOfTickets: 12,
            NumOfPreimages: 3,
            NumOfBytesAllPreimages: 104,
            NumOfGuaranteedReports: 4,
            NumOfAvailabilityAssurances: 3,
        }

        assert.Equal(t, expected, newStats[1][1], "All statistics should be updated correctly")
    })
}

func createVerdictWithJudgments(reportHash crypto.Hash, positiveJudgments uint16) block.Verdict {
	var judgments [common.ValidatorsSuperMajority]block.Judgement
	for i := uint16(0); i < positiveJudgments; i++ {
		judgments[i] = block.Judgement{
			IsValid:        i < positiveJudgments,
			ValidatorIndex: i,
		}
	}
	return block.Verdict{
		ReportHash: reportHash,
		Judgements: judgments,
	}
}

func createInitialAssignments() CoreAssignments {
	var initialAssignments CoreAssignments
	for i := range initialAssignments {
		initialAssignments[i] = Assignment{
			WorkReport: &block.WorkReport{CoreIndex: uint16(i)},
			Time:       jamtime.Timeslot(100),
		}
	}
	return initialAssignments
}

func createAssuranceExtrinsic(availableCores uint16, validators uint16) block.AssurancesExtrinsic {
	assurances := make(block.AssurancesExtrinsic, validators)
	for i := uint16(0); i < validators; i++ {
		assurance := block.Assurance{
			ValidatorIndex: uint16(i),
		}

		for j := uint16(0); j < availableCores && j < common.TotalNumberOfCores; j++ {
			byteIndex := j / 8
			bitIndex := j % 8
			assurance.Bitfield[byteIndex] |= 1 << bitIndex
		}

		assurances[i] = assurance
	}
	return assurances
}
