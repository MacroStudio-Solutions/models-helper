package rtresolve

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/MacroStudio-Solutions/models-helper/internal/env"
)

func SanitizeError(raw string) string {
	msg := strings.ReplaceAll(strings.ReplaceAll(raw, "\r", " "), "\n", " ")
	msg = strings.TrimSpace(msg)
	msg = strings.TrimPrefix(msg, "✗")
	msg = strings.TrimSpace(msg)
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
}

func ResolveLlamaRuntime(timeout time.Duration) (bool, string) {
	return ResolveRuntime("llama-cpp", timeout)
}

// ResolveRuntime pergunta ao Studio se um runtime de pack esta instalado nesta
// maquina. A falha vira campo, nao codigo de saida: o painel precisa continuar
// renderizando para explicar o que fazer.
func ResolveRuntime(runtimeId string, timeout time.Duration) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, env.StudioBin(), "pack", "resolve-runtime", runtimeId)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := SanitizeError(string(out))
		if msg == "" {
			msg = SanitizeError(err.Error())
		}
		if msg == "" {
			msg = "falha ao resolver o runtime " + runtimeId
		}
		return false, msg
	}
	return true, ""
}
