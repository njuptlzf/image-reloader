package model

// Image represents the docker image.
type Image struct {
	Digest      string `json:"digest"`
	Tag         string `json:"tag"`
	ResourceURL string `json:"resource_url"`
}

// PushEvent represents the harbor push event.
type PushEvent struct {
	Resources  []Image `json:"resources"`
	Repository struct {
		DateCreated  int64  `json:"date_created"`
		Name         string `json:"name"`
		Namespace    string `json:"namespace"`
		RepoFullName string `json:"repo_full_name"`
		RepoType     string `json:"repo_type"`
	} `json:"repository"`
}
