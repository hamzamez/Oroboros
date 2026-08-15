package gauntlet

import "testing"

func TestGeneratedStencilMatchesHandWritten(t *testing.T) {
	want := WindowSum(vecA)
	if got := GenStencil(vecA); got != want {
		t.Errorf("GenStencil = %v, want %v", got, want)
	}
	if got := WindowSumMaterialised(vecA); got != want {
		t.Errorf("WindowSumMaterialised = %v, want %v", got, want)
	}
}

func BenchmarkG8WindowSum(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = WindowSum(vecA)
	}
}

func BenchmarkG8GenStencil(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenStencil(vecA)
	}
}

func BenchmarkG8WindowSumMaterialised(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = WindowSumMaterialised(vecA)
	}
}
