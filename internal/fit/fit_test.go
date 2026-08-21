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

func TestRankAndLabelFollowTheVerdict(t *testing.T) {
	const gib = uint64(1073741824)

	gpu := Compute(32*gib, 24*gib, 12*gib, 4*gib)
	if gpu.FitRank != 0 || gpu.FitLabel != "roda na GPU" {
		t.Fatalf("veredito de GPU: %+v", gpu)
	}

	ok := Compute(32*gib, 24*gib, 0, 4*gib)
	if ok.FitRank != 1 || ok.FitLabel != "roda bem" {
		t.Fatalf("veredito folgado: %+v", ok)
	}

	tight := Compute(16*gib, 4*gib, 0, 8*gib)
	if tight.FitRank != 2 || tight.FitLabel != "no limite" {
		t.Fatalf("veredito no limite: %+v", tight)
	}

	no := Compute(8*gib, 4*gib, 0, 60*gib)
	if no.FitRank != 3 || no.FitLabel != "não recomendado" {
		t.Fatalf("veredito negativo: %+v", no)
	}
	if no.RequiredLabel == "" {
		t.Fatalf("rotulo de memoria necessaria vazio: %+v", no)
	}
}

// A margem de fala e o que separa um modelo de transcricao de um modelo de
// linguagem: com a margem de KV-cache do llama.cpp, o large-v3-turbo seria
// reprovado numa maquina que o roda sem esforco.
func TestSpeechMarginIsSmallerThanTheLanguageMargin(t *testing.T) {
	const gib = uint64(1073741824)
	const turbo = uint64(1624555275)

	language := Compute(8*gib, 7*gib/2, 0, turbo)
	speech := Speech(8*gib, 7*gib/2, turbo)

	if speech.RequiredBytes >= language.RequiredBytes {
		t.Fatalf("fala deveria exigir menos: fala=%d linguagem=%d", speech.RequiredBytes, language.RequiredBytes)
	}
	if !speech.FitOk {
		t.Fatalf("large-v3-turbo deveria caber em 3,5 GiB livres pela formula de fala: %+v", speech)
	}
	if language.FitOk {
		t.Fatalf("a margem de KV-cache deveria reprovar o mesmo peso na mesma maquina: %+v", language)
	}
	if speech.FitGpu {
		t.Fatalf("a formula de fala nunca promete GPU: %+v", speech)
	}
}
