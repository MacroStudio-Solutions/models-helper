package catalog

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var quantTokenRe = regexp.MustCompile(`(?i)\b(IQ|Q)\d+(?:_[A-Z0-9]+)+\b|\b(BF16|F16|F32|F8)\b`)

var quantPreference = []string{
	"Q4_K_M", "Q4_K_S", "Q5_K_M", "Q4_0", "IQ4_XS", "Q6_K", "Q8_0",
	"Q5_K_S", "Q3_K_M", "Q4_K_L", "IQ5_XS", "IQ4_NL", "Q2_K", "Q3_K_L",
	"Q3_K_S", "IQ3_XXS", "Q3_0", "Q2_K_S", "IQ2_M", "BF16", "F16", "F32",
}

func QuantToken(fileName string) string {
	base := filepath.Base(fileName)
	match := quantTokenRe.FindString(strings.TrimSuffix(base, ".gguf"))
	if match == "" {
		return ""
	}
	return strings.ToUpper(match)
}

func quantRank(token string) int {
	for i, pref := range quantPreference {
		if pref == token {
			return i
		}
	}
	return len(quantPreference)
}

type TTreeFile struct {
	Path string
	Size uint64
}

func PickDefaultFile(files []TTreeFile) (TTreeFile, bool) {
	if len(files) == 0 {
		return TTreeFile{}, false
	}
	byToken := make(map[string]TTreeFile)
	for _, f := range files {
		token := QuantToken(f.Path)
		if _, exists := byToken[token]; !exists {
			byToken[token] = f
		}
	}
	for _, pref := range quantPreference {
		if f, exists := byToken[pref]; exists {
			return f, true
		}
	}
	rest := append([]TTreeFile(nil), files...)
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Size != rest[j].Size {
			return rest[i].Size < rest[j].Size
		}
		return rest[i].Path < rest[j].Path
	})
	return rest[0], true
}

func SortBySize(files []TTreeFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Size != files[j].Size {
			return files[i].Size < files[j].Size
		}
		return files[i].Path < files[j].Path
	})
}

func DisplayName(repoId string, token string) string {
	short := repoId
	if i := strings.LastIndex(repoId, "/"); i >= 0 {
		short = repoId[i+1:]
	}
	short = strings.TrimSuffix(strings.TrimSuffix(short, "-GGUF"), "-gguf")
	if token != "" {
		return short + " " + token
	}
	return short
}
