package repository

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newGitHubRepositoryBindingTestRepo(t *testing.T) (*GitHubRepositoryBindingRepo, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewGitHubRepositoryBindingRepo(database)
	if err := repo.ready(); err != nil {
		database.Close()
		t.Fatalf("initialize binding repo: %v", err)
	}
	return repo, database
}

func TestGitHubRepositoryBindingRepoRoundTripsAndRebinds(t *testing.T) {
	repo, database := newGitHubRepositoryBindingTestRepo(t)
	defer database.Close()

	binding := GitHubRepositoryBinding{
		LocalRepositoryID:  "omni",
		GitHubUserID:       7,
		GitHubRepositoryID: 1001,
		GitHubFullName:     "octo/one",
		DefaultBranch:      "main",
		Private:            true,
	}
	if err := repo.Upsert("owner-a", binding); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	got, err := repo.Get("owner-a", "omni")
	if err != nil || got == nil {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	if got.GitHubRepositoryID != 1001 || got.GitHubFullName != "octo/one" || !got.Private || got.OwnerID != "owner-a" {
		t.Fatalf("unexpected binding: %#v", got)
	}

	binding.GitHubRepositoryID = 2002
	binding.GitHubFullName = "octo/two"
	binding.Private = false
	binding.Archived = true
	if err := repo.Upsert("owner-a", binding); err != nil {
		t.Fatalf("rebind error = %v", err)
	}
	got, err = repo.Get("owner-a", "omni")
	if err != nil || got == nil || got.GitHubRepositoryID != 2002 || got.GitHubFullName != "octo/two" || !got.Archived {
		t.Fatalf("unexpected rebound binding: %#v, %v", got, err)
	}
}

func TestGitHubRepositoryBindingRepoIsolatesOwners(t *testing.T) {
	repo, database := newGitHubRepositoryBindingTestRepo(t)
	defer database.Close()

	for owner, repositoryID := range map[string]int64{"owner-a": 10, "owner-b": 20} {
		if err := repo.Upsert(owner, GitHubRepositoryBinding{
			LocalRepositoryID:  "omni",
			GitHubUserID:       repositoryID,
			GitHubRepositoryID: repositoryID,
			GitHubFullName:     "octo/repo",
		}); err != nil {
			t.Fatalf("save %s: %v", owner, err)
		}
	}
	if err := repo.Delete("owner-a", "omni"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	first, err := repo.Get("owner-a", "omni")
	if err != nil || first != nil {
		t.Fatalf("owner-a after delete = %#v, %v", first, err)
	}
	second, err := repo.Get("owner-b", "omni")
	if err != nil || second == nil || second.GitHubRepositoryID != 20 {
		t.Fatalf("owner-b was affected: %#v, %v", second, err)
	}
}

func TestGitHubRepositoryBindingRepoListsByLocalRepository(t *testing.T) {
	repo, database := newGitHubRepositoryBindingTestRepo(t)
	defer database.Close()

	for _, id := range []string{"zeta", "alpha"} {
		if err := repo.Upsert("owner", GitHubRepositoryBinding{
			LocalRepositoryID:  id,
			GitHubUserID:       1,
			GitHubRepositoryID: int64(len(id)),
			GitHubFullName:     "octo/" + id,
		}); err != nil {
			t.Fatal(err)
		}
	}
	bindings, err := repo.List("owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 || bindings[0].LocalRepositoryID != "alpha" || bindings[1].LocalRepositoryID != "zeta" {
		t.Fatalf("unexpected binding order: %#v", bindings)
	}
}

func TestGitHubRepositoryBindingRepoValidatesRequiredIdentity(t *testing.T) {
	repo, database := newGitHubRepositoryBindingTestRepo(t)
	defer database.Close()

	valid := GitHubRepositoryBinding{LocalRepositoryID: "omni", GitHubUserID: 1, GitHubRepositoryID: 2, GitHubFullName: "octo/repo"}
	if err := repo.Upsert("", valid); err == nil {
		t.Fatal("Upsert accepted empty owner")
	}
	if err := repo.Upsert("owner", GitHubRepositoryBinding{LocalRepositoryID: "omni", GitHubUserID: 1, GitHubFullName: "octo/repo"}); err == nil {
		t.Fatal("Upsert accepted missing repository ID")
	}
	if _, err := repo.List(""); err == nil {
		t.Fatal("List accepted empty owner")
	}
	if err := repo.Delete("", "omni"); err == nil {
		t.Fatal("Delete accepted empty owner")
	}
}
