package importer

type UploadResponse struct {
	ImportResult ImportResult `json:"import_result"`
}

type ImportLogsResponse struct {
	Logs []ImportLog `json:"logs"`
}
