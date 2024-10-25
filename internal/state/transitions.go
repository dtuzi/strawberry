package state

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/eigerco/strawberry/internal/block"
	"github.com/eigerco/strawberry/internal/common"
	"github.com/eigerco/strawberry/internal/crypto"
	"github.com/eigerco/strawberry/internal/jamtime"
	"github.com/eigerco/strawberry/internal/safrole"
)
const (
	signatureContextGuarantee = "$jam_guarantee"
)
// TODO: These calculations are just mocks for now. They will be replaced with actual calculations when the state transitions are implemented.

// Intermediate State Calculation Functions

// calculateIntermediateBlockState Equation 17: β† ≺ (H, β)
func calculateIntermediateBlockState(header block.Header, previousRecentBlocks []BlockState) []BlockState {
	intermediateBlocks := make([]BlockState, len(previousRecentBlocks))

	// Copy all elements from previousRecentBlocks to intermediateBlocks
	copy(intermediateBlocks, previousRecentBlocks)

	// Update the state root of the most recent block
	if len(intermediateBlocks) > 0 {
		lastIndex := len(intermediateBlocks) - 1
		intermediateBlocks[lastIndex].StateRoot = header.PriorStateRoot
	}

	return intermediateBlocks
}

// calculateIntermediateServiceState Equation 24: δ† ≺ (EP, δ, τ′)
// This function calculates the intermediate service state δ† based on:
// - The current service state δ (serviceState)
// - The preimage extrinsic EP (preimages)
// - The new timeslot τ′ (newTimeslot)
//
// For each preimage in EP:
//  1. It adds the preimage p to the PreimageLookup of service s, keyed by its hash H(p)
//  2. It adds a new entry to the PreimageMeta of service s, keyed by the hash H(p) and
//     length |p|, with the value being the new timeslot τ′
//
// The function returns a new ServiceState without modifying the input state.
func calculateIntermediateServiceState(preimages block.PreimageExtrinsic, serviceState ServiceState, newTimeslot jamtime.Timeslot) ServiceState {
	// Equation 156:
	// δ† = δ ex. ∀⎧⎩s, p⎫⎭ ∈ EP:
	// ⎧ δ†[s]p[H(p)] = p
	// ⎩ δ†[s]l[H(p), |p|] = [τ′]

	// Shallow copy of the entire state
	newState := make(ServiceState, len(serviceState))
	for k, v := range serviceState {
		newState[k] = v
	}

	for _, preimage := range preimages {
		serviceId := block.ServiceId(preimage.ServiceIndex)
		account, exists := newState[serviceId]
		if !exists {
			continue
		}

		preimageHash := crypto.HashData(preimage.Data)
		preimageLength := PreimageLength(len(preimage.Data))

		// Check conditions from equation 155
		// Eq. 155: ∀⎧⎩s, p⎫⎭ ∈ EP : K(δ[s]p) ∌ H(p) ∧ δ[s]l[⎧⎩H(p), |p|⎫⎭] = []
		// For all preimages: hash not in lookup and no existing metadata
		if _, exists := account.PreimageLookup[preimageHash]; exists {
			continue // Skip if preimage already exists
		}
		metaKey := PreImageMetaKey{Hash: preimageHash, Length: preimageLength}
		if existingMeta, exists := account.PreimageMeta[metaKey]; exists && len(existingMeta) > 0 {
			continue // Skip if metadata already exists and is not empty
		}

		// If checks pass, add the new preimage
		if account.PreimageLookup == nil {
			account.PreimageLookup = make(map[crypto.Hash][]byte)
		}
		account.PreimageLookup[preimageHash] = preimage.Data

		if account.PreimageMeta == nil {
			account.PreimageMeta = make(map[PreImageMetaKey]PreimageHistoricalTimeslots)
		}
		account.PreimageMeta[metaKey] = []jamtime.Timeslot{newTimeslot}

		newState[serviceId] = account
	}

	return newState
}

// calculateIntermediateCoreAssignmentsFromExtrinsics Equation 25: ρ† ≺ (ED , ρ)
func calculateIntermediateCoreAssignmentsFromExtrinsics(disputes block.DisputeExtrinsic, coreAssignments CoreAssignments) CoreAssignments {
	newAssignments := coreAssignments // Create a copy of the current assignments

	// Process each verdict in the disputes
	for _, verdict := range disputes.Verdicts {
		reportHash := verdict.ReportHash
		positiveJudgments := block.CountPositiveJudgments(verdict.Judgements)

		// If less than 2/3 majority of positive judgments, clear the assignment for matching cores
		if positiveJudgments < common.ValidatorsSuperMajority {
			for c := uint16(0); c < common.TotalNumberOfCores; c++ {
				if newAssignments[c].WorkReport != nil {
					if hash, err := newAssignments[c].WorkReport.Hash(); err == nil && hash == reportHash {
						newAssignments[c] = Assignment{} // Clear the assignment
					}
				}
			}
		}
	}

	return newAssignments
}

// calculateIntermediateCoreAssignmentsFromAvailability implements equation 26: ρ‡ ≺ (EA, ρ†)
// It calculates the intermediate core assignments based on availability assurances.
func calculateIntermediateCoreAssignmentsFromAvailability(assurances block.AssurancesExtrinsic, coreAssignments CoreAssignments) CoreAssignments {
    // Initialize availability count for each core
    availabilityCounts := make([]int, common.TotalNumberOfCores)

    // Process each assurance in the AssurancesExtrinsic (EA)
    for _, assurance := range assurances {
        // Check the availability status for each core in this assurance
        for coreIndex := uint16(0); coreIndex < common.TotalNumberOfCores; coreIndex++ {
            // Calculate which byte and bit within the Bitfield correspond to this core
            byteIndex := coreIndex / 8
            bitIndex := coreIndex % 8

            // Check if the bit corresponding to this core is set (1) in the Bitfield
            if assurance.Bitfield[byteIndex]&(1<<bitIndex) != 0 {
                // If set, increment the availability count for this core
                availabilityCounts[coreIndex]++
            }
        }
    }

    // Create new CoreAssignments (ρ‡)
    var newAssignments CoreAssignments

    // Calculate the availability threshold (2/3 of validators)
    // This implements part of equation 129: ∑a∈EA av[c] > 2/3 V
    availabilityThreshold := (2 * common.NumberOfValidators) / 3

    // Update assignments based on availability
    // This implements equation 130: ∀c ∈ NC : ρ‡[c] ≡ { ∅ if ρ[c]w ∈ W, ρ†[c] otherwise }
    for coreIndex := uint16(0); coreIndex < common.TotalNumberOfCores; coreIndex++ {
        if availabilityCounts[coreIndex] > availabilityThreshold {
            // If the availability count exceeds the threshold, keep the assignment
            // This corresponds to ρ[c]w ∈ W in equation 129
            newAssignments[coreIndex] = coreAssignments[coreIndex]
        } else {
            // If the availability count doesn't exceed the threshold, clear the assignment
            // This corresponds to the ∅ case in equation 130
            newAssignments[coreIndex] = Assignment{}
        }
    }

    // Return the new intermediate CoreAssignments (ρ‡)
    return newAssignments
}

// Final State Calculation Functions

// calculateNewTimeState Equation 16: τ′ ≺ H
func calculateNewTimeState(header block.Header) jamtime.Timeslot {
	return header.TimeSlotIndex
}

// calculateNewRecentBlocks Equation 18: β′ ≺ (H, EG, β†, C)
func calculateNewRecentBlocks(header block.Header, guarantees block.GuaranteesExtrinsic, intermediateRecentBlocks []BlockState, context Context) ([]BlockState, error) {
	// Calculate accumulation-result Merkle tree root (r)
	accumulationRoot := calculateAccumulationRoot(context.Accumulations)

	// Append to the previous block's Merkle mountain range (b)
	var lastBlockMMR crypto.Hash
	if len(intermediateRecentBlocks) > 0 {
		lastBlockMMR = intermediateRecentBlocks[len(intermediateRecentBlocks)-1].AccumulationResultMMR
	}
	newMMR := AppendToMMR(lastBlockMMR, accumulationRoot)
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// Create new block state (n)
	reportHashes, err := calculateWorkReportHashes(guarantees)
	if err != nil {
		return nil, err
	}

	newBlockState := BlockState{
		HeaderHash:            crypto.HashData(headerBytes),
		StateRoot:             header.PriorStateRoot,
		AccumulationResultMMR: newMMR,
		WorkReportHashes:      reportHashes,
	}

	// Update β† with the new block state (Equation 83)
	newRecentBlocks := append(intermediateRecentBlocks, newBlockState)

	// Ensure we only keep the most recent H blocks
	if len(newRecentBlocks) > MaxRecentBlocks {
		newRecentBlocks = newRecentBlocks[len(newRecentBlocks)-MaxRecentBlocks:]
	}

	return newRecentBlocks, nil
}

// TODO: this is just a mock implementation
func AppendToMMR(lastBlockMMR crypto.Hash, accumulationRoot crypto.Hash) crypto.Hash {
	return crypto.Hash{}
}

// TODO: this is just a mock implementation
// This should create a Merkle tree from the accumulations and return the root
func calculateAccumulationRoot(accumulations map[uint32]crypto.Hash) crypto.Hash {
	return crypto.Hash{}
}

func calculateWorkReportHashes(guarantees block.GuaranteesExtrinsic) ([common.TotalNumberOfCores]crypto.Hash, error) {
	var hashes [common.TotalNumberOfCores]crypto.Hash
	for _, guarantee := range guarantees.Guarantees {
		// Assuming CoreIndex is part of the WorkReport struct
		coreIndex := guarantee.WorkReport.CoreIndex
		reportBytes, err := json.Marshal(guarantee.WorkReport)
		if err != nil {
			return [common.TotalNumberOfCores]crypto.Hash{}, err
		}
		hashes[coreIndex] = crypto.HashData(reportBytes)
	}
	return hashes, nil
}

// calculateNewSafroleState Equation 19: γ′ ≺ (H, τ, ET , γ, ι, η′, κ′)
func calculateNewSafroleState(header block.Header, timeslot jamtime.Timeslot, tickets block.TicketExtrinsic, queuedValidators safrole.ValidatorsData) (safrole.State, error) {
	if !header.TimeSlotIndex.IsFirstTimeslotInEpoch() {
		return safrole.State{}, errors.New("not first timeslot in epoch")
	}
	validTickets := block.ExtractTicketFromProof(tickets.TicketProofs)
	newSafrole := safrole.State{}
	newNextValidators := nullifyOffenders(queuedValidators, header.OffendersMarkers)
	ringCommitment := CalculateRingCommitment(newNextValidators)
	newSealingKeySeries, err := safrole.DetermineNewSealingKeys(timeslot, validTickets, safrole.TicketsOrKeys{}, header.EpochMarker)
	if err != nil {
		return safrole.State{}, err
	}
	newSafrole.NextValidators = newNextValidators
	newSafrole.RingCommitment = ringCommitment
	newSafrole.SealingKeySeries = newSealingKeySeries
	return newSafrole, nil
}

// calculateNewEntropyPool Equation 20: η′ ≺ (H, τ, η)
func calculateNewEntropyPool(header block.Header, timeslot jamtime.Timeslot, entropyPool EntropyPool) (EntropyPool, error) {
	newEntropyPool := entropyPool
	vrfOutput, err := extractVRFOutput(header)
	if err != nil {
		return EntropyPool{}, err
	}
	newEntropy := crypto.Hash(append(entropyPool[0][:], vrfOutput[:]...))
	if header.TimeSlotIndex.IsFirstTimeslotInEpoch() {
		newEntropyPool = rotateEntropyPool(entropyPool)
	}
	newEntropyPool[0] = newEntropy
	return newEntropyPool, nil
}

// calculateNewCoreAuthorizations implements equation 29: α' ≺ (H, EG, φ', α)
func calculateNewCoreAuthorizations(header block.Header, guarantees block.GuaranteesExtrinsic, pendingAuthorizations PendingAuthorizersQueues, currentAuthorizations CoreAuthorizersPool) CoreAuthorizersPool {
    var newCoreAuthorizations CoreAuthorizersPool

    // For each core
    for c := uint16(0); c < common.TotalNumberOfCores; c++ {
        // Start with the existing authorizations for this core
        newAuths := make([]crypto.Hash, len(currentAuthorizations[c]))
        copy(newAuths, currentAuthorizations[c])

        // F(c) - Remove authorizer if it was used in a guarantee for this core
        for _, guarantee := range guarantees.Guarantees {
            if guarantee.WorkReport.CoreIndex == c {
                // Remove the used authorizer from the list 
                newAuths = removeAuthorizer(newAuths, guarantee.WorkReport.AuthorizerHash)
            }
        }

        // Get new authorizer from the queue based on current timeslot
        // φ'[c][Ht]↺O - Get authorizer from queue, wrapping around queue size
        queueIndex := header.TimeSlotIndex % PendingAuthorizersQueueSize
        newAuthorizer := pendingAuthorizations[c][queueIndex]
        
        // Only add new authorizer if it's not empty
        if newAuthorizer != (crypto.Hash{}) {
            // ← Append new authorizer maintaining max size O
            newAuths = appendAuthorizerLimited(newAuths, newAuthorizer, MaxAuthorizersPerCore)
        }

        // Store the new authorizations for this core
        newCoreAuthorizations[c] = newAuths
    }

    return newCoreAuthorizations
}

// removeAuthorizer removes an authorizer from a list while maintaining order
func removeAuthorizer(auths []crypto.Hash, toRemove crypto.Hash) []crypto.Hash {
    for i := 0; i < len(auths); i++ {
        if auths[i] == toRemove {
            // Remove by shifting remaining elements left
            copy(auths[i:], auths[i+1:])
            return auths[:len(auths)-1]
        }
    }
    return auths
}

// appendAuthorizerLimited appends a new authorizer while maintaining max size limit
// This implements the "←" (append limited) operator from the paper
func appendAuthorizerLimited(auths []crypto.Hash, newAuth crypto.Hash, maxSize int) []crypto.Hash {
    // If at max size, remove oldest (leftmost) element
    if len(auths) >= maxSize {
        copy(auths, auths[1:])
        auths = auths[:len(auths)-1]
    }
    
    // Append new authorizer
    return append(auths, newAuth)
}

// calculateNewValidators Equation 21: κ′ ≺ (H, τ, κ, γ, ψ′)
func calculateNewValidators(header block.Header, timeslot jamtime.Timeslot, validators safrole.ValidatorsData, nextValidators safrole.ValidatorsData) (safrole.ValidatorsData, error) {
	if !header.TimeSlotIndex.IsFirstTimeslotInEpoch() {
		return validators, errors.New("not first timeslot in epoch")
	}
	return nextValidators, nil
}

// addUniqueHash adds a hash to a slice if it's not already present
func addUniqueHash(slice []crypto.Hash, hash crypto.Hash) []crypto.Hash {
	for _, v := range slice {
		if v == hash {
			return slice
		}
	}
	return append(slice, hash)
}

// addUniqueEdPubKey adds a public key to a slice if it's not already present
func addUniqueEdPubKey(slice []ed25519.PublicKey, key ed25519.PublicKey) []ed25519.PublicKey {
	for _, v := range slice {
		if bytes.Equal(v, key) {
			return slice
		}
	}
	return append(slice, key)
}

// processVerdict categorizes a verdict based on positive judgments. Equations 111, 112, 113.
func processVerdict(judgements *Judgements, verdict block.Verdict) {
	positiveJudgments := 0
	for _, judgment := range verdict.Judgements {
		if judgment.IsValid {
			positiveJudgments++
		}
	}

	switch positiveJudgments {
	// Equation 111: ψ'g ≡ ψg ∪ {r | {r, ⌊2/3V⌋ + 1} ∈ V}
	case common.ValidatorsSuperMajority:
		judgements.GoodWorkReports = addUniqueHash(judgements.GoodWorkReports, verdict.ReportHash)
		// Equation 112: ψ'b ≡ ψb ∪ {r | {r, 0} ∈ V}
	case 0:
		judgements.BadWorkReports = addUniqueHash(judgements.BadWorkReports, verdict.ReportHash)
		// Equation 113: ψ'w ≡ ψw ∪ {r | {r, ⌊1/3V⌋} ∈ V}
	case common.NumberOfValidators / 3:
		judgements.WonkyWorkReports = addUniqueHash(judgements.WonkyWorkReports, verdict.ReportHash)
		// TODO: The GP gives only the above 3 cases. Check back later how can we be sure only the above 3 cases are possible.
	default:
		panic(fmt.Sprintf("Unexpected number of positive judgments: %d", positiveJudgments))
	}
}

// processOffender adds an offending validator to the list
func processOffender(judgements *Judgements, key ed25519.PublicKey) {
	judgements.OffendingValidators = addUniqueEdPubKey(judgements.OffendingValidators, key)
}

// calculateNewJudgements Equation 23: ψ′ ≺ (ED, ψ)
func calculateNewJudgements(disputes block.DisputeExtrinsic, stateJudgements Judgements) Judgements {
	newJudgements := Judgements{
		BadWorkReports:      make([]crypto.Hash, len(stateJudgements.BadWorkReports)),
		GoodWorkReports:     make([]crypto.Hash, len(stateJudgements.GoodWorkReports)),
		WonkyWorkReports:    make([]crypto.Hash, len(stateJudgements.WonkyWorkReports)),
		OffendingValidators: make([]ed25519.PublicKey, len(stateJudgements.OffendingValidators)),
	}

	copy(newJudgements.BadWorkReports, stateJudgements.BadWorkReports)
	copy(newJudgements.GoodWorkReports, stateJudgements.GoodWorkReports)
	copy(newJudgements.WonkyWorkReports, stateJudgements.WonkyWorkReports)
	copy(newJudgements.OffendingValidators, stateJudgements.OffendingValidators)

	// Process verdicts (Equations 111, 112, 113)
	for _, verdict := range disputes.Verdicts {
		processVerdict(&newJudgements, verdict)
	}

	// Process culprits and faults (Equation 114)
	for _, culprit := range disputes.Culprits {
		processOffender(&newJudgements, culprit.ValidatorEd25519PublicKey)
	}
	for _, fault := range disputes.Faults {
		processOffender(&newJudgements, fault.ValidatorEd25519PublicKey)
	}

	return newJudgements
}

// calculateNewCoreAssignments updates the core assignments based on new guarantees.
// This implements equation 27: ρ′ ≺ (EG, ρ‡, κ, τ′)
//
// It also implements part of equation 139 regarding timeslot validation:
// R(⌊τ′/R⌋ - 1) ≤ t ≤ τ′
func calculateNewCoreAssignments(
	guarantees block.GuaranteesExtrinsic,
	intermediateAssignments CoreAssignments,
	validatorState ValidatorState,
	newTimeslot jamtime.Timeslot,
) CoreAssignments {
	newAssignments := intermediateAssignments
	sortedGuarantees := sortGuaranteesByCoreIndex(guarantees.Guarantees)

	for _, guarantee := range sortedGuarantees {
		coreIndex := guarantee.WorkReport.CoreIndex

		// Check timeslot range: R(⌊τ′/R⌋ - 1) ≤ t ≤ τ′
		previousRotationStart := (newTimeslot/common.ValidatorRotationPeriod - 1) * common.ValidatorRotationPeriod
		if guarantee.Timeslot < jamtime.Timeslot(previousRotationStart) ||
			guarantee.Timeslot > newTimeslot {
			continue
		}

		if isAssignmentValid(intermediateAssignments[coreIndex], newTimeslot) {
			// Determine which validator set to use based on timeslots
			validators := determineValidatorSet(
				guarantee.Timeslot,
				newTimeslot,
				validatorState.CurrentValidators,
				validatorState.ArchivedValidators,
			)

			if verifyGuaranteeCredentials(guarantee, validators) {
				newAssignments[coreIndex] = Assignment{
					WorkReport: &guarantee.WorkReport,
					Time:       newTimeslot,
				}
			}
		}
	}

	return newAssignments
}

// determineValidatorSet implements validator set selection from equations 135 and 139:
// From equation 139:
//
//	(c, k) = {
//	    G if ⌊τ′/R⌋ = ⌊t/R⌋
//	    G* otherwise
//
// Where G* is determined by equation 135:
//
//	let (e, k) = {
//	    (η′2, κ′) if ⌊τ′/R⌋ = ⌊τ′/E⌋
//	    (η′3, λ′) otherwise
func determineValidatorSet(
	guaranteeTimeslot jamtime.Timeslot,
	currentTimeslot jamtime.Timeslot,
	currentValidators safrole.ValidatorsData,
	archivedValidators safrole.ValidatorsData,
) safrole.ValidatorsData {
	currentRotation := currentTimeslot / common.ValidatorRotationPeriod
	guaranteeRotation := guaranteeTimeslot / common.ValidatorRotationPeriod

	if currentRotation == guaranteeRotation {
		return currentValidators
	}
	return archivedValidators
}

// sortGuaranteesByCoreIndex sorts the guarantees by their core index in ascending order.
// This implements equation 137 from the graypaper: EG = [(gw)c ^ g ∈ EG]
// which ensures that guarantees are ordered by core index.
func sortGuaranteesByCoreIndex(guarantees []block.Guarantee) []block.Guarantee {
	sortedGuarantees := make([]block.Guarantee, len(guarantees))
	copy(sortedGuarantees, guarantees)

	sort.Slice(sortedGuarantees, func(i, j int) bool {
		return sortedGuarantees[i].WorkReport.CoreIndex < sortedGuarantees[j].WorkReport.CoreIndex
	})

	return sortedGuarantees
}

// isAssignmentValid checks if a new assignment can be made for a core.
// This implements the condition from equation 142:
// ρ‡[wc] = ∅ ∨ Ht ≥ ρ‡[wc]t + U
func isAssignmentValid(currentAssignment Assignment, newTimeslot jamtime.Timeslot) bool {
	return currentAssignment.WorkReport == nil ||
		newTimeslot >= currentAssignment.Time+common.WorkReportTimeoutPeriod
}

// verifyGuaranteeCredentials verifies the credentials of a guarantee.
// This implements two equations from the graypaper:
//
// Equation 138: ∀g ∈ EG : ga = [v _ {v, s} ∈ ga]
// Which ensures credentials are ordered by validator index
//
//	Equation 139: ∀(w, t, a) ∈ EG, ∀(v, s) ∈ a : {
//	    s ∈ Ek[v]E⟨XG ⌢ H(E(w))⟩
//	    cv = wc
//	}
func verifyGuaranteeCredentials(
	guarantee block.Guarantee,
	validators safrole.ValidatorsData,
) bool {
	// Verify that credentials are ordered by validator index (equation 138)
	for i := 1; i < len(guarantee.Credentials); i++ {
		if guarantee.Credentials[i-1].ValidatorIndex >= guarantee.Credentials[i].ValidatorIndex {
			return false
		}
	}

	// Verify the signatures using the correct validator keys (equation 139)
	for _, credential := range guarantee.Credentials {
		if credential.ValidatorIndex >= uint16(len(validators)) {
			return false
		}

		// Check if the validator is assigned to the core specified in the work report
		if !isValidatorAssignedToCore(credential.ValidatorIndex, guarantee.WorkReport.CoreIndex, validators) {
			return false
		}

		validatorKey := validators[credential.ValidatorIndex]
		// Check if the validator key is valid
		if len(validatorKey.Ed25519) != ed25519.PublicKeySize {
			return false
		}
		reportBytes, err := json.Marshal(guarantee.WorkReport)
		if err != nil {
			return false
		}
		hashed := crypto.HashData(reportBytes)
		message := append([]byte(signatureContextGuarantee), hashed[:]...)
		if !ed25519.Verify(validatorKey.Ed25519, message, credential.Signature[:]) {
			return false
		}
	}

	return true
}

// TODO: This function should implement the logic to check if the validator is assigned to the core
// For now, it's a placeholder implementation
func isValidatorAssignedToCore(validatorIndex uint16, coreIndex uint16, validators safrole.ValidatorsData) bool {
	return true
}
// calculateNewArchivedValidators Equation 22: λ′ ≺ (H, τ, λ, κ)
func calculateNewArchivedValidators(header block.Header, timeslot jamtime.Timeslot, archivedValidators safrole.ValidatorsData, validators safrole.ValidatorsData) (safrole.ValidatorsData, error) {
	if !header.TimeSlotIndex.IsFirstTimeslotInEpoch() {
		return archivedValidators, errors.New("not first timeslot in epoch")
	}
	return validators, nil
}

// calculateServiceState Equation 28: δ′, 𝝌′, ι′, φ′, C ≺ (EA, ρ′, δ†, 𝝌, ι, φ)
func calculateServiceState(assurances block.AssurancesExtrinsic, coreAssignments CoreAssignments, intermediateServiceState ServiceState, privilegedServices PrivilegedServices, queuedValidators safrole.ValidatorsData, coreAuthorizationQueue PendingAuthorizersQueues) (ServiceState, PrivilegedServices, safrole.ValidatorsData, PendingAuthorizersQueues, Context) {
	return make(ServiceState), PrivilegedServices{}, safrole.ValidatorsData{}, PendingAuthorizersQueues{}, Context{}
}

// calculateNewValidatorStatistics Equation 30: π′ ≺ (EG, EP, EA, ET, τ, τ′, π)
func calculateNewValidatorStatistics(extrinsics block.Extrinsic, timeslot jamtime.Timeslot, newTimeSlot jamtime.Timeslot, validatorStatistics ValidatorStatisticsState) ValidatorStatisticsState {
	return ValidatorStatisticsState{}
}
