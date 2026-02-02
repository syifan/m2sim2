package insts_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/insts"
)

var _ = Describe("Insts Package", func() {
	It("should have an Instruction type", func() {
		var i insts.Instruction
		Expect(i).To(BeZero())
	})

	It("should have a Decoder type", func() {
		decoder := insts.NewDecoder()
		Expect(decoder).ToNot(BeNil())
	})
})
