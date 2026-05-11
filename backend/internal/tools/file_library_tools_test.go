package tools

import "testing"

func TestFileSummarizeToolValidate(t *testing.T) {
	tool := NewFileSummarizeTool(nil, "")

	if err := tool.Validate([]byte(`{"library_file_ids":["f1"]}`)); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
	if err := tool.Validate([]byte(`{"library_file_ids":[]}`)); err == nil {
		t.Fatalf("expected error when library_file_ids is empty")
	}
}

func TestFileCompareToolValidate(t *testing.T) {
	tool := NewFileCompareTool(nil, "")

	if err := tool.Validate([]byte(`{"library_file_ids":["f1","f2"]}`)); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
	if err := tool.Validate([]byte(`{"library_file_ids":["f1"]}`)); err == nil {
		t.Fatalf("expected error when fewer than two ids are provided")
	}
}

func TestFileDeleteToolValidate(t *testing.T) {
	tool := NewFileDeleteTool(nil, "")

	if err := tool.Validate([]byte(`{"library_file_id":"abc"}`)); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
	if err := tool.Validate([]byte(`{}`)); err == nil {
		t.Fatalf("expected error when library_file_id is missing")
	}
}

func TestFileReindexToolValidate(t *testing.T) {
	tool := NewFileReindexTool(nil, "")

	if err := tool.Validate([]byte(`{"library_file_id":"abc"}`)); err != nil {
		t.Fatalf("expected valid args, got error: %v", err)
	}
	if err := tool.Validate([]byte(`{"library_file_id":""}`)); err == nil {
		t.Fatalf("expected error when library_file_id is empty")
	}
}
