package pipeline

// BranchPredictorConfig holds configuration for the branch predictor.
type BranchPredictorConfig struct {
	// BHTSize is the number of entries in the Branch History Table.
	// Must be a power of 2. Default is 1024.
	BHTSize uint32
	// BTBSize is the number of entries in the Branch Target Buffer.
	// Must be a power of 2. Default is 256.
	BTBSize uint32
}

// DefaultBranchPredictorConfig returns a default configuration.
func DefaultBranchPredictorConfig() BranchPredictorConfig {
	return BranchPredictorConfig{
		BHTSize: 1024,
		BTBSize: 256,
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

// BranchPredictor implements a 2-bit saturating counter (bimodal) predictor
// with a Branch Target Buffer (BTB).
type BranchPredictor struct {
	// Branch History Table (BHT) - 2-bit saturating counters
	// States: 0=Strongly Not Taken, 1=Weakly Not Taken,
	//         2=Weakly Taken, 3=Strongly Taken
	bht []uint8

	// Branch Target Buffer (BTB)
	// Maps PC to target address
	btb      []btbEntry
	btbValid []bool

	// Configuration
	bhtSize uint32
	btbSize uint32

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

	// Default sizes if not specified
	if bhtSize == 0 {
		bhtSize = 1024
	}
	if btbSize == 0 {
		btbSize = 256
	}

	bp := &BranchPredictor{
		bht:      make([]uint8, bhtSize),
		btb:      make([]btbEntry, btbSize),
		btbValid: make([]bool, btbSize),
		bhtSize:  bhtSize,
		btbSize:  btbSize,
	}

	// Initialize BHT with weakly taken (2) - biased towards taken
	for i := range bp.bht {
		bp.bht[i] = 2
	}

	return bp
}

// bhtIndex computes the BHT index for a given PC.
func (bp *BranchPredictor) bhtIndex(pc uint64) uint32 {
	// Use lower bits of PC (excluding alignment bits)
	return uint32((pc >> 2) & uint64(bp.bhtSize-1))
}

// btbIndex computes the BTB index for a given PC.
func (bp *BranchPredictor) btbIndex(pc uint64) uint32 {
	// Use lower bits of PC (excluding alignment bits)
	return uint32((pc >> 2) & uint64(bp.btbSize-1))
}

// Predict makes a branch prediction for the given PC.
func (bp *BranchPredictor) Predict(pc uint64) Prediction {
	pred := Prediction{}

	// Look up BHT for taken/not-taken prediction
	bhtIdx := bp.bhtIndex(pc)
	counter := bp.bht[bhtIdx]
	pred.Taken = counter >= 2 // Taken if counter is 2 or 3

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
	// Update BHT
	bhtIdx := bp.bhtIndex(pc)
	counter := bp.bht[bhtIdx]

	// Check if prediction was correct
	predicted := counter >= 2
	if predicted == taken {
		bp.stats.Correct++
	} else {
		bp.stats.Mispredictions++
	}

	// Update 2-bit saturating counter
	if taken {
		if counter < 3 {
			bp.bht[bhtIdx] = counter + 1
		}
	} else {
		if counter > 0 {
			bp.bht[bhtIdx] = counter - 1
		}
	}

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

// Stats returns the branch predictor statistics.
func (bp *BranchPredictor) Stats() BranchPredictorStats {
	return bp.stats
}

// Reset clears all predictor state and statistics.
func (bp *BranchPredictor) Reset() {
	// Reset BHT to weakly taken
	for i := range bp.bht {
		bp.bht[i] = 2
	}

	// Clear BTB
	for i := range bp.btbValid {
		bp.btbValid[i] = false
	}

	// Clear statistics
	bp.stats = BranchPredictorStats{}
}
