package sysinfo

import (
	"testing"
)

func TestDefaultOSReserve(t *testing.T) {
	reserve := defaultOSReserve()
	if reserve < 0 {
		t.Errorf("expected positive OS reserve, got %f", reserve)
	}
}

func TestRoundDownContext(t *testing.T) {
	tests := []struct {
		ctx  int
		want int
	}{
		{1000000, 1000000},
		{600000, 512000},
		{300000, 262144},
		{150000, 131072},
		{70000, 65536},
		{40000, 32768},
		{20000, 16384},
		{10000, 8192},
		{5000, 4096},
		{2000, 4096},
	}
	for _, tt := range tests {
		got := roundDownContext(tt.ctx)
		if got != tt.want {
			t.Errorf("roundDownContext(%d) = %d, want %d", tt.ctx, got, tt.want)
		}
	}
}

func TestCalcMaxContext_Fallback(t *testing.T) {
	ctx := CalcMaxContext(32, 131072, 4.28)
	if ctx < 4096 || ctx > 131072 {
		t.Errorf("ctx = %d outside expected range", ctx)
	}
}

func TestCalcMaxContext_ZeroModelMax(t *testing.T) {
	ctx := CalcMaxContext(32, 0, 4.28)
	if ctx != 131072 {
		t.Errorf("expected 131072 fallback, got %d", ctx)
	}
}
