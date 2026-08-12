package gitrepo

// RepositoryIDs returns the stable IDs of configured local worktrees without
// exposing filesystem paths. The returned slice is a copy and is safe for API
// selection surfaces.
func (s *Service) RepositoryIDs() []string {
	if s == nil || len(s.ids) == 0 {
		return nil
	}
	ids := make([]string, len(s.ids))
	copy(ids, s.ids)
	return ids
}

// HasRepository reports whether a stable local repository ID is in the startup
// allowlist. Callers never provide or receive its filesystem path.
func (s *Service) HasRepository(repositoryID string) bool {
	if s == nil || !ValidRepositoryID(repositoryID) {
		return false
	}
	_, ok := s.repositories[repositoryID]
	return ok
}
