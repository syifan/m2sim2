package benchmarks_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/benchmarks"
)

var _ = Describe("SPEC Runner", func() {
	Describe("GetSPECBenchmarks", func() {
		It("should return a list of supported benchmarks", func() {
			benchs := benchmarks.GetSPECBenchmarks()
			Expect(len(benchs)).To(BeNumerically(">=", 3))

			// Check that 557.xz_r is in the list (our primary test benchmark)
			var found bool
			for _, b := range benchs {
				if b.Name == "557.xz_r" {
					found = true
					Expect(b.Description).NotTo(BeEmpty())
					Expect(b.Binary).To(ContainSubstring("xz_r"))
					break
				}
			}
			Expect(found).To(BeTrue(), "557.xz_r should be in benchmark list")
		})
	})

	Describe("NewSPECRunner", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "spec-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		It("should fail if SPEC directory doesn't exist", func() {
			_, err := benchmarks.NewSPECRunner(tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("should succeed with valid SPEC structure", func() {
			// Create minimal SPEC structure
			specDir := filepath.Join(tempDir, "benchmarks", "spec")
			Expect(os.MkdirAll(specDir, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(specDir, "benchspec", "CPU"), 0755)).To(Succeed())

			shrcPath := filepath.Join(specDir, "shrc")
			Expect(os.WriteFile(shrcPath, []byte("# SPEC shrc"), 0644)).To(Succeed())

			runner, err := benchmarks.NewSPECRunner(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})
	})

	Describe("SPECRunner operations", func() {
		var runner *benchmarks.SPECRunner
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "spec-ops-*")
			Expect(err).NotTo(HaveOccurred())

			// Create minimal SPEC structure
			specDir := filepath.Join(tempDir, "benchmarks", "spec")
			Expect(os.MkdirAll(specDir, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(specDir, "benchspec", "CPU"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(specDir, "shrc"), []byte("# shrc"), 0644)).To(Succeed())

			runner, err = benchmarks.NewSPECRunner(tempDir)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		It("should validate SPEC setup", func() {
			err := runner.ValidateSetup()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report no available benchmarks initially", func() {
			available := runner.ListAvailableBenchmarks()
			Expect(available).To(BeEmpty())
		})

		It("should report all benchmarks as missing initially", func() {
			missing := runner.ListMissingBenchmarks()
			Expect(len(missing)).To(Equal(len(benchmarks.GetSPECBenchmarks())))
		})

		It("should detect when a binary exists", func() {
			bench := benchmarks.GetSPECBenchmarks()[0]

			// Initially missing
			Expect(runner.BinaryExists(bench)).To(BeFalse())

			// Create the binary directory and file
			binaryPath := runner.GetBinaryPath(bench)
			Expect(os.MkdirAll(filepath.Dir(binaryPath), 0755)).To(Succeed())
			Expect(os.WriteFile(binaryPath, []byte("binary"), 0755)).To(Succeed())

			// Now should exist
			Expect(runner.BinaryExists(bench)).To(BeTrue())
		})
	})
})

func TestSPECRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SPEC Runner Suite")
}
