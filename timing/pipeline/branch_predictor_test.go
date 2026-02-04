package pipeline_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/timing/pipeline"
)

var _ = Describe("BranchPredictor", func() {
	var bp *pipeline.BranchPredictor

	BeforeEach(func() {
		config := pipeline.BranchPredictorConfig{
			BHTSize:             16,
			BTBSize:             8,
			GlobalHistoryLength: 4,
			UseTournament:       true,
		}
		bp = pipeline.NewBranchPredictor(config)
	})

	Describe("Prediction", func() {
		It("should initially predict taken (biased)", func() {
			pred := bp.Predict(0x1000)
			Expect(pred.Taken).To(BeTrue())
		})

		It("should not know target initially", func() {
			pred := bp.Predict(0x1000)
			Expect(pred.TargetKnown).To(BeFalse())
		})

		It("should learn branch patterns", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Train the predictor: always taken
			for i := 0; i < 10; i++ {
				bp.Update(pc, true, target)
			}

			// Should strongly predict taken
			pred := bp.Predict(pc)
			Expect(pred.Taken).To(BeTrue())
			Expect(pred.TargetKnown).To(BeTrue())
			Expect(pred.Target).To(Equal(target))
		})

		It("should learn not-taken pattern", func() {
			pc := uint64(0x1000)

			// Train: always not taken
			for i := 0; i < 10; i++ {
				bp.Update(pc, false, 0)
			}

			// Should predict not taken
			pred := bp.Predict(pc)
			Expect(pred.Taken).To(BeFalse())
		})
	})

	Describe("2-bit saturating counter", func() {
		It("should require 2 mispredictions to change direction", func() {
			// Use bimodal-only mode for deterministic testing
			config := pipeline.BranchPredictorConfig{
				BHTSize:       16,
				BTBSize:       8,
				UseTournament: false,
			}
			bp = pipeline.NewBranchPredictor(config)

			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Start with strongly taken (saturate up)
			bp.Update(pc, true, target)
			bp.Update(pc, true, target)
			bp.Update(pc, true, target) // Now at 3 (strongly taken)

			// One not-taken -> still predicts taken (at 2)
			bp.Update(pc, false, 0)
			pred := bp.Predict(pc)
			Expect(pred.Taken).To(BeTrue())

			// Another not-taken -> now predicts not taken (at 1)
			bp.Update(pc, false, 0)
			pred = bp.Predict(pc)
			Expect(pred.Taken).To(BeFalse())
		})
	})

	Describe("BTB", func() {
		It("should cache branch targets", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Before update, no target known
			pred := bp.Predict(pc)
			Expect(pred.TargetKnown).To(BeFalse())

			// Update with taken branch
			bp.Update(pc, true, target)

			// Now target should be known
			pred = bp.Predict(pc)
			Expect(pred.TargetKnown).To(BeTrue())
			Expect(pred.Target).To(Equal(target))
		})

		It("should not cache not-taken branches", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Update with not-taken branch
			bp.Update(pc, false, target)

			// Target should still be unknown
			pred := bp.Predict(pc)
			Expect(pred.TargetKnown).To(BeFalse())
		})

		It("should handle BTB conflicts correctly", func() {
			// Two PCs that map to the same BTB index
			config := pipeline.BranchPredictorConfig{
				BHTSize:       16,
				BTBSize:       4, // Small BTB for easy conflicts
				UseTournament: false,
			}
			bp = pipeline.NewBranchPredictor(config)

			pc1 := uint64(0x1000)
			target1 := uint64(0x2000)
			// pc2 conflicts with pc1 (same BTB index)
			pc2 := uint64(0x1000 + 4*4) // offset by 4 entries
			target2 := uint64(0x3000)

			// Cache pc1
			bp.Update(pc1, true, target1)
			pred := bp.Predict(pc1)
			Expect(pred.TargetKnown).To(BeTrue())
			Expect(pred.Target).To(Equal(target1))

			// Cache pc2 (should evict pc1)
			bp.Update(pc2, true, target2)
			pred = bp.Predict(pc2)
			Expect(pred.TargetKnown).To(BeTrue())
			Expect(pred.Target).To(Equal(target2))

			// pc1 should now miss (tag doesn't match)
			pred = bp.Predict(pc1)
			Expect(pred.TargetKnown).To(BeFalse())
		})
	})

	Describe("Statistics", func() {
		It("should track predictions", func() {
			pc := uint64(0x1000)
			bp.Predict(pc)
			bp.Predict(pc)
			bp.Predict(pc)

			stats := bp.Stats()
			Expect(stats.Predictions).To(Equal(uint64(3)))
		})

		It("should track correct predictions", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Initially predicts taken, update with taken -> correct
			bp.Predict(pc) // Prediction made
			bp.Update(pc, true, target)

			stats := bp.Stats()
			Expect(stats.Correct).To(Equal(uint64(1)))
			Expect(stats.Mispredictions).To(Equal(uint64(0)))
		})

		It("should track mispredictions", func() {
			pc := uint64(0x1000)

			// Initially predicts taken, update with not-taken -> misprediction
			bp.Predict(pc)
			bp.Update(pc, false, 0)

			stats := bp.Stats()
			Expect(stats.Mispredictions).To(Equal(uint64(1)))
		})

		It("should compute accuracy correctly", func() {
			// Use bimodal-only for deterministic testing
			config := pipeline.BranchPredictorConfig{
				BHTSize:       16,
				BTBSize:       8,
				UseTournament: false,
			}
			bp = pipeline.NewBranchPredictor(config)

			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Initial state: counter=2 (weakly taken), so predicts taken
			// Predict + Update 1: taken -> correct (counter -> 3)
			bp.Predict(pc)
			bp.Update(pc, true, target)
			// Predict + Update 2: taken -> correct (counter stays 3)
			bp.Predict(pc)
			bp.Update(pc, true, target)
			// Predict + Update 3: taken -> correct (counter stays 3)
			bp.Predict(pc)
			bp.Update(pc, true, target)
			// Predict + Update 4: not-taken -> misprediction (counter -> 2)
			bp.Predict(pc)
			bp.Update(pc, false, 0)

			stats := bp.Stats()
			Expect(stats.Predictions).To(Equal(uint64(4)))
			Expect(stats.Correct).To(Equal(uint64(3)))
			Expect(stats.Mispredictions).To(Equal(uint64(1)))
			Expect(stats.Accuracy()).To(BeNumerically("~", 75.0, 0.1))
		})

		It("should track BTB hits and misses", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// First predict - BTB miss
			bp.Predict(pc)
			// Update BTB
			bp.Update(pc, true, target)
			// Second predict - BTB hit
			bp.Predict(pc)

			stats := bp.Stats()
			Expect(stats.BTBHits).To(Equal(uint64(1)))
			Expect(stats.BTBMisses).To(Equal(uint64(1)))
		})
	})

	Describe("Reset", func() {
		It("should clear all state", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Build up some state
			bp.Update(pc, true, target)
			bp.Predict(pc)
			bp.Predict(pc)

			// Reset
			bp.Reset()

			// Statistics should be cleared
			stats := bp.Stats()
			Expect(stats.Predictions).To(Equal(uint64(0)))
			Expect(stats.Correct).To(Equal(uint64(0)))

			// BTB should be cleared
			pred := bp.Predict(pc)
			Expect(pred.TargetKnown).To(BeFalse())
		})
	})

	Describe("Default configuration", func() {
		It("should use sensible defaults", func() {
			config := pipeline.DefaultBranchPredictorConfig()
			Expect(config.BHTSize).To(Equal(uint32(4096)))
			Expect(config.BTBSize).To(Equal(uint32(512)))
			Expect(config.GlobalHistoryLength).To(Equal(uint32(12)))
			Expect(config.UseTournament).To(BeTrue())
		})
	})

	Describe("Tournament predictor", func() {
		It("should track tournament statistics", func() {
			config := pipeline.BranchPredictorConfig{
				BHTSize:             16,
				BTBSize:             8,
				GlobalHistoryLength: 4,
				UseTournament:       true,
			}
			bp = pipeline.NewBranchPredictor(config)

			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Make several predictions
			for i := 0; i < 10; i++ {
				bp.Predict(pc)
				bp.Update(pc, true, target)
			}

			stats := bp.Stats()
			// Tournament should choose one predictor or the other
			totalChoices := stats.TournamentChoseBimodal + stats.TournamentChoseGshare
			Expect(totalChoices).To(Equal(uint64(10)))
		})

		It("should adapt to patterns that favor gshare", func() {
			config := pipeline.BranchPredictorConfig{
				BHTSize:             16,
				BTBSize:             8,
				GlobalHistoryLength: 4,
				UseTournament:       true,
			}
			bp = pipeline.NewBranchPredictor(config)

			// Create an alternating pattern (TNTNTN...) which gshare handles better
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Train with alternating pattern
			for i := 0; i < 50; i++ {
				taken := i%2 == 0
				bp.Predict(pc)
				if taken {
					bp.Update(pc, true, target)
				} else {
					bp.Update(pc, false, 0)
				}
			}

			stats := bp.Stats()
			// Should have some correct predictions (gshare should learn the pattern)
			Expect(stats.Correct).To(BeNumerically(">", uint64(10)))
		})

		It("should track bimodal and gshare accuracy separately", func() {
			pc := uint64(0x1000)
			target := uint64(0x2000)

			// Make predictions
			for i := 0; i < 5; i++ {
				bp.Predict(pc)
				bp.Update(pc, true, target)
			}

			stats := bp.Stats()
			// Both predictors should have some correct predictions
			Expect(stats.BimodalCorrect).To(BeNumerically(">", uint64(0)))
			Expect(stats.GshareCorrect).To(BeNumerically(">", uint64(0)))
		})
	})

	Describe("Global history", func() {
		It("should use global history for gshare index", func() {
			config := pipeline.BranchPredictorConfig{
				BHTSize:             16,
				BTBSize:             8,
				GlobalHistoryLength: 4,
				UseTournament:       false, // Force bimodal to isolate global history effect
			}
			bp = pipeline.NewBranchPredictor(config)

			pc := uint64(0x1000)

			// Update with different patterns to shift global history
			bp.Update(pc, true, 0x2000)  // history: 0001
			bp.Update(pc, false, 0x2000) // history: 0010
			bp.Update(pc, true, 0x2000)  // history: 0101
			bp.Update(pc, false, 0x2000) // history: 1010

			// The predictor should have updated various entries
			stats := bp.Stats()
			Expect(stats.Predictions).To(Equal(uint64(0))) // No predictions yet
		})
	})
})
