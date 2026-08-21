// Rotulos legiveis para o painel.
//
// O painel do Construtor nao formata nada: um bloco so interpola um token, e o
// Construtor nao tem operadores de numero nem de data. Entao todo valor que uma
// pessoa le precisa chegar pronto, ao lado do valor bruto — que continua
// disponivel para quem consome o envelope por programa.
package format

import (
	"fmt"
	"strings"
)

const (
	kib = 1024
	mib = 1024 * kib
	gib = 1024 * mib
)

// decimalComma troca o separador decimal para a convencao pt-BR, que e a lingua
// dos paineis das duas extensoes.
func decimalComma(value string) string {
	return strings.Replace(value, ".", ",", 1)
}

// Bytes devolve o rotulo curto de um tamanho: "1,5 GB", "148 MB", "12 KB".
// A unidade e binaria (1 GB = 1024 MB), a mesma do campo sizeGb ja existente.
func Bytes(n uint64) string {
	switch {
	case n == 0:
		return "0 MB"
	case n < mib:
		return fmt.Sprintf("%d KB", (n+kib-1)/kib)
	case n < gib:
		return fmt.Sprintf("%d MB", (n+mib-1)/mib)
	default:
		return decimalComma(fmt.Sprintf("%.1f GB", float64(n)/gib))
	}
}

// Transfer descreve o andamento de um download em uma frase so.
func Transfer(received uint64, total uint64) string {
	if total == 0 {
		return Bytes(received)
	}
	return fmt.Sprintf("%s de %s", Bytes(received), Bytes(total))
}

// Cores evita o plural errado no rotulo de processadores.
func Cores(n int) string {
	if n == 1 {
		return "1 núcleo"
	}
	return fmt.Sprintf("%d núcleos", n)
}
