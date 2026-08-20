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
}

type TModelFit struct {
	FitOk         bool   `json:"fitOk"`
	FitTight      bool   `json:"fitTight"`
	FitGpu        bool   `json:"fitGpu"`
	RequiredBytes uint64 `json:"requiredBytes"`
}

type TCatalogEntry struct {
	TModelFit
	Name         string        `json:"name"`
	RepoId       string        `json:"repoId"`
	File         string        `json:"file"`
	Quantization string        `json:"quantization"`
	SizeBytes    uint64        `json:"sizeBytes"`
	SizeGb       string        `json:"sizeGb"`
	Installed    bool          `json:"installed"`
	Download     *TDownloadJob `json:"download"`
}

type TInstalledModel struct {
	TModelFit
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes uint64 `json:"sizeBytes"`
	SizeGb    string `json:"sizeGb"`
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
