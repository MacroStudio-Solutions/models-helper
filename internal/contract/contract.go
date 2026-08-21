package contract

import "fmt"

const SchemaVersion = 1

const (
	JobStateRunning   = "running"
	JobStateCompleted = "completed"
	JobStateCancelled = "cancelled"
	JobStateFailed    = "failed"
)

type THelperEnvelope[TData any] struct {
	SchemaVersion int           `json:"schemaVersion"`
	Ok            bool          `json:"ok"`
	Data          *TData        `json:"data,omitempty"`
	Error         *THelperError `json:"error,omitempty"`
}

type THelperError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *THelperError) Error() string {
	return e.Code + ": " + e.Message
}

func Errorf(code string, format string, args ...any) *THelperError {
	return &THelperError{Code: code, Message: fmt.Sprintf(format, args...)}
}

type TMachineProfile struct {
	RamTotalBytes           uint64 `json:"ramTotalBytes"`
	RamAvailableBytes       uint64 `json:"ramAvailableBytes"`
	CpuCores                int    `json:"cpuCores"`
	HasGpu                  bool   `json:"hasGpu"`
	GpuName                 string `json:"gpuName"`
	VramBytes               uint64 `json:"vramBytes"`
	HasVulkan               bool   `json:"hasVulkan"`
	VulkanUnavailableReason string `json:"vulkanUnavailableReason"`
	// Rotulos prontos. O painel nao formata numero, entao a alternativa a
	// estes campos e a tela imprimir "25200041984 bytes".
	RamTotalLabel     string `json:"ramTotalLabel"`
	RamAvailableLabel string `json:"ramAvailableLabel"`
	VramLabel         string `json:"vramLabel"`
	CpuLabel          string `json:"cpuLabel"`
	GpuLabel          string `json:"gpuLabel"`
}

type TModelFit struct {
	FitOk         bool   `json:"fitOk"`
	FitTight      bool   `json:"fitTight"`
	FitGpu        bool   `json:"fitGpu"`
	RequiredBytes uint64 `json:"requiredBytes"`
	// FitRank ordena o veredito sem que o consumidor compare numero nem
	// encadeie tres condicoes: 0 GPU, 1 folgado, 2 no limite, 3 nao cabe.
	FitRank  int    `json:"fitRank"`
	FitLabel string `json:"fitLabel"`
	// FitTone e o nome do tom visual do veredito. Existe porque o bloco de
	// condicao do painel so avalia verdadeiro ou falso de um token: sem um tom
	// pronto, colorir um selo custa tres condicoes encadeadas por linha.
	FitTone       string `json:"fitTone"`
	RequiredLabel string `json:"requiredLabel"`
}

const (
	FitRankGpu   = 0
	FitRankOk    = 1
	FitRankTight = 2
	FitRankNo    = 3
)

type TCatalogEntry struct {
	TModelFit
	Name         string        `json:"name"`
	RepoId       string        `json:"repoId"`
	File         string        `json:"file"`
	Quantization string        `json:"quantization"`
	SizeBytes    uint64        `json:"sizeBytes"`
	SizeGb       string        `json:"sizeGb"`
	SizeLabel    string        `json:"sizeLabel"`
	Installed    bool          `json:"installed"`
	Download     *TDownloadJob `json:"download"`
	// Preenchidos apenas pelos catalogos curados. Vazios no catalogo remoto,
	// onde nao ha nada editorial a afirmar sobre um modelo qualquer.
	Engine      string `json:"engine"`
	EngineLabel string `json:"engineLabel"`
	Summary     string `json:"summary"`
	Recommended bool   `json:"recommended"`
}

type TInstalledModel struct {
	TModelFit
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes uint64 `json:"sizeBytes"`
	SizeGb    string `json:"sizeGb"`
	SizeLabel string `json:"sizeLabel"`
	// ApiName e o nome pelo qual o servidor em modo roteador enderaca este
	// arquivo: o nome do arquivo sem a extensao. E o que o agente configura.
	ApiName     string `json:"apiName"`
	Engine      string `json:"engine"`
	EngineLabel string `json:"engineLabel"`
	IsDefault   bool   `json:"isDefault"`
	IsLoaded    bool   `json:"isLoaded"`
	// CanServe diz se o servidor HTTP consegue carregar este peso. O parakeet
	// tem binario proprio e nao carrega no whisper-server, entao oferecer o
	// botao de servir para ele seria oferecer um comando que nao roda.
	CanServe bool `json:"canServe"`
}

type TDownloadJob struct {
	JobId         string `json:"jobId"`
	RepoId        string `json:"repoId"`
	File          string `json:"file"`
	Destination   string `json:"destination"`
	State         string `json:"state"`
	ReceivedBytes uint64 `json:"receivedBytes"`
	TotalBytes    uint64 `json:"totalBytes"`
	Percent       int    `json:"percent"`
	Pid           int    `json:"pid"`
	StartedAt     string `json:"startedAt"`
	UpdatedAt     string `json:"updatedAt"`
	Error         string `json:"error"`
	ReceivedLabel string `json:"receivedLabel"`
	TotalLabel    string `json:"totalLabel"`
	ProgressLabel string `json:"progressLabel"`
}

type TRuntimeHealth struct {
	Ok                   bool   `json:"ok"`
	Error                string `json:"error"`
	RecommendedRuntimeId string `json:"recommendedRuntimeId"`
	RecommendationReason string `json:"recommendationReason"`
}

type TServerState struct {
	Online  bool   `json:"online"`
	ModelId string `json:"modelId"`
	BaseUrl string `json:"baseUrl"`
	// Modo roteador: o servidor sobe sem modelo fixo e carrega sob demanda o
	// que a requisicao nomear, entao "o modelo em execucao" deixa de ser uma
	// pergunta com uma resposta so.
	Mode        string         `json:"mode"`
	Models      []TServerModel `json:"models"`
	ModelCount  int            `json:"modelCount"`
	LoadedCount int            `json:"loadedCount"`
	HasDefault  bool           `json:"hasDefault"`
	DefaultName string         `json:"defaultName"`
}

type TServerModel struct {
	Id         string `json:"id"`
	Loaded     bool   `json:"loaded"`
	StateLabel string `json:"stateLabel"`
	IsDefault  bool   `json:"isDefault"`
	FromPreset bool   `json:"fromPreset"`
}

type TLocalModelsStatus struct {
	Runtime      TRuntimeHealth    `json:"runtime"`
	Server       TServerState      `json:"server"`
	Machine      TMachineProfile   `json:"machine"`
	Installed    []TInstalledModel `json:"installed"`
	HasInstalled bool              `json:"hasInstalled"`
	Catalog      []TCatalogEntry   `json:"catalog"`
	CatalogError string            `json:"catalogError"`
}

type TTranscriptionServer struct {
	Online       bool   `json:"online"`
	BaseUrl      string `json:"baseUrl"`
	InferenceUrl string `json:"inferenceUrl"`
	ModelName    string `json:"modelName"`
	ModelPath    string `json:"modelPath"`
	HasModelName bool   `json:"hasModelName"`
}

type TTranscriptionStatus struct {
	Runtime      TRuntimeHealth       `json:"runtime"`
	Server       TTranscriptionServer `json:"server"`
	Machine      TMachineProfile      `json:"machine"`
	Installed    []TInstalledModel    `json:"installed"`
	HasInstalled bool                 `json:"hasInstalled"`
	Catalog      []TCatalogEntry      `json:"catalog"`
	CatalogError string               `json:"catalogError"`
	// A recomendacao e editorial e medida, nao derivada de tamanho: o parakeet
	// acertou a mesma transcricao do large-v3-turbo em uma fracao do tempo.
	Recommended     string `json:"recommended"`
	RecommendedFile string `json:"recommendedFile"`
	HasRecommended  bool   `json:"hasRecommended"`
	// HasServable separa "nenhum modelo baixado" de "nenhum modelo que o
	// servidor consiga carregar" — com so um parakeet no disco, as duas frases
	// sao diferentes e a segunda e a util.
	HasServable bool `json:"hasServable"`
}
