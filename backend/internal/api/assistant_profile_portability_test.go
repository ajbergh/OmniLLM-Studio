package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

func newAssistantPortabilityTestRepos(t *testing.T) (*repository.AssistantProfileRepo, *repository.SkillRepo, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.SetMaxOpenConns(1)
	_, err = database.Exec(`
		CREATE TABLE assistant_profiles (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			tool_names_json TEXT NOT NULL DEFAULT '[]',
			skill_ids_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE skills (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			body_markdown TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		_ = database.Close()
		t.Fatalf("create assistant portability schema: %v", err)
	}
	return repository.NewAssistantProfileRepo(database), repository.NewSkillRepo(database), database
}

func requestWithRouteID(request *http.Request, id string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", id)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestAssistantProfilePortableBundleRoundTrip(t *testing.T) {
	profiles, skills, database := newAssistantPortabilityTestRepos(t)
	defer database.Close()
	ownerID := auth.LocalScopeUserID

	skill, err := skills.Save(ownerID, models.Skill{
		Name:         "Incident triage",
		Description:  "Investigate tool failures",
		BodyMarkdown: "# Triage\nInspect evidence before acting.",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("save skill: %v", err)
	}
	profile, err := profiles.Save(ownerID, models.AssistantProfile{
		Name:         "Operations assistant",
		Description:  "Portable operations profile",
		Provider:     "openrouter",
		Model:        "example/model",
		SystemPrompt: "Use the attached triage Skill.",
		ToolNames:    []string{"web_search", "tool_search"},
		SkillIDs:     []string{skill.ID},
	})
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}

	handler := NewAssistantProfileHandler(profiles, skills)
	exportRecorder := httptest.NewRecorder()
	exportRequest := requestWithRouteID(httptest.NewRequest(http.MethodGet, "/v1/assistant-profiles/"+profile.ID+"/export", nil), profile.ID)
	handler.ExportProfile(exportRecorder, exportRequest)
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	for _, forbidden := range []string{"owner_user_id", "workspace_id", "created_at", "updated_at"} {
		if strings.Contains(exportRecorder.Body.String(), forbidden) {
			t.Fatalf("portable bundle leaked local field %q: %s", forbidden, exportRecorder.Body.String())
		}
	}

	var bundle models.AssistantProfileBundle
	if err := json.Unmarshal(exportRecorder.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode export bundle: %v", err)
	}
	if bundle.Schema != models.AssistantProfileBundleSchema || bundle.Version != models.AssistantProfileBundleVersion {
		t.Fatalf("unexpected bundle contract: %#v", bundle)
	}
	if len(bundle.Skills) != 1 || bundle.Skills[0].BodyMarkdown != skill.BodyMarkdown {
		t.Fatalf("exported Skills = %#v", bundle.Skills)
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("encode import bundle: %v", err)
	}
	importRecorder := httptest.NewRecorder()
	importRequest := httptest.NewRequest(http.MethodPost, "/v1/assistant-profiles/import", bytes.NewReader(payload))
	importRequest.Header.Set("Content-Type", "application/json")
	handler.ImportProfile(importRecorder, importRequest)
	if importRecorder.Code != http.StatusCreated {
		t.Fatalf("import status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}

	var imported models.AssistantProfile
	if err := json.Unmarshal(importRecorder.Body.Bytes(), &imported); err != nil {
		t.Fatalf("decode imported profile: %v", err)
	}
	if imported.ID == profile.ID || imported.OwnerUserID != ownerID || imported.WorkspaceID != "" {
		t.Fatalf("imported ownership/id was not regenerated: %#v", imported)
	}
	if imported.Name != profile.Name || imported.SystemPrompt != profile.SystemPrompt || len(imported.SkillIDs) != 1 || imported.SkillIDs[0] == skill.ID {
		t.Fatalf("imported profile did not preserve portable settings: %#v", imported)
	}
	importedSkill, err := skills.Get(ownerID, imported.SkillIDs[0])
	if err != nil || importedSkill == nil {
		t.Fatalf("get imported Skill: item=%#v err=%v", importedSkill, err)
	}
	if importedSkill.BodyMarkdown != skill.BodyMarkdown || importedSkill.OwnerUserID != ownerID || importedSkill.WorkspaceID != "" {
		t.Fatalf("imported Skill mismatch: %#v", importedSkill)
	}
}

func TestAssistantProfileImportRejectsUnknownBundleVersion(t *testing.T) {
	profiles, skills, database := newAssistantPortabilityTestRepos(t)
	defer database.Close()
	handler := NewAssistantProfileHandler(profiles, skills)

	payload, err := json.Marshal(models.AssistantProfileBundle{
		Schema:  models.AssistantProfileBundleSchema,
		Version: 99,
		Profile: models.PortableAssistantProfile{Name: "bad", ToolNames: []string{}},
		Skills:  []models.PortableSkill{},
	})
	if err != nil {
		t.Fatalf("encode unsupported bundle: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/assistant-profiles/import", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ImportProfile(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
}
