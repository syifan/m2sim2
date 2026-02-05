package emu_test

import (
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sarchlab/m2sim/emu"
)

var _ = Describe("SIMD", func() {
	var (
		simdRegFile *emu.SIMDRegFile
		regFile     *emu.RegFile
		memory      *emu.Memory
		simd        *emu.SIMD
	)

	BeforeEach(func() {
		simdRegFile = emu.NewSIMDRegFile()
		regFile = &emu.RegFile{}
		memory = emu.NewMemory()
		simd = emu.NewSIMD(simdRegFile, regFile, memory)
	})

	Describe("VADD (Vector Integer Add)", func() {
		Context("8-bit elements (16B arrangement)", func() {
			It("should add 16 byte elements", func() {
				// V0 = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16]
				// V1 = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160]
				simdRegFile.WriteQ(0, 0x0807060504030201, 0x100F0E0D0C0B0A09)
				simdRegFile.WriteQ(1, 0x50403020100A0A0A, 0xA09688786860605A)

				simd.VADD(2, 0, 1, emu.Arr16B)

				// Check a few lanes
				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(0x01 + 0x0A)))
				Expect(simdRegFile.ReadLane8(2, 1)).To(Equal(uint8(0x02 + 0x0A)))
			})

			It("should handle overflow correctly (wrapping)", func() {
				simdRegFile.WriteLane8(0, 0, 200)
				simdRegFile.WriteLane8(1, 0, 100)

				simd.VADD(2, 0, 1, emu.Arr16B)

				// 200 + 100 = 300, which wraps to 44 (300 - 256)
				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(44)))
			})
		})

		Context("32-bit elements (4S arrangement)", func() {
			It("should add 4 word elements", func() {
				simdRegFile.WriteLane32(0, 0, 100)
				simdRegFile.WriteLane32(0, 1, 200)
				simdRegFile.WriteLane32(0, 2, 300)
				simdRegFile.WriteLane32(0, 3, 400)

				simdRegFile.WriteLane32(1, 0, 10)
				simdRegFile.WriteLane32(1, 1, 20)
				simdRegFile.WriteLane32(1, 2, 30)
				simdRegFile.WriteLane32(1, 3, 40)

				simd.VADD(2, 0, 1, emu.Arr4S)

				Expect(simdRegFile.ReadLane32(2, 0)).To(Equal(uint32(110)))
				Expect(simdRegFile.ReadLane32(2, 1)).To(Equal(uint32(220)))
				Expect(simdRegFile.ReadLane32(2, 2)).To(Equal(uint32(330)))
				Expect(simdRegFile.ReadLane32(2, 3)).To(Equal(uint32(440)))
			})
		})

		Context("64-bit elements (2D arrangement)", func() {
			It("should add 2 doubleword elements", func() {
				simdRegFile.WriteLane64(0, 0, 0x1234567890ABCDEF)
				simdRegFile.WriteLane64(0, 1, 0xFEDCBA0987654321)
				simdRegFile.WriteLane64(1, 0, 0x0000000000000001)
				simdRegFile.WriteLane64(1, 1, 0x0000000000000002)

				simd.VADD(2, 0, 1, emu.Arr2D)

				Expect(simdRegFile.ReadLane64(2, 0)).To(Equal(uint64(0x1234567890ABCDF0)))
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0xFEDCBA0987654323)))
			})
		})
	})

	Describe("VSUB (Vector Integer Subtract)", func() {
		Context("32-bit elements (4S arrangement)", func() {
			It("should subtract 4 word elements", func() {
				simdRegFile.WriteLane32(0, 0, 100)
				simdRegFile.WriteLane32(0, 1, 200)
				simdRegFile.WriteLane32(0, 2, 300)
				simdRegFile.WriteLane32(0, 3, 400)

				simdRegFile.WriteLane32(1, 0, 10)
				simdRegFile.WriteLane32(1, 1, 20)
				simdRegFile.WriteLane32(1, 2, 30)
				simdRegFile.WriteLane32(1, 3, 40)

				simd.VSUB(2, 0, 1, emu.Arr4S)

				Expect(simdRegFile.ReadLane32(2, 0)).To(Equal(uint32(90)))
				Expect(simdRegFile.ReadLane32(2, 1)).To(Equal(uint32(180)))
				Expect(simdRegFile.ReadLane32(2, 2)).To(Equal(uint32(270)))
				Expect(simdRegFile.ReadLane32(2, 3)).To(Equal(uint32(360)))
			})

			It("should handle underflow correctly (wrapping)", func() {
				simdRegFile.WriteLane32(0, 0, 10)
				simdRegFile.WriteLane32(1, 0, 100)

				simd.VSUB(2, 0, 1, emu.Arr4S)

				// 10 - 100 wraps around: 10 - 100 = -90 = 0xFFFFFF A6 (4294967206) in unsigned
				Expect(simdRegFile.ReadLane32(2, 0)).To(Equal(uint32(0xFFFFFFa6))) // -90 as unsigned
			})
		})
	})

	Describe("VMUL (Vector Integer Multiply)", func() {
		Context("16-bit elements (8H arrangement)", func() {
			It("should multiply 8 halfword elements", func() {
				for i := 0; i < 8; i++ {
					simdRegFile.WriteLane16(0, uint8(i), uint16(i+1))
					simdRegFile.WriteLane16(1, uint8(i), uint16(10))
				}

				simd.VMUL(2, 0, 1, emu.Arr8H)

				Expect(simdRegFile.ReadLane16(2, 0)).To(Equal(uint16(10)))
				Expect(simdRegFile.ReadLane16(2, 1)).To(Equal(uint16(20)))
				Expect(simdRegFile.ReadLane16(2, 7)).To(Equal(uint16(80)))
			})
		})

		Context("32-bit elements (4S arrangement)", func() {
			It("should multiply 4 word elements", func() {
				simdRegFile.WriteLane32(0, 0, 5)
				simdRegFile.WriteLane32(0, 1, 6)
				simdRegFile.WriteLane32(0, 2, 7)
				simdRegFile.WriteLane32(0, 3, 8)

				simdRegFile.WriteLane32(1, 0, 10)
				simdRegFile.WriteLane32(1, 1, 10)
				simdRegFile.WriteLane32(1, 2, 10)
				simdRegFile.WriteLane32(1, 3, 10)

				simd.VMUL(2, 0, 1, emu.Arr4S)

				Expect(simdRegFile.ReadLane32(2, 0)).To(Equal(uint32(50)))
				Expect(simdRegFile.ReadLane32(2, 1)).To(Equal(uint32(60)))
				Expect(simdRegFile.ReadLane32(2, 2)).To(Equal(uint32(70)))
				Expect(simdRegFile.ReadLane32(2, 3)).To(Equal(uint32(80)))
			})
		})
	})

	Describe("VFADD (Vector Floating-Point Add)", func() {
		Context("32-bit floats (4S arrangement)", func() {
			It("should add 4 float32 elements", func() {
				simdRegFile.WriteLane32(0, 0, math.Float32bits(1.5))
				simdRegFile.WriteLane32(0, 1, math.Float32bits(2.5))
				simdRegFile.WriteLane32(0, 2, math.Float32bits(3.5))
				simdRegFile.WriteLane32(0, 3, math.Float32bits(4.5))

				simdRegFile.WriteLane32(1, 0, math.Float32bits(0.5))
				simdRegFile.WriteLane32(1, 1, math.Float32bits(0.5))
				simdRegFile.WriteLane32(1, 2, math.Float32bits(0.5))
				simdRegFile.WriteLane32(1, 3, math.Float32bits(0.5))

				simd.VFADD(2, 0, 1, emu.Arr4S)

				r0 := math.Float32frombits(simdRegFile.ReadLane32(2, 0))
				r1 := math.Float32frombits(simdRegFile.ReadLane32(2, 1))
				r2 := math.Float32frombits(simdRegFile.ReadLane32(2, 2))
				r3 := math.Float32frombits(simdRegFile.ReadLane32(2, 3))

				Expect(r0).To(BeNumerically("~", 2.0, 0.001))
				Expect(r1).To(BeNumerically("~", 3.0, 0.001))
				Expect(r2).To(BeNumerically("~", 4.0, 0.001))
				Expect(r3).To(BeNumerically("~", 5.0, 0.001))
			})
		})

		Context("64-bit doubles (2D arrangement)", func() {
			It("should add 2 float64 elements", func() {
				simdRegFile.WriteLane64(0, 0, math.Float64bits(1.5))
				simdRegFile.WriteLane64(0, 1, math.Float64bits(2.5))

				simdRegFile.WriteLane64(1, 0, math.Float64bits(10.0))
				simdRegFile.WriteLane64(1, 1, math.Float64bits(20.0))

				simd.VFADD(2, 0, 1, emu.Arr2D)

				r0 := math.Float64frombits(simdRegFile.ReadLane64(2, 0))
				r1 := math.Float64frombits(simdRegFile.ReadLane64(2, 1))

				Expect(r0).To(BeNumerically("~", 11.5, 0.0001))
				Expect(r1).To(BeNumerically("~", 22.5, 0.0001))
			})
		})
	})

	Describe("VFSUB (Vector Floating-Point Subtract)", func() {
		It("should subtract 4 float32 elements", func() {
			simdRegFile.WriteLane32(0, 0, math.Float32bits(10.0))
			simdRegFile.WriteLane32(0, 1, math.Float32bits(20.0))
			simdRegFile.WriteLane32(0, 2, math.Float32bits(30.0))
			simdRegFile.WriteLane32(0, 3, math.Float32bits(40.0))

			simdRegFile.WriteLane32(1, 0, math.Float32bits(1.0))
			simdRegFile.WriteLane32(1, 1, math.Float32bits(2.0))
			simdRegFile.WriteLane32(1, 2, math.Float32bits(3.0))
			simdRegFile.WriteLane32(1, 3, math.Float32bits(4.0))

			simd.VFSUB(2, 0, 1, emu.Arr4S)

			r0 := math.Float32frombits(simdRegFile.ReadLane32(2, 0))
			r1 := math.Float32frombits(simdRegFile.ReadLane32(2, 1))
			r2 := math.Float32frombits(simdRegFile.ReadLane32(2, 2))
			r3 := math.Float32frombits(simdRegFile.ReadLane32(2, 3))

			Expect(r0).To(BeNumerically("~", 9.0, 0.001))
			Expect(r1).To(BeNumerically("~", 18.0, 0.001))
			Expect(r2).To(BeNumerically("~", 27.0, 0.001))
			Expect(r3).To(BeNumerically("~", 36.0, 0.001))
		})
	})

	Describe("VFMUL (Vector Floating-Point Multiply)", func() {
		It("should multiply 4 float32 elements", func() {
			simdRegFile.WriteLane32(0, 0, math.Float32bits(2.0))
			simdRegFile.WriteLane32(0, 1, math.Float32bits(3.0))
			simdRegFile.WriteLane32(0, 2, math.Float32bits(4.0))
			simdRegFile.WriteLane32(0, 3, math.Float32bits(5.0))

			simdRegFile.WriteLane32(1, 0, math.Float32bits(10.0))
			simdRegFile.WriteLane32(1, 1, math.Float32bits(10.0))
			simdRegFile.WriteLane32(1, 2, math.Float32bits(10.0))
			simdRegFile.WriteLane32(1, 3, math.Float32bits(10.0))

			simd.VFMUL(2, 0, 1, emu.Arr4S)

			r0 := math.Float32frombits(simdRegFile.ReadLane32(2, 0))
			r1 := math.Float32frombits(simdRegFile.ReadLane32(2, 1))
			r2 := math.Float32frombits(simdRegFile.ReadLane32(2, 2))
			r3 := math.Float32frombits(simdRegFile.ReadLane32(2, 3))

			Expect(r0).To(BeNumerically("~", 20.0, 0.001))
			Expect(r1).To(BeNumerically("~", 30.0, 0.001))
			Expect(r2).To(BeNumerically("~", 40.0, 0.001))
			Expect(r3).To(BeNumerically("~", 50.0, 0.001))
		})
	})

	Describe("VADD (Vector Integer Add) - Halfword Arrangements", func() {
		Context("16-bit elements (4H arrangement - 64-bit)", func() {
			It("should add 4 halfword elements", func() {
				simdRegFile.WriteLane16(0, 0, 100)
				simdRegFile.WriteLane16(0, 1, 200)
				simdRegFile.WriteLane16(0, 2, 300)
				simdRegFile.WriteLane16(0, 3, 400)

				simdRegFile.WriteLane16(1, 0, 10)
				simdRegFile.WriteLane16(1, 1, 20)
				simdRegFile.WriteLane16(1, 2, 30)
				simdRegFile.WriteLane16(1, 3, 40)

				simd.VADD(2, 0, 1, emu.Arr4H)

				Expect(simdRegFile.ReadLane16(2, 0)).To(Equal(uint16(110)))
				Expect(simdRegFile.ReadLane16(2, 1)).To(Equal(uint16(220)))
				Expect(simdRegFile.ReadLane16(2, 2)).To(Equal(uint16(330)))
				Expect(simdRegFile.ReadLane16(2, 3)).To(Equal(uint16(440)))
				// Upper 64 bits should be zeroed for 64-bit arrangement
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0)))
			})
		})

		Context("16-bit elements (8H arrangement - 128-bit)", func() {
			It("should add 8 halfword elements", func() {
				for i := 0; i < 8; i++ {
					simdRegFile.WriteLane16(0, uint8(i), uint16(i*10+100))
					simdRegFile.WriteLane16(1, uint8(i), uint16(i+1))
				}

				simd.VADD(2, 0, 1, emu.Arr8H)

				Expect(simdRegFile.ReadLane16(2, 0)).To(Equal(uint16(101)))
				Expect(simdRegFile.ReadLane16(2, 1)).To(Equal(uint16(112)))
				Expect(simdRegFile.ReadLane16(2, 7)).To(Equal(uint16(178)))
			})
		})
	})

	Describe("VSUB (Vector Integer Subtract) - Byte Arrangements", func() {
		Context("8-bit elements (8B arrangement - 64-bit)", func() {
			It("should subtract 8 byte elements", func() {
				for i := 0; i < 8; i++ {
					simdRegFile.WriteLane8(0, uint8(i), uint8(100+i*10))
					simdRegFile.WriteLane8(1, uint8(i), uint8(i+1))
				}

				simd.VSUB(2, 0, 1, emu.Arr8B)

				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(99)))  // 100-1
				Expect(simdRegFile.ReadLane8(2, 1)).To(Equal(uint8(108))) // 110-2
				Expect(simdRegFile.ReadLane8(2, 7)).To(Equal(uint8(162))) // 170-8
				// Upper 64 bits should be zeroed
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0)))
			})
		})

		Context("8-bit elements (16B arrangement - 128-bit)", func() {
			It("should subtract 16 byte elements", func() {
				for i := 0; i < 16; i++ {
					simdRegFile.WriteLane8(0, uint8(i), uint8(200))
					simdRegFile.WriteLane8(1, uint8(i), uint8(i))
				}

				simd.VSUB(2, 0, 1, emu.Arr16B)

				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(200)))  // 200-0
				Expect(simdRegFile.ReadLane8(2, 5)).To(Equal(uint8(195)))  // 200-5
				Expect(simdRegFile.ReadLane8(2, 15)).To(Equal(uint8(185))) // 200-15
			})
		})
	})

	Describe("VSUB (Vector Integer Subtract) - Halfword Arrangements", func() {
		Context("16-bit elements (4H arrangement - 64-bit)", func() {
			It("should subtract 4 halfword elements", func() {
				simdRegFile.WriteLane16(0, 0, 1000)
				simdRegFile.WriteLane16(0, 1, 2000)
				simdRegFile.WriteLane16(0, 2, 3000)
				simdRegFile.WriteLane16(0, 3, 4000)

				simdRegFile.WriteLane16(1, 0, 100)
				simdRegFile.WriteLane16(1, 1, 200)
				simdRegFile.WriteLane16(1, 2, 300)
				simdRegFile.WriteLane16(1, 3, 400)

				simd.VSUB(2, 0, 1, emu.Arr4H)

				Expect(simdRegFile.ReadLane16(2, 0)).To(Equal(uint16(900)))
				Expect(simdRegFile.ReadLane16(2, 1)).To(Equal(uint16(1800)))
				Expect(simdRegFile.ReadLane16(2, 2)).To(Equal(uint16(2700)))
				Expect(simdRegFile.ReadLane16(2, 3)).To(Equal(uint16(3600)))
				// Upper 64 bits should be zeroed
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0)))
			})
		})

		Context("16-bit elements (8H arrangement - 128-bit)", func() {
			It("should subtract 8 halfword elements", func() {
				for i := 0; i < 8; i++ {
					simdRegFile.WriteLane16(0, uint8(i), uint16(5000))
					simdRegFile.WriteLane16(1, uint8(i), uint16(i*100))
				}

				simd.VSUB(2, 0, 1, emu.Arr8H)

				Expect(simdRegFile.ReadLane16(2, 0)).To(Equal(uint16(5000))) // 5000-0
				Expect(simdRegFile.ReadLane16(2, 5)).To(Equal(uint16(4500))) // 5000-500
				Expect(simdRegFile.ReadLane16(2, 7)).To(Equal(uint16(4300))) // 5000-700
			})
		})
	})

	Describe("VSUB (Vector Integer Subtract) - Doubleword Arrangements", func() {
		Context("64-bit elements (2D arrangement)", func() {
			It("should subtract 2 doubleword elements", func() {
				simdRegFile.WriteLane64(0, 0, 0x1234567890ABCDEF)
				simdRegFile.WriteLane64(0, 1, 0xFEDCBA0987654321)
				simdRegFile.WriteLane64(1, 0, 0x0000000000000001)
				simdRegFile.WriteLane64(1, 1, 0x0000000000000020)

				simd.VSUB(2, 0, 1, emu.Arr2D)

				Expect(simdRegFile.ReadLane64(2, 0)).To(Equal(uint64(0x1234567890ABCDEE)))
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0xFEDCBA0987654301)))
			})
		})
	})

	Describe("VMUL (Vector Integer Multiply) - Byte Arrangements", func() {
		Context("8-bit elements (8B arrangement - 64-bit)", func() {
			It("should multiply 8 byte elements", func() {
				for i := 0; i < 8; i++ {
					simdRegFile.WriteLane8(0, uint8(i), uint8(i+1))
					simdRegFile.WriteLane8(1, uint8(i), uint8(2))
				}

				simd.VMUL(2, 0, 1, emu.Arr8B)

				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(2)))  // 1*2
				Expect(simdRegFile.ReadLane8(2, 1)).To(Equal(uint8(4)))  // 2*2
				Expect(simdRegFile.ReadLane8(2, 7)).To(Equal(uint8(16))) // 8*2
				// Upper 64 bits should be zeroed
				Expect(simdRegFile.ReadLane64(2, 1)).To(Equal(uint64(0)))
			})
		})

		Context("8-bit elements (16B arrangement - 128-bit)", func() {
			It("should multiply 16 byte elements", func() {
				for i := 0; i < 16; i++ {
					simdRegFile.WriteLane8(0, uint8(i), uint8(i+1))
					simdRegFile.WriteLane8(1, uint8(i), uint8(3))
				}

				simd.VMUL(2, 0, 1, emu.Arr16B)

				Expect(simdRegFile.ReadLane8(2, 0)).To(Equal(uint8(3)))   // 1*3
				Expect(simdRegFile.ReadLane8(2, 5)).To(Equal(uint8(18)))  // 6*3
				Expect(simdRegFile.ReadLane8(2, 15)).To(Equal(uint8(48))) // 16*3
			})
		})
	})

	Describe("VFSUB (Vector Floating-Point Subtract) - 2D arrangement", func() {
		It("should subtract 2 float64 elements", func() {
			simdRegFile.WriteLane64(0, 0, math.Float64bits(100.5))
			simdRegFile.WriteLane64(0, 1, math.Float64bits(200.25))

			simdRegFile.WriteLane64(1, 0, math.Float64bits(10.5))
			simdRegFile.WriteLane64(1, 1, math.Float64bits(50.25))

			simd.VFSUB(2, 0, 1, emu.Arr2D)

			r0 := math.Float64frombits(simdRegFile.ReadLane64(2, 0))
			r1 := math.Float64frombits(simdRegFile.ReadLane64(2, 1))

			Expect(r0).To(BeNumerically("~", 90.0, 0.0001))
			Expect(r1).To(BeNumerically("~", 150.0, 0.0001))
		})
	})

	Describe("VFMUL (Vector Floating-Point Multiply) - 2D arrangement", func() {
		It("should multiply 2 float64 elements", func() {
			simdRegFile.WriteLane64(0, 0, math.Float64bits(2.5))
			simdRegFile.WriteLane64(0, 1, math.Float64bits(3.5))

			simdRegFile.WriteLane64(1, 0, math.Float64bits(4.0))
			simdRegFile.WriteLane64(1, 1, math.Float64bits(2.0))

			simd.VFMUL(2, 0, 1, emu.Arr2D)

			r0 := math.Float64frombits(simdRegFile.ReadLane64(2, 0))
			r1 := math.Float64frombits(simdRegFile.ReadLane64(2, 1))

			Expect(r0).To(BeNumerically("~", 10.0, 0.0001))
			Expect(r1).To(BeNumerically("~", 7.0, 0.0001))
		})
	})

	Describe("Vector Load/Store", func() {
		Context("LDR128", func() {
			It("should load 128 bits from memory", func() {
				// Write data to memory
				memory.Write64(0x1000, 0x1234567890ABCDEF)
				memory.Write64(0x1008, 0xFEDCBA0987654321)

				simd.LDR128(0, 0x1000)

				low, high := simdRegFile.ReadQ(0)
				Expect(low).To(Equal(uint64(0x1234567890ABCDEF)))
				Expect(high).To(Equal(uint64(0xFEDCBA0987654321)))
			})
		})

		Context("STR128", func() {
			It("should store 128 bits to memory", func() {
				simdRegFile.WriteQ(0, 0xAAAAAAAAAAAAAAAA, 0xBBBBBBBBBBBBBBBB)

				simd.STR128(0, 0x2000)

				Expect(memory.Read64(0x2000)).To(Equal(uint64(0xAAAAAAAAAAAAAAAA)))
				Expect(memory.Read64(0x2008)).To(Equal(uint64(0xBBBBBBBBBBBBBBBB)))
			})
		})

		Context("Round-trip", func() {
			It("should preserve data through store and load", func() {
				simdRegFile.WriteQ(0, 0x1111222233334444, 0x5555666677778888)

				simd.STR128(0, 0x3000)
				simd.LDR128(1, 0x3000)

				low0, high0 := simdRegFile.ReadQ(0)
				low1, high1 := simdRegFile.ReadQ(1)

				Expect(low1).To(Equal(low0))
				Expect(high1).To(Equal(high0))
			})
		})
	})
})
