package phab

type SearchConstaints struct {
	Projects []string `json:"projects"`
}

type ManifestSearch struct {
	Constraints SearchConstaints `json:"constraints"`
}
