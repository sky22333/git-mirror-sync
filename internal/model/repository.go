package model

// Repository is the unified repository model used across providers.
type Repository struct {
	Name        string
	FullName    string
	Description string
	Private     bool
	CloneURL    string
	HTMLURL     string
	Fork        bool
	Archived    bool
	DefaultBranch string
}

// CreateOptions describes how a target repository should be created.
type CreateOptions struct {
	Name        string
	Description string
	Private     bool
}

// VisibilityPolicy controls how target repository visibility is decided.
type VisibilityPolicy string

const (
	VisibilityPrivate VisibilityPolicy = "private"
	VisibilityPublic  VisibilityPolicy = "public"
	VisibilityFollow  VisibilityPolicy = "follow"
)

// ResolvePrivate returns whether the target repository should be private.
func (p VisibilityPolicy) ResolvePrivate(sourcePrivate bool) bool {
	switch p {
	case VisibilityPublic:
		return false
	case VisibilityFollow:
		return sourcePrivate
	default:
		return true
	}
}
