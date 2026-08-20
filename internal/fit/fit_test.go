package fit

import "testing"

const gib = uint64(1073741824)

func TestSmallModelFits(t *testing.T) {
	f := Compute(16*gib, 8*gib, 0, 2500000000)
	if !f.FitOk || f.FitTight {
		t.Fatalf("esperado fitOk, obtido %+v", f)
	}
	if f.FitGpu {
		t.Fatalf("sem vram nao pode ser fitGpu")
	}
	if f.RequiredBytes != 4610612736 {
		t.Fatalf("requiredBytes %d != 4610612736", f.RequiredBytes)
	}
}

func TestHugeModelDoesNotFit(t *testing.T) {
	f := Compute(16*gib, 8*gib, 0, 18600000000)
	if f.FitOk || f.FitTight {
		t.Fatalf("esperado sem fit, obtido %+v", f)
	}
}

func TestTightZone(t *testing.T) {
	f := Compute(16*gib, 4*gib, 0, 8000000000)
	if f.FitOk || !f.FitTight {
		t.Fatalf("esperado tight, obtido %+v", f)
	}
}

func TestFitGpuBoundary(t *testing.T) {
	if !Compute(16*gib, 8*gib, 1150, 1000).FitGpu {
		t.Fatalf("vram == 1.15x deve ser fitGpu")
	}
	if Compute(16*gib, 8*gib, 1149, 1000).FitGpu {
		t.Fatalf("vram < 1.15x nao deve ser fitGpu")
	}
}

func TestSizeGb(t *testing.T) {
	if SizeGb(gib) != "1.0" {
		t.Fatalf("SizeGb(1GiB) = %s", SizeGb(gib))
	}
	if SizeGb(0) != "0.0" {
		t.Fatalf("SizeGb(0) = %s", SizeGb(0))
	}
}
