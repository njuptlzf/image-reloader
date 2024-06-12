package model

import (
	"time"
)

// Image represents the docker image.
type Image struct {
	Digest      string `json:"digest"`
	Tag         string `json:"tag"`
	ResourceURL string `json:"resource_url"`
}

// Repository represents the repository details.
type Repository struct {
	DateCreated  int64  `json:"date_created"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	RepoFullName string `json:"repo_full_name"`
	RepoType     string `json:"repo_type"`
}

// Data represents the actual data within the PushEvent.
type Data struct {
	Resources  []Image    `json:"resources"`
	Repository Repository `json:"repository"`
}

// PushEvent represents the harbor push event with its metadata.
type PushEvent struct {
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Source          string    `json:"source"`
	Type            string    `json:"type"`
	Time            time.Time `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	Data            Data      `json:"data"`
}
