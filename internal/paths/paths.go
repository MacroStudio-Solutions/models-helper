package paths

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

func ModelsRoot() string {
	if v := os.Getenv("MODELS_HELPER_MODELS_ROOT"); v != "" {
		return filepath.Clean(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".studio", "models")
	}
	return filepath.Join(home, ".studio", "models")
}

func LlamaModelsDir() string {
	return filepath.Join(ModelsRoot(), "llama-cpp")
}

func CacheDir() string {
	return filepath.Join(ModelsRoot(), ".models-helper", "cache")
}

func resolveReal(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	dir := filepath.Dir(p)
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
}

func relWithin(root, target string) (string, bool) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	rootReal := resolveReal(root)
	targetReal := resolveReal(absTarget)
	rel, err := filepath.Rel(rootReal, targetReal)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return rel, true
}

func ResolveWithin(root, target string) (string, *contract.THelperError) {
	rel, ok := relWithin(root, target)
	if !ok {
		return "", contract.Errorf("DEST_OUTSIDE_MODELS_DIR", "caminho %s esta fora do diretorio de modelos %s", target, root)
	}
	if rel == "." {
		return "", contract.Errorf("DEST_OUTSIDE_MODELS_DIR", "caminho %s e o proprio diretorio de modelos", target)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", contract.Errorf("INVALID_PATH", "caminho invalido: %v", err)
	}
	return resolveReal(absTarget), nil
}

func IsStrictlyWithin(root, target string) bool {
	rel, ok := relWithin(root, target)
	return ok && rel != "."
}

func SafeRelFile(file string) *contract.THelperError {
	if file == "" {
		return contract.Errorf("INVALID_FILE_NAME", "arquivo vazio")
	}
	if filepath.IsAbs(file) {
		return contract.Errorf("INVALID_FILE_NAME", "arquivo %s e um caminho absoluto", file)
	}
	clean := filepath.Clean(file)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return contract.Errorf("INVALID_FILE_NAME", "arquivo %s sai do diretorio de destino", file)
	}
	if strings.Contains(file, "\x00") {
		return contract.Errorf("INVALID_FILE_NAME", "arquivo contem bytes invalidos")
	}
	return nil
}

func SafeRepoId(repo string) *contract.THelperError {
	if repo == "" {
		return contract.Errorf("INVALID_REPO_ID", "repositorio vazio")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return contract.Errorf("INVALID_REPO_ID", "identificador %s nao tem a forma org/nome", repo)
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return contract.Errorf("INVALID_REPO_ID", "identificador %s invalido", repo)
		}
		if strings.ContainsAny(part, "\x00\\") {
			return contract.Errorf("INVALID_REPO_ID", "identificador %s contem caracteres invalidos", repo)
		}
	}
	return nil
}
