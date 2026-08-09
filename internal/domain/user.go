package domain

// User is an authenticated actor: an uploader, reviewer, or admin
// within a tenant (SRS Feature: Human Review — "only authorized
// reviewers can act on a review task").
type User struct {
	ID       string
	TenantID string
	Email    string
	Role     string
}
