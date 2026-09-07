package htr

import (
	"testing"

	"github.com/klauspost/cpuid/v2"
)

// The hashing libraries choose their code path from the CPUID instruction,
// not from /proc/cpuinfo, and a hypervisor can hide features from one and
// not the other. Log what they see so a runner's path can be read off its
// log next to the consistency test's verdict.
func TestReportHashingCPUPaths(t *testing.T) {
	t.Logf("cpu %q vendor %s", cpuid.CPU.BrandName, cpuid.CPU.VendorString)
	t.Logf("gohashtree path: shani=%v avx512=%v avx2=%v",
		cpuid.CPU.Supports(cpuid.SHA, cpuid.AVX),
		cpuid.CPU.Supports(cpuid.AVX512F, cpuid.AVX512VL),
		cpuid.CPU.Supports(cpuid.AVX2, cpuid.BMI2))
	t.Logf("sha256-simd path: sha=%v avx512=%v",
		cpuid.CPU.Supports(cpuid.SHA, cpuid.SSSE3, cpuid.SSE4),
		cpuid.CPU.Supports(cpuid.AVX512F, cpuid.AVX512DQ, cpuid.AVX512BW, cpuid.AVX512VL))
}
