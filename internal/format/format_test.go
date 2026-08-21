package format

import "testing"

func TestBytesUsesBinaryUnitsAndComma(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 MB"},
		{12 * 1024, "12 KB"},
		{147951465, "142 MB"},
		{2497281120, "2,3 GB"},
		{25200041984, "23,5 GB"},
	}
	for _, c := range cases {
		if got := Bytes(c.in); got != c.want {
			t.Fatalf("Bytes(%d) = %q, esperado %q", c.in, got, c.want)
		}
	}
}

func TestTransferWithoutTotalOmitsTheDenominator(t *testing.T) {
	if got := Transfer(1073741824, 0); got != "1,0 GB" {
		t.Fatalf("sem total esperado apenas o recebido, obtido %q", got)
	}
	if got := Transfer(536870912, 1073741824); got != "512 MB de 1,0 GB" {
		t.Fatalf("transferencia formatada errada: %q", got)
	}
}

func TestCoresSingular(t *testing.T) {
	if got := Cores(1); got != "1 núcleo" {
		t.Fatalf("singular errado: %q", got)
	}
	if got := Cores(16); got != "16 núcleos" {
		t.Fatalf("plural errado: %q", got)
	}
}
