// Preset do servidor em modo roteador.
//
// Em modo roteador o servidor enderaca cada modelo pelo nome do arquivo, o que
// quebraria toda configuracao de agente ja escrita contra o nome studio-local.
// Um arquivo de preset resolve as duas coisas ao mesmo tempo: o diretorio
// continua expondo todos os modelos pelos seus nomes, e o preset acrescenta
// studio-local apontando para o que o operador escolheu como padrao.
package preset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
	"github.com/MacroStudio-Solutions/models-helper/internal/paths"
	"github.com/MacroStudio-Solutions/models-helper/internal/server"
)

const ContextSize = 16384

func Path() string {
	return filepath.Join(paths.LlamaModelsDir(), "presets.ini")
}

// Ensure garante que o arquivo exista antes de o servidor subir apontando para
// ele. Um preset vazio e valido: o roteador simplesmente nao acrescenta nada
// ao que o diretorio ja expoe.
func Ensure() (string, *contract.THelperError) {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", contract.Errorf("PRESET_FAILED", "falha ao criar o diretorio de modelos: %v", err)
	}
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if err := os.WriteFile(p, []byte(""), 0644); err != nil {
		return "", contract.Errorf("PRESET_FAILED", "falha ao criar %s: %v", p, err)
	}
	return p, nil
}

type TPreset struct {
	Path      string `json:"path"`
	Alias     string `json:"alias"`
	ModelPath string `json:"modelPath"`
	HasModel  bool   `json:"hasModel"`
}

func Read() TPreset {
	state := TPreset{Path: Path(), Alias: server.DefaultAlias}
	data, err := os.ReadFile(state.Path)
	if err != nil {
		return state
	}
	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inSection = trimmed == "["+server.DefaultAlias+"]"
			continue
		}
		if !inSection {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "model" {
			continue
		}
		state.ModelPath = strings.TrimSpace(value)
		state.HasModel = state.ModelPath != ""
	}
	return state
}

// Set aponta o padrao para um modelo ja instalado. O caminho e conferido contra
// o diretorio de modelos pelo mesmo guarda que protege o download e a remocao:
// este arquivo vira linha de comando de um processo servidor.
func Set(modelPath string) (TPreset, *contract.THelperError) {
	resolved, herr := paths.ResolveWithin(paths.ModelsRoot(), modelPath)
	if herr != nil {
		return TPreset{}, herr
	}
	st, err := os.Stat(resolved)
	if err != nil || st.IsDir() {
		return TPreset{}, contract.Errorf("MODEL_NOT_FOUND", "nenhum modelo em %s", modelPath)
	}
	if _, herr := Ensure(); herr != nil {
		return TPreset{}, herr
	}
	body := fmt.Sprintf("[%s]\nctx-size = %d\nmodel = %s\n", server.DefaultAlias, ContextSize, resolved)
	if err := write(body); err != nil {
		return TPreset{}, contract.Errorf("PRESET_FAILED", "falha ao gravar %s: %v", Path(), err)
	}
	return Read(), nil
}

func Clear() (TPreset, *contract.THelperError) {
	if _, herr := Ensure(); herr != nil {
		return TPreset{}, herr
	}
	if err := write(""); err != nil {
		return TPreset{}, contract.Errorf("PRESET_FAILED", "falha ao limpar %s: %v", Path(), err)
	}
	return Read(), nil
}

func write(body string) error {
	p := Path()
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
