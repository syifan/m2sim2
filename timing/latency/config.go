package latency

import (
	"encoding/json"
	"fmt"
	"os"
)

// TimingConfig holds latency values for different instruction types.
// Values are based on Apple M2 microarchitecture estimates.
type TimingConfig struct {
	// ALULatency is the execution latency for basic ALU operations
	// (ADD, SUB, AND, OR, XOR). Default: 1 cycle.
	ALULatency uint64 `json:"alu_latency"`

	// BranchLatency is the base execution latency for branch instructions.
	// This does not include misprediction penalty. Default: 1 cycle.
	BranchLatency uint64 `json:"branch_latency"`

	// BranchMispredictPenalty is the additional cycles lost on branch misprediction.
	// Default: 12 cycles (typical for modern out-of-order cores).
	BranchMispredictPenalty uint64 `json:"branch_mispredict_penalty"`

	// LoadLatency is the latency for load operations assuming L1 cache hit.
	// Default: 4 cycles.
	LoadLatency uint64 `json:"load_latency"`

	// StoreLatency is the latency for store operations (fire-and-forget to LSQ).
	// Default: 1 cycle.
	StoreLatency uint64 `json:"store_latency"`

	// MultiplyLatency is the latency for integer multiply operations.
	// Default: 3 cycles. (For future MUL instruction support)
	MultiplyLatency uint64 `json:"multiply_latency"`

	// DivideLatencyMin is the minimum latency for integer divide operations.
	// Default: 10 cycles. (For future DIV instruction support)
	DivideLatencyMin uint64 `json:"divide_latency_min"`

	// DivideLatencyMax is the maximum latency for integer divide operations.
	// Default: 15 cycles. (For future DIV instruction support)
	DivideLatencyMax uint64 `json:"divide_latency_max"`

	// SyscallLatency is the latency for system call instructions.
	// Default: 1 cycle (handling is external).
	SyscallLatency uint64 `json:"syscall_latency"`

	// SIMDIntLatency is the execution latency for SIMD integer operations
	// (VADD, VSUB, VMUL). Default: 2 cycles.
	SIMDIntLatency uint64 `json:"simd_int_latency"`

	// SIMDFloatLatency is the execution latency for SIMD floating-point operations
	// (VFADD, VFSUB, VFMUL). Default: 3 cycles.
	SIMDFloatLatency uint64 `json:"simd_float_latency"`

	// SIMDLoadLatency is the latency for SIMD load operations (128-bit).
	// Default: 5 cycles (slightly higher than scalar due to wider access).
	SIMDLoadLatency uint64 `json:"simd_load_latency"`

	// SIMDStoreLatency is the latency for SIMD store operations (128-bit).
	// Default: 1 cycle (fire-and-forget to LSQ).
	SIMDStoreLatency uint64 `json:"simd_store_latency"`

	// Note: Memory hierarchy latencies (L1/L2/L3/DRAM) are configured in
	// cache.Config.HitLatency and cache.Config.MissLatency, not here.
	// This table provides instruction execution latencies only.
}

// DefaultTimingConfig returns a TimingConfig with M2-based default values.
func DefaultTimingConfig() *TimingConfig {
	return &TimingConfig{
		ALULatency:              1,
		BranchLatency:           1,
		BranchMispredictPenalty: 14,
		LoadLatency:             4,
		StoreLatency:            1,
		MultiplyLatency:         3,
		DivideLatencyMin:        10,
		DivideLatencyMax:        15,
		SyscallLatency:          1,
		SIMDIntLatency:          2,
		SIMDFloatLatency:        3,
		SIMDLoadLatency:         5,
		SIMDStoreLatency:        1,
	}
}

// LoadConfig loads a TimingConfig from a JSON file.
func LoadConfig(path string) (*TimingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read timing config file: %w", err)
	}

	config := DefaultTimingConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse timing config: %w", err)
	}

	return config, nil
}

// SaveConfig writes a TimingConfig to a JSON file.
func (c *TimingConfig) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize timing config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write timing config file: %w", err)
	}

	return nil
}

// Validate checks that all latency values are valid (> 0).
func (c *TimingConfig) Validate() error {
	if c.ALULatency == 0 {
		return fmt.Errorf("alu_latency must be > 0")
	}
	if c.BranchLatency == 0 {
		return fmt.Errorf("branch_latency must be > 0")
	}
	if c.LoadLatency == 0 {
		return fmt.Errorf("load_latency must be > 0")
	}
	if c.StoreLatency == 0 {
		return fmt.Errorf("store_latency must be > 0")
	}
	if c.SyscallLatency == 0 {
		return fmt.Errorf("syscall_latency must be > 0")
	}
	if c.DivideLatencyMin > c.DivideLatencyMax {
		return fmt.Errorf("divide_latency_min must be <= divide_latency_max")
	}
	return nil
}

// Clone returns a deep copy of the TimingConfig.
func (c *TimingConfig) Clone() *TimingConfig {
	return &TimingConfig{
		ALULatency:              c.ALULatency,
		BranchLatency:           c.BranchLatency,
		BranchMispredictPenalty: c.BranchMispredictPenalty,
		LoadLatency:             c.LoadLatency,
		StoreLatency:            c.StoreLatency,
		MultiplyLatency:         c.MultiplyLatency,
		DivideLatencyMin:        c.DivideLatencyMin,
		DivideLatencyMax:        c.DivideLatencyMax,
		SyscallLatency:          c.SyscallLatency,
		SIMDIntLatency:          c.SIMDIntLatency,
		SIMDFloatLatency:        c.SIMDFloatLatency,
		SIMDLoadLatency:         c.SIMDLoadLatency,
		SIMDStoreLatency:        c.SIMDStoreLatency,
	}
}
