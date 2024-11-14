package host_call

import (
	"maps"
	"math"

	"github.com/eigerco/strawberry/internal/block"
	"github.com/eigerco/strawberry/internal/common"
	"github.com/eigerco/strawberry/internal/crypto"
	"github.com/eigerco/strawberry/internal/jamtime"
	. "github.com/eigerco/strawberry/internal/polkavm"
	. "github.com/eigerco/strawberry/internal/polkavm/util"
	"github.com/eigerco/strawberry/internal/service"
	"github.com/eigerco/strawberry/internal/state"
)

// Empower ΩE(ϱ, ω, μ, (x, y))
func Empower(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < EmpowerCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= EmpowerCost

	// let [m, a, v, o, n] = ω7...12
	managerServiceId, assignServiceId, designateServiceId, addr, servicesNr := regs[A0], regs[A1], regs[A2], regs[A3], regs[A4]
	// let g = {(s ↦ g) where E4(s) ⌢ E8(g) = μ_o+12i⋅⋅⋅+12 | i ∈ Nn} if Zo⋅⋅⋅+12n ⊂ Vμ otherwise ∇
	for i := range servicesNr {
		serviceId, err := readNumber[block.ServiceId](mem, addr+(12*i), 4)
		if err != nil {
			return gas, withCode(regs, OOB), mem, ctxPair, err
		}
		serviceGas, err := readNumber[uint64](mem, addr+(12*i)+4, 8)
		if err != nil {
			return gas, withCode(regs, OOB), mem, ctxPair, err
		}

		if ctxPair.RegularCtx.AccumulationState.PrivilegedServices.AmountOfGasPerServiceId == nil {
			ctxPair.RegularCtx.AccumulationState.PrivilegedServices.AmountOfGasPerServiceId = make(map[block.ServiceId]uint64)
		}
		ctxPair.RegularCtx.AccumulationState.PrivilegedServices.AmountOfGasPerServiceId[serviceId] = serviceGas
	}
	ctxPair.RegularCtx.AccumulationState.PrivilegedServices.ManagerServiceId = block.ServiceId(managerServiceId)
	ctxPair.RegularCtx.AccumulationState.PrivilegedServices.AssignServiceId = block.ServiceId(assignServiceId)
	ctxPair.RegularCtx.AccumulationState.PrivilegedServices.DesignateServiceId = block.ServiceId(designateServiceId)
	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Assign ΩA(ϱ, ω, μ, (x, y))
func Assign(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < AssignCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= AssignCost

	// let o = ω8
	addr := regs[A1]
	core := regs[A0]
	if core >= uint32(common.TotalNumberOfCores) {
		return gas, withCode(regs, CORE), mem, ctxPair, nil
	}
	for i := 0; i < state.PendingAuthorizersQueueSize; i++ {
		bytes := make([]byte, 32)
		if err := mem.Read(addr+uint32(32*i), bytes); err != nil {
			return gas, withCode(regs, OOB), mem, ctxPair, nil
		}
		ctxPair.RegularCtx.AccumulationState.WorkReportsQueue[core][i] = crypto.Hash(bytes)
	}
	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Designate ΩD (ϱ, ω, μ, (x, y))
func Designate(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < DesignateCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= DesignateCost

	const (
		bandersnatch = crypto.BandersnatchSize
		ed25519      = bandersnatch + 32
		bls          = ed25519 + crypto.BLSSize
		metadata     = bls + crypto.MetadataSize
	)
	// let o = ω7
	addr := regs[A0]
	for i := 0; i < common.NumberOfValidators; i++ {
		bytes := make([]byte, 336)
		if err := mem.Read(addr+uint32(336*i), bytes); err != nil {
			return gas, withCode(regs, OOB), mem, ctxPair, nil
		}

		ctxPair.RegularCtx.AccumulationState.ValidatorKeys[i] = crypto.ValidatorKey{
			Bandersnatch: crypto.BandersnatchPublicKey(bytes[:bandersnatch]),
			Ed25519:      bytes[bandersnatch:ed25519],
			Bls:          crypto.BlsKey(bytes[ed25519:bls]),
			Metadata:     crypto.MetadataKey(bytes[bls:metadata]),
		}
	}

	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Checkpoint ΩC(ϱ, ω, μ, (x, y))
func Checkpoint(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < CheckpointCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= CheckpointCost

	ctxPair.ExceptionalCtx = ctxPair.RegularCtx

	// Split the new ϱ' value into its lower and upper parts.
	regs[A0] = uint32(gas & ((1 << 32) - 1))
	regs[A1] = uint32(gas >> 32)

	return gas, regs, mem, ctxPair, nil
}

// New ΩN(ϱ, ω, μ, (x, y))
func New(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < NewCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= NewCost

	// let [o, l, gl, gh, ml, mh] = ω7..13
	addr, preimageLength, gl, gh, ml, mh := regs[A0], regs[A1], regs[A2], regs[A3], regs[A4], regs[A5]

	// c = μo⋅⋅⋅+32 if No⋅⋅⋅+32 ⊂ Vμ otherwise ∇
	codeHashBytes := make([]byte, 32)
	if err := mem.Read(addr, codeHashBytes); err != nil {
		return gas, withCode(regs, OOB), mem, ctxPair, nil
	}
	// let g = 2^32 ⋅ gh + gl
	gasLimitAccumulator := uint64(gh)<<32 | uint64(gl)

	// let m = 2^32 ⋅ mh + ml
	gasLimitTransfer := uint64(mh)<<32 | uint64(ml)

	codeHash := crypto.Hash(codeHashBytes)

	// let a = (c, s ∶ {}, l ∶ {(c, l) ↦ []}, b ∶ at, g, m) if c ≠ ∇
	account := service.ServiceAccount{
		Storage: make(map[crypto.Hash][]byte),
		PreimageMeta: map[service.PreImageMetaKey]service.PreimageHistoricalTimeslots{
			{Hash: codeHash, Length: service.PreimageLength(preimageLength)}: {},
		},
		CodeHash:               codeHash,
		GasLimitForAccumulator: gasLimitAccumulator,
		GasLimitOnTransfer:     gasLimitTransfer,
	}
	account.Balance = account.ThresholdBalance()

	// let b = (Xs)b − at
	b := ctxPair.RegularCtx.ServiceAccount().Balance - account.ThresholdBalance()

	// if a ≠ ∇ ∧ b ≥ (xs)t
	if b >= ctxPair.RegularCtx.ServiceAccount().ThresholdBalance() {
		regs[A0] = uint32(ctxPair.RegularCtx.ServiceId)
		currentAccount := ctxPair.RegularCtx.ServiceAccount()
		currentAccount.Balance = b
		ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = currentAccount

		// check(bump(xi))
		ctxPair.RegularCtx.ServiceId = Check(Bump(ctxPair.RegularCtx.NewServiceId), ctxPair.RegularCtx.AccumulationState.ServiceState)
		//(xu)d ∪ {xi ↦ a, xs ↦ s}, b
		ctxPair.RegularCtx.AccumulationState.ServiceState[ctxPair.RegularCtx.ServiceId] = account
		return gas, regs, mem, ctxPair, nil
	}

	// otherwise
	return gas, withCode(regs, CASH), mem, ctxPair, nil
}

// Upgrade ΩU(ϱ, ω, μ, (x, y))
func Upgrade(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < UpgradeCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= UpgradeCost
	// let [o, gh, gl, mh, ml] = ω7...12
	addr, gl, gh, ml, mh := regs[A0], regs[A1], regs[A2], regs[A3], regs[A4]

	// c = μo⋅⋅⋅+32 if No⋅⋅⋅+32 ⊂ Vμ otherwise ∇
	codeHash := make([]byte, 32)
	if err := mem.Read(addr, codeHash); err != nil {
		return gas, withCode(regs, OOB), mem, ctxPair, nil
	}

	// let g = 2^32 ⋅ gh + gl
	gasLimitAccumulator := uint64(gh)<<32 | uint64(gl)

	// let m = 2^32 ⋅ mh + ml
	gasLimitTransfer := uint64(mh)<<32 | uint64(ml)

	// (ω′7, (X′s)c, (X′s)g , (X′s)m) = (OK, c, g, m) if c ≠ ∇
	currentService := ctxPair.RegularCtx.ServiceAccount()
	currentService.CodeHash = crypto.Hash(codeHash)
	currentService.GasLimitForAccumulator = gasLimitAccumulator
	currentService.GasLimitOnTransfer = gasLimitTransfer
	ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = currentService
	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Transfer ΩT(ϱ, ω, μ, (x, y))
func Transfer(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	// let (d, al, ah, gl, gh, o) = ω7..13
	receiverId, al, ah, gl, gh, o := regs[A0], regs[A1], regs[A2], regs[A3], regs[A4], regs[A5]

	// let a = 2^32 ⋅ ah + al
	newBalance := uint64(ah)<<32 | uint64(al)

	transferCost := TransferBaseCost + Gas(newBalance)
	if gas < transferCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= transferCost

	// let g = 2^32 ⋅ gh + gl
	gasLimit := uint64(gh)<<32 | uint64(gl)

	// m = μo⋅⋅⋅+M if No⋅⋅⋅+M ⊂ Vμ otherwise ∇
	m := make([]byte, service.TransferMemoSizeBytes)
	if err := mem.Read(o, m); err != nil {
		return gas, withCode(regs, OK), mem, ctxPair, nil
	}

	// let t ∈ T = (s, d, a, m, g)
	deferredTransfer := service.DeferredTransfer{
		SenderServiceIndex:   ctxPair.RegularCtx.ServiceId,
		ReceiverServiceIndex: block.ServiceId(receiverId),
		Balance:              newBalance,
		Memo:                 service.Memo(m),
		GasLimit:             gasLimit,
	}

	// let d = xd ∪ (xu)d
	allServices := maps.Clone(ctxPair.RegularCtx.ServiceState)
	maps.Copy(allServices, ctxPair.RegularCtx.AccumulationState.ServiceState)

	receiverService, ok := allServices[block.ServiceId(receiverId)]
	// if d !∈ K(δ ∪ xn)
	if !ok {
		return gas, withCode(regs, WHO), mem, ctxPair, nil
	}

	// if g < (δ ∪ xn)[d]m
	if gasLimit < receiverService.GasLimitOnTransfer {
		return gas, withCode(regs, LOW), mem, ctxPair, nil
	}

	// if ϱ < g
	if gas < Gas(gasLimit) {
		return gas, withCode(regs, HIGH), mem, ctxPair, nil
	}

	// let b = (xs)b − a
	// if b < (xs)t
	if ctxPair.RegularCtx.ServiceAccount().Balance-newBalance < ctxPair.RegularCtx.ServiceAccount().ThresholdBalance() {
		return gas, withCode(regs, CASH), mem, ctxPair, nil
	}

	ctxPair.RegularCtx.DeferredTransfers = append(ctxPair.RegularCtx.DeferredTransfers, deferredTransfer)
	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Quit ΩQ(ϱ, ω, μ, (x, y))
func Quit(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < QuitCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= QuitCost

	// let [d, o] = ω7,8
	receiverId, addr := regs[A0], regs[A1]

	// let a = (xs)b − (xs)t + BS
	newBalance := ctxPair.RegularCtx.ServiceAccount().Balance - ctxPair.RegularCtx.ServiceAccount().ThresholdBalance() + service.BasicMinimumBalance

	// let g = ϱ
	gasLimit := uint64(gas)

	// m = E−1(μo⋅⋅⋅+M)
	memo := make([]byte, service.TransferMemoSizeBytes)
	if err := mem.Read(addr, memo); err != nil {
		return gas, withCode(regs, OOB), mem, ctxPair, nil
	}

	// if d ∈ {s, 2^32 − 1}
	if block.ServiceId(receiverId) == ctxPair.RegularCtx.ServiceId || receiverId == math.MaxUint32 {
		delete(ctxPair.RegularCtx.AccumulationState.ServiceState, ctxPair.RegularCtx.ServiceId)
		return gas, withCode(regs, OK), mem, ctxPair, ErrHalt
	}
	// let t ∈ T ≡ (s, d, a, m, g)
	deferredTransfer := service.DeferredTransfer{
		SenderServiceIndex:   ctxPair.RegularCtx.ServiceId,
		ReceiverServiceIndex: block.ServiceId(receiverId),
		Balance:              newBalance,
		Memo:                 service.Memo(memo),
		GasLimit:             gasLimit,
	}
	// let d = xd ∪ (xu)d
	allServices := maps.Clone(ctxPair.RegularCtx.ServiceState)
	maps.Copy(allServices, ctxPair.RegularCtx.AccumulationState.ServiceState)

	receiverService, ok := allServices[block.ServiceId(receiverId)]
	// if d !∈ K(d)
	if !ok {
		return gas, withCode(regs, WHO), mem, ctxPair, nil
	}
	//if g < d[d]m
	if gasLimit < receiverService.GasLimitOnTransfer {
		return gas, withCode(regs, LOW), mem, ctxPair, nil
	}

	ctxPair.RegularCtx.DeferredTransfers = append(ctxPair.RegularCtx.DeferredTransfers, deferredTransfer)
	return gas, withCode(regs, OK), mem, ctxPair, ErrHalt
}

// Solicit ΩS(ϱ, ω, μ, (x, y), t)
func Solicit(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair, timeslot jamtime.Timeslot) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < SolicitCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= SolicitCost

	// let [o, z] = ω7,8
	addr, preimageLength := regs[A0], regs[A1]
	// let h = μo⋅⋅⋅+32 if Zo⋅⋅⋅+32 ⊂ Vμ otherwise ∇
	preimageHashBytes := make([]byte, 32)
	if err := mem.Read(addr, preimageHashBytes); err != nil {
		return gas, withCode(regs, OOB), mem, ctxPair, nil
	}

	// let a = xs
	serviceAccount := ctxPair.RegularCtx.ServiceAccount()
	preimageHash := crypto.Hash(preimageHashBytes)
	// (h, z)
	key := service.PreImageMetaKey{Hash: preimageHash, Length: service.PreimageLength(preimageLength)}

	if _, ok := serviceAccount.PreimageMeta[key]; !ok {
		// except: al[(h, z)] = [] if h ≠ ∇ ∧ (h, z) !∈ (xs)l
		serviceAccount.PreimageMeta[key] = service.PreimageHistoricalTimeslots{}
	} else if len(serviceAccount.PreimageMeta[key]) == 2 {
		// except: al[(h, z)] = (xs)l[(h, z)] ++ t if (xs)l[(h, z)] = [X, Y]
		serviceAccount.PreimageMeta[key] = append(serviceAccount.PreimageMeta[key], timeslot)
	} else {
		return gas, withCode(regs, HUH), mem, ctxPair, nil
	}

	// if ab < at
	if serviceAccount.Balance < serviceAccount.ThresholdBalance() {
		return gas, withCode(regs, FULL), mem, ctxPair, nil
	}

	ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = serviceAccount
	return gas, withCode(regs, OK), mem, ctxPair, nil
}

// Forget ΩF(ϱ, ω, μ, (x, y), t)
func Forget(gas Gas, regs Registers, mem Memory, ctxPair AccumulateContextPair, timeslot jamtime.Timeslot) (Gas, Registers, Memory, AccumulateContextPair, error) {
	if gas < ForgetCost {
		return gas, regs, mem, ctxPair, ErrOutOfGas
	}
	gas -= ForgetCost

	// let [o, z] = ω0,1
	addr, preimageLength := regs[A0], regs[A1]

	// let h = μo⋅⋅⋅+32 if Zo⋅⋅⋅+32 ⊂ Vμ otherwise ∇
	preimageHashBytes := make([]byte, 32)
	if err := mem.Read(addr, preimageHashBytes); err != nil {
		return gas, withCode(regs, OOB), mem, ctxPair, nil
	}

	// let a = xs
	serviceAccount := ctxPair.RegularCtx.ServiceAccount()
	preimageHash := crypto.Hash(preimageHashBytes)

	// (h, z)
	key := service.PreImageMetaKey{Hash: preimageHash, Length: service.PreimageLength(preimageLength)}

	switch len(serviceAccount.PreimageMeta[key]) {
	case 0: // if (xs)l[h, z] ∈ {[]}

		// except: K(al) = K((xs)l) ∖ {(h, z)}
		// except: K(ap) = K((xs)p) ∖ {h}
		delete(serviceAccount.PreimageMeta, key)
		delete(serviceAccount.PreimageLookup, preimageHash)

		ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = serviceAccount
		return gas, withCode(regs, OK), mem, ctxPair, nil

	case 2: // if (xs)l[h, z] ∈ {[], [X, Y]}, Y < t − D
		if serviceAccount.PreimageMeta[key][1] < timeslot-jamtime.PreimageExpulsionPeriod {

			// except: K(al) = K((xs)l) ∖ {(h, z)}
			// except: K(ap) = K((xs)p) ∖ {h}
			delete(serviceAccount.PreimageMeta, key)
			delete(serviceAccount.PreimageLookup, preimageHash)

			ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = serviceAccount
			return gas, withCode(regs, OK), mem, ctxPair, nil
		}

	case 1: // if S(xs)l[h, z]S = 1

		// except: al[h, z] = (xs)l[h, z] ++ t
		serviceAccount.PreimageMeta[key] = append(serviceAccount.PreimageMeta[key], timeslot)

		ctxPair.RegularCtx.ServiceState[ctxPair.RegularCtx.ServiceId] = serviceAccount
		return gas, withCode(regs, OK), mem, ctxPair, nil

	case 3: // if (xs)l[h, z] = [X, Y, w]
		if serviceAccount.PreimageMeta[key][1] < timeslot-jamtime.PreimageExpulsionPeriod { // if Y < t − D

			// except: al[h, z] = [(xs)l[h, z]2, t]
			serviceAccount.PreimageMeta[key] = service.PreimageHistoricalTimeslots{serviceAccount.PreimageMeta[key][2], timeslot}
			return gas, withCode(regs, OK), mem, ctxPair, nil
		}
	}

	return gas, withCode(regs, HUH), mem, ctxPair, nil
}
