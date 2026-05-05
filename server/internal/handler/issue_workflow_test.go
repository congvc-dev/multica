package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateIssueAcceptsProjectCustomWorkflowStatus(t *testing.T) {
	projectID := createWorkflowTestProject(t, "Workflow custom status project")
	insertWorkflowTestStatus(t, projectID, "ready_for_demo", "Ready for demo")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":      "Custom workflow status issue",
		"project_id": projectID,
		"status":     "ready_for_demo",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue custom status: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}
	if created.Status != "ready_for_demo" {
		t.Fatalf("CreateIssue custom status: expected ready_for_demo, got %q", created.Status)
	}
}

func TestCreateProjectSeedsDefaultIssueWorkflow(t *testing.T) {
	projectID := createWorkflowTestProject(t, "Workflow seeded on project create")

	var workflowID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id FROM issue_workflow
		WHERE workspace_id = $1 AND project_id = $2 AND is_default = true
	`, testWorkspaceID, projectID).Scan(&workflowID); err != nil {
		t.Fatalf("load default workflow after project create: %v", err)
	}

	var statusCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM issue_status_def WHERE workflow_id = $1
	`, workflowID).Scan(&statusCount); err != nil {
		t.Fatalf("count default statuses after project create: %v", err)
	}
	if statusCount != len(defaultIssueWorkflowStatuses) {
		t.Fatalf("default status count = %d, want %d", statusCount, len(defaultIssueWorkflowStatuses))
	}
}

func TestUpdateIssueResolvesStatusAfterProjectChange(t *testing.T) {
	sourceProjectID := createWorkflowTestProject(t, "Workflow update source project")
	targetProjectID := createWorkflowTestProject(t, "Workflow update target project")
	insertWorkflowTestStatus(t, targetProjectID, "ready_for_demo", "Ready for demo")
	issueID := createWorkflowTestIssue(t, "Workflow update project and status", sourceProjectID)
	t.Cleanup(func() { deleteTestIssue(t, issueID) })

	w := httptest.NewRecorder()
	req := newRequest("PATCH", "/api/issues/"+issueID, map[string]any{
		"project_id": targetProjectID,
		"status":     "ready_for_demo",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue project/status: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated issue: %v", err)
	}
	if updated.ProjectID == nil || *updated.ProjectID != targetProjectID {
		t.Fatalf("project_id = %v, want %q", updated.ProjectID, targetProjectID)
	}
	if updated.Status != "ready_for_demo" {
		t.Fatalf("status = %q, want ready_for_demo", updated.Status)
	}
}

func TestBatchUpdateIssuesResolvesStatusAfterProjectChange(t *testing.T) {
	sourceProjectID := createWorkflowTestProject(t, "Workflow batch source project")
	targetProjectID := createWorkflowTestProject(t, "Workflow batch target project")
	insertWorkflowTestStatus(t, targetProjectID, "ready_for_demo", "Ready for demo")
	issueID := createWorkflowTestIssue(t, "Workflow batch project and status", sourceProjectID)
	t.Cleanup(func() { deleteTestIssue(t, issueID) })

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates": map[string]any{
			"project_id": targetProjectID,
			"status":     "ready_for_demo",
		},
	})
	testHandler.BatchUpdateIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("BatchUpdateIssues project/status: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode batch update response: %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("updated = %d, want 1", resp.Updated)
	}

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/issues/"+issueID, nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssue after batch update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated issue: %v", err)
	}
	if updated.ProjectID == nil || *updated.ProjectID != targetProjectID {
		t.Fatalf("project_id = %v, want %q", updated.ProjectID, targetProjectID)
	}
	if updated.Status != "ready_for_demo" {
		t.Fatalf("status = %q, want ready_for_demo", updated.Status)
	}
}

func TestIssueAvailableTransitionsReturnsDefaultWorkflow(t *testing.T) {
	issueID := createTestIssue(t, "Workflow default transitions", "todo", "medium")
	t.Cleanup(func() { deleteTestIssue(t, issueID) })

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/issues/"+issueID+"/available-transitions", nil)
	req = withURLParam(req, "id", issueID)
	testHandler.GetIssueAvailableTransitions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetIssueAvailableTransitions: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		IssueID       string `json:"issue_id"`
		CurrentStatus struct {
			Key string `json:"key"`
		} `json:"current_status"`
		Transitions []struct {
			Key string `json:"key"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode transitions: %v", err)
	}
	if resp.IssueID != issueID {
		t.Fatalf("issue_id = %q, want %q", resp.IssueID, issueID)
	}
	if resp.CurrentStatus.Key != "todo" {
		t.Fatalf("current_status.key = %q, want todo", resp.CurrentStatus.Key)
	}
	if len(resp.Transitions) == 0 {
		t.Fatalf("expected default workflow transitions from todo")
	}
	seen := map[string]bool{}
	for _, tr := range resp.Transitions {
		seen[tr.Key] = true
	}
	for _, key := range []string{"backlog", "in_progress", "in_review", "done", "blocked", "cancelled"} {
		if !seen[key] {
			t.Fatalf("expected transition to %q in default workflow, got %+v", key, resp.Transitions)
		}
	}
}

func TestDefaultIssueWorkflowMatchesDocumentedStatuses(t *testing.T) {
	projectID := createWorkflowTestProject(t, "Workflow documentation alignment project")
	seedWorkflowTestDefaultIssueWorkflow(t, projectID)

	var workflowID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id FROM issue_workflow
		WHERE workspace_id = $1 AND project_id = $2 AND is_default = true
	`, testWorkspaceID, projectID).Scan(&workflowID); err != nil {
		t.Fatalf("load default workflow: %v", err)
	}

	rows, err := testPool.Query(context.Background(), `
		SELECT key, description
		FROM issue_status_def
		WHERE workflow_id = $1
		ORDER BY position ASC
	`, workflowID)
	if err != nil {
		t.Fatalf("query default statuses: %v", err)
	}
	defer rows.Close()

	gotDescriptions := map[string]string{}
	var gotOrder []string
	for rows.Next() {
		var key string
		var description string
		if err := rows.Scan(&key, &description); err != nil {
			t.Fatalf("scan status: %v", err)
		}
		gotOrder = append(gotOrder, key)
		gotDescriptions[key] = description
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	wantOrder := []string{"backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("default status count = %d (%v), want %d", len(gotOrder), gotOrder, len(wantOrder))
	}
	for i, want := range wantOrder {
		if gotOrder[i] != want {
			t.Fatalf("status order[%d] = %q, want %q (full order %v)", i, gotOrder[i], want, gotOrder)
		}
	}

	wantDescriptions := map[string]string{
		"backlog":     "Parked before the main workflow starts",
		"todo":        "Ready to start",
		"in_progress": "Work is underway",
		"in_review":   "Awaiting review",
		"done":        "Completed successfully",
		"blocked":     "Stuck on an external factor",
		"cancelled":   "Cancelled",
	}
	for key, want := range wantDescriptions {
		if gotDescriptions[key] != want {
			t.Fatalf("description[%s] = %q, want %q", key, gotDescriptions[key], want)
		}
	}
}

func createWorkflowTestProject(t *testing.T, title string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": title,
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject %q: expected 201, got %d: %s", title, w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	t.Cleanup(func() {
		req := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		req = withURLParam(req, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), req)
	})
	return project.ID
}

func createWorkflowTestIssue(t *testing.T, title, projectID string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":      title,
		"project_id": projectID,
		"status":     "todo",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue %q: expected 201, got %d: %s", title, w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	return issue.ID
}

func seedWorkflowTestDefaultIssueWorkflow(t *testing.T, projectID string) {
	t.Helper()
	projectUUID := parseUUID(projectID)
	workspaceUUID := parseUUID(testWorkspaceID)
	if _, err := testHandler.ensureDefaultIssueWorkflow(context.Background(), workspaceUUID, projectUUID); err != nil {
		t.Fatalf("seed default workflow: %v", err)
	}
}

func insertWorkflowTestStatus(t *testing.T, projectID, key, name string) {
	t.Helper()
	var workflowID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id FROM issue_workflow
		WHERE workspace_id = $1 AND project_id = $2 AND is_default = true
	`, testWorkspaceID, projectID).Scan(&workflowID); err != nil {
		t.Fatalf("load default workflow: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO issue_status_def (workflow_id, key, name, category, position, on_main_graph)
		VALUES ($1, $2, $3, 'started', 50, true)
	`, workflowID, key, name); err != nil {
		t.Fatalf("insert custom status: %v", err)
	}
}
