package pipeline

// BranchPredictorConfig holds configuration for the branch predictor.
type BranchPredictorConfig struct {
	// BHTSize is the number of entries in the Branch History Table.
	// Must be a power of 2. Default is 4096.
	BHTSize uint32
	// BTBSize is the number of entries in the Branch Target Buffer.
	// Must be a power of 2. Default is 512.
	BTBSize uint32
	// GlobalHistoryLength is the length of the global history register.
	// Default is 12 bits.
	GlobalHistoryLength uint32
	// UseTournament enables the tournament predictor (combining bimodal and gshare).
	// Default is true.
	UseTournament bool
}

// DefaultBranchPredictorConfig returns a default configuration.
func DefaultBranchPredictorConfig() BranchPredictorConfig {
	return BranchPredictorConfig{
		BHTSize:             4096,
		BTBSize:             512,
		GlobalHistoryLength: 12,
		UseTournament:       true,
	}
}

// BranchPredictorStats holds statistics for the branch predictor.
type BranchPredictorStats struct {
	// Predictions is the total number of branch predictions made.
	Predictions uint64
	// Correct is the number of correct predictions.
	Correct uint64
	// Mispredictions is the number of incorrect predictions.
	Mispredictions uint64
	// BTBHits is the number of BTB hits.
	BTBHits uint64
	// BTBMisses is the number of BTB misses.
	BTBMisses uint64
	// BimodalCorrect is correct predictions from the bimodal predictor.
	BimodalCorrect uint64
	// GshareCorrect is correct predictions from the gshare predictor.
	GshareCorrect uint64
	// TournamentChoseBimodal is how often the tournament chose bimodal.
	TournamentChoseBimodal uint64
	// TournamentChoseGshare is how often the tournament chose gshare.
	TournamentChoseGshare uint64
}

// Accuracy returns the prediction accuracy as a percentage.
func (s BranchPredictorStats) Accuracy() float64 {
	if s.Predictions == 0 {
		return 0
	}
	return float64(s.Correct) / float64(s.Predictions) * 100
}

// MispredictionRate returns the misprediction rate as a percentage.
func (s BranchPredictorStats) MispredictionRate() float64 {
	if s.Predictions == 0 {
		return 0
	}
	return float64(s.Mispredictions) / float64(s.Predictions) * 100
}

// BTBHitRate returns the BTB hit rate as a percentage.
func (s BranchPredictorStats) BTBHitRate() float64 {
	total := s.BTBHits + s.BTBMisses
	if total == 0 {
		return 0
	}
	return float64(s.BTBHits) / float64(total) * 100
}

// Prediction represents a branch prediction result.
type Prediction struct {
	// Taken indicates whether the branch is predicted to be taken.
	Taken bool
	// Target is the predicted target address (if known from BTB).
	Target uint64
	// TargetKnown indicates whether the target address is known.
	TargetKnown bool
}

// BranchPredictor implements a tournament predictor combining:
// 1. Bimodal predictor (2-bit saturating counters indexed by PC)
// 2. Gshare predictor (2-bit counters indexed by PC XOR global history)
// 3. Choice predictor (selects between bimodal and gshare)
//
// This matches the style of modern high-performance branch predictors.
type BranchPredictor struct {
	// Bimodal: Branch History Table - 2-bit saturating counters
	// States: 0=Strongly Not Taken, 1=Weakly Not Taken,
	//         2=Weakly Taken, 3=Strongly Taken
	bimodal []uint8

	// Gshare: Pattern History Table - 2-bit saturating counters
	// Indexed by (PC XOR global_history)
	gshare []uint8

	// Choice predictor: 2-bit counters that select bimodal vs gshare
	// 0,1 = favor bimodal, 2,3 = favor gshare
	choice []uint8

	// Global history register (shift register of branch outcomes)
	globalHistory uint32
	historyMask   uint32

	// Branch Target Buffer (BTB)
	btb      []btbEntry
	btbValid []bool

	// Configuration
	bhtSize       uint32
	btbSize       uint32
	useTournament bool

	// Statistics
	stats BranchPredictorStats
}

// btbEntry represents an entry in the Branch Target Buffer.
type btbEntry struct {
	pc     uint64 // The PC of the branch instruction
	target uint64 // The target address
}

// NewBranchPredictor creates a new branch predictor with the given configuration.
func NewBranchPredictor(config BranchPredictorConfig) *BranchPredictor {
	bhtSize := config.BHTSize
	btbSize := config.BTBSize
	historyLen := config.GlobalHistoryLength

	// Default sizes if not specified
	if bhtSize == 0 {
		bhtSize = 4096
	}
	if btbSize == 0 {
		btbSize = 512
	}
	if historyLen == 0 {
		historyLen = 12
	}

	historyMask := uint32((1 << historyLen) - 1)

	bp := &BranchPredictor{
		bimodal:       make([]uint8, bhtSize),
		gshare:        make([]uint8, bhtSize),
		choice:        make([]uint8, bhtSize),
		btb:           make([]btbEntry, btbSize),
		btbValid:      make([]bool, btbSize),
		bhtSize:       bhtSize,
		btbSize:       btbSize,
		globalHistory: 0,
		historyMask:   historyMask,
		useTournament: config.UseTournament,
	}

	// Initialize bimodal BHT with weakly taken (2) - biased towards taken
	for i := range bp.bimodal {
		bp.bimodal[i] = 2
	}

	// Initialize gshare PHT with weakly taken (2)
	for i := range bp.gshare {
		bp.gshare[i] = 2
	}

	// Initialize choice with weakly favor gshare (2)
	// Gshare tends to work better for most workloads
	for i := range bp.choice {
		bp.choice[i] = 2
	}

	return bp
}

// bimodalIndex computes the bimodal predictor index for a given PC.
func (bp *BranchPredictor) bimodalIndex(pc uint64) uint32 {
	// Use lower bits of PC (excluding alignment bits)
	return uint32((pc >> 2) & uint64(bp.bhtSize-1))
}

// gshareIndex computes the gshare predictor index (PC XOR global history).
func (bp *BranchPredictor) gshareIndex(pc uint64) uint32 {
	pcBits := uint32((pc >> 2) & uint64(bp.bhtSize-1))
	// XOR PC with global history (extended to match index width)
	history := bp.globalHistory & bp.historyMask
	return (pcBits ^ history) & (bp.bhtSize - 1)
}

// choiceIndex computes the choice predictor index.
func (bp *BranchPredictor) choiceIndex(pc uint64) uint32 {
	// Use same indexing as bimodal for simplicity
	return bp.bimodalIndex(pc)
}

// btbIndex computes the BTB index for a given PC.
func (bp *BranchPredictor) btbIndex(pc uint64) uint32 {
	// Use lower bits of PC (excluding alignment bits)
	return uint32((pc >> 2) & uint64(bp.btbSize-1))
}

// Predict makes a branch prediction for the given PC.
func (bp *BranchPredictor) Predict(pc uint64) Prediction {
	pred := Prediction{}

	// Get predictions from both predictors
	bimodalIdx := bp.bimodalIndex(pc)
	bimodalCounter := bp.bimodal[bimodalIdx]
	bimodalTaken := bimodalCounter >= 2

	gshareIdx := bp.gshareIndex(pc)
	gshareCounter := bp.gshare[gshareIdx]
	gshareTaken := gshareCounter >= 2

	if bp.useTournament {
		// Use choice predictor to select between bimodal and gshare
		choiceIdx := bp.choiceIndex(pc)
		choiceCounter := bp.choice[choiceIdx]
		useGshare := choiceCounter >= 2

		if useGshare {
			pred.Taken = gshareTaken
			bp.stats.TournamentChoseGshare++
		} else {
			pred.Taken = bimodalTaken
			bp.stats.TournamentChoseBimodal++
		}
	} else {
		// Just use bimodal (legacy mode)
		pred.Taken = bimodalTaken
	}

	// Look up BTB for target address
	btbIdx := bp.btbIndex(pc)
	if bp.btbValid[btbIdx] && bp.btb[btbIdx].pc == pc {
		pred.Target = bp.btb[btbIdx].target
		pred.TargetKnown = true
		bp.stats.BTBHits++
	} else {
		bp.stats.BTBMisses++
	}

	bp.stats.Predictions++
	return pred
}

// Update updates the predictor with the actual branch outcome.
func (bp *BranchPredictor) Update(pc uint64, taken bool, target uint64) {
	// Get indices
	bimodalIdx := bp.bimodalIndex(pc)
	gshareIdx := bp.gshareIndex(pc)
	choiceIdx := bp.choiceIndex(pc)

	// Get current predictions
	bimodalCounter := bp.bimodal[bimodalIdx]
	bimodalPredicted := bimodalCounter >= 2
	bimodalCorrect := bimodalPredicted == taken

	gshareCounter := bp.gshare[gshareIdx]
	gsharePredicted := gshareCounter >= 2
	gshareCorrect := gsharePredicted == taken

	// Track which predictor was correct
	if bimodalCorrect {
		bp.stats.BimodalCorrect++
	}
	if gshareCorrect {
		bp.stats.GshareCorrect++
	}

	// Determine if final prediction was correct
	var finalPrediction bool
	if bp.useTournament {
		choiceCounter := bp.choice[choiceIdx]
		useGshare := choiceCounter >= 2
		if useGshare {
			finalPrediction = gsharePredicted
		} else {
			finalPrediction = bimodalPredicted
		}
	} else {
		finalPrediction = bimodalPredicted
	}

	if finalPrediction == taken {
		bp.stats.Correct++
	} else {
		bp.stats.Mispredictions++
	}

	// Update bimodal 2-bit saturating counter
	if taken {
		if bp.bimodal[bimodalIdx] < 3 {
			bp.bimodal[bimodalIdx]++
		}
	} else {
		if bp.bimodal[bimodalIdx] > 0 {
			bp.bimodal[bimodalIdx]--
		}
	}

	// Update gshare 2-bit saturating counter
	if taken {
		if bp.gshare[gshareIdx] < 3 {
			bp.gshare[gshareIdx]++
		}
	} else {
		if bp.gshare[gshareIdx] > 0 {
			bp.gshare[gshareIdx]--
		}
	}

	// Update choice predictor only when predictors disagree
	if bp.useTournament && bimodalCorrect != gshareCorrect {
		if gshareCorrect {
			// Gshare was correct, bimodal was wrong -> favor gshare more
			if bp.choice[choiceIdx] < 3 {
				bp.choice[choiceIdx]++
			}
		} else {
			// Bimodal was correct, gshare was wrong -> favor bimodal more
			if bp.choice[choiceIdx] > 0 {
				bp.choice[choiceIdx]--
			}
		}
	}

	// Update global history (shift in the branch outcome)
	bp.globalHistory = ((bp.globalHistory << 1) | boolToUint32(taken)) & bp.historyMask

	// Update BTB if branch was taken
	if taken {
		btbIdx := bp.btbIndex(pc)
		bp.btb[btbIdx] = btbEntry{
			pc:     pc,
			target: target,
		}
		bp.btbValid[btbIdx] = true
	}
}

// boolToUint32 converts a bool to 0 or 1.
func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// Stats returns the branch predictor statistics.
func (bp *BranchPredictor) Stats() BranchPredictorStats {
	return bp.stats
}

// Reset clears all predictor state and statistics.
func (bp *BranchPredictor) Reset() {
	// Reset bimodal to weakly taken
	for i := range bp.bimodal {
		bp.bimodal[i] = 2
	}

	// Reset gshare to weakly taken
	for i := range bp.gshare {
		bp.gshare[i] = 2
	}

	// Reset choice to weakly favor gshare
	for i := range bp.choice {
		bp.choice[i] = 2
	}

	// Reset global history
	bp.globalHistory = 0

	// Clear BTB
	for i := range bp.btbValid {
		bp.btbValid[i] = false
	}

	// Clear statistics
	bp.stats = BranchPredictorStats{}
}
