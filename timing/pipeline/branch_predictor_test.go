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
			BHTSize: 16,
			BTBSize: 8,
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
				BHTSize: 16,
				BTBSize: 4, // Small BTB for easy conflicts
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
			Expect(config.BHTSize).To(Equal(uint32(1024)))
			Expect(config.BTBSize).To(Equal(uint32(256)))
		})
	})
})
