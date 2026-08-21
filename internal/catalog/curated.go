package catalog

import (
	"context"

	"github.com/MacroStudio-Solutions/models-helper/internal/contract"
)

// A vitrine.
//
// A lista por popularidade da API publica nao serve como recomendacao: ela
// mistura pesos de 70B, modelos de imagem e conversoes abandonadas, e numa
// maquina comum a maioria do que ela devolve nao roda. Esta lista e editorial —
// modelos de instrucao conhecidos, com peso que cabe em maquina de trabalho — e
// a ordem daqui e a preferencia de quem escreveu, usada como desempate depois
// que o veredito de viabilidade ordena.
//
// A busca aberta continua existindo e continua sendo popularidade: quem procura
// um modelo especifico nao quer a opiniao de ninguem.

type TCuratedModel struct {
	RepoId  string
	Summary string
}

var curated = []TCuratedModel{
	{"unsloth/Qwen3-4B-Instruct-2507-GGUF", "Equilíbrio entre qualidade e tamanho. É o ponto de partida para uma máquina de trabalho comum."},
	{"bartowski/Llama-3.2-3B-Instruct-GGUF", "Leve e rápido, bom em instrução curta e em resposta objetiva."},
	{"ggml-org/gemma-3-4b-it-GGUF", "Gemma 3 do Google, um dos melhores desse tamanho em português."},
	{"unsloth/Qwen3-1.7B-GGUF", "O menor que ainda conversa bem. Para máquina apertada de memória."},
	{"bartowski/microsoft_Phi-4-mini-instruct-GGUF", "Phi-4 mini da Microsoft, forte em raciocínio para o tamanho que tem."},
	{"ggml-org/SmolLM3-3B-GGUF", "Compacto e recente, pensado para rodar fora de servidor."},
	{"ggml-org/Qwen3-8B-GGUF", "Um degrau acima em qualidade. Pede bem mais memória disponível."},
	{"bartowski/Mistral-7B-Instruct-v0.3-GGUF", "Clássico confiável de 7B, com muito material escrito sobre ele."},
	{"unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF", "Especialista em código. Só vale em máquina com bastante memória."},
}

func CuratedRepos() []TCuratedModel {
	return curated
}

// CuratedEntries resolve a vitrine contra a maquina: peso real por repositorio,
// veredito calculado e a ordem editorial preservada dentro de cada veredito.
func CuratedEntries(ctx context.Context, c *TClient, machineProfile contract.TMachineProfile, modelsDir string) []contract.TCatalogEntry {
	models := make([]TModel, 0, len(curated))
	for _, item := range curated {
		models = append(models, TModel{Id: item.RepoId})
	}
	entries := DefaultEntries(ctx, c, models, machineProfile, modelsDir)
	summaries := make(map[string]string, len(curated))
	for _, item := range curated {
		summaries[item.RepoId] = item.Summary
	}
	for i := range entries {
		entries[i].Engine = "llama"
		entries[i].Summary = summaries[entries[i].RepoId]
	}
	return entries
}
