package cli

import (
	"fmt"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/jira"
)

// stubClient implements client.JiraClient for testing CLI commands.
// Only the methods actually exercised by CLI commands carry configurable
// behaviour; the remainder return sensible zero values.
type stubClient struct {
	cfg          *config.Config
	meName       string
	meErr        error
	issue        *jira.Issue
	issueErr     error
	boards       []jira.Board
	boardsErr    error
	boardSprints []jira.Sprint
	boardSprtErr error
	sprintIssues []jira.Issue
	sprintIssErr error
	boardIssues  []jira.Issue
	boardIssErr  error
	searchIssues []jira.Issue
	searchErr    error
}

func (s *stubClient) Me() (string, error)    { return s.meName, s.meErr }
func (s *stubClient) Config() *config.Config { return s.cfg }
func (s *stubClient) IssueURL(key string) string {
	return fmt.Sprintf("https://test.atlassian.net/browse/%s", key)
}
func (s *stubClient) GetIssue(_ string) (*jira.Issue, error) { return s.issue, s.issueErr }
func (s *stubClient) Boards(_ string) ([]jira.Board, error)  { return s.boards, s.boardsErr }
func (s *stubClient) BoardSprints(_ int, _ string) ([]jira.Sprint, error) {
	return s.boardSprints, s.boardSprtErr
}
func (s *stubClient) SprintIssues(_ int) ([]jira.Issue, error) {
	return s.sprintIssues, s.sprintIssErr
}
func (s *stubClient) BoardIssues(_ string, _ ...string) ([]jira.Issue, error) {
	return s.boardIssues, s.boardIssErr
}
func (s *stubClient) SearchJQL(_ string, _ uint) ([]jira.Issue, error) {
	return s.searchIssues, s.searchErr
}

// --- Methods not exercised by CLI commands — return zero values. ---

func (s *stubClient) CreateIssue(_ *client.CreateIssueRequest) (*client.CreateIssueResponse, error) {
	return nil, nil
}
func (s *stubClient) IssueTypesWithID(_ string) ([]jira.IssueTypeInfo, error) { return nil, nil }
func (s *stubClient) BoardIssuesPage(_ int, _, _ int) (*client.PageResult, error) {
	return &client.PageResult{}, nil
}
func (s *stubClient) EpicIssues(_ string) ([]jira.Issue, error) { return nil, nil }
func (s *stubClient) EpicIssuesPage(_ string, _, _ int) (*client.PageResult, error) {
	return &client.PageResult{}, nil
}
func (s *stubClient) SprintIssuesPage(_ int, _, _ int) (*client.PageResult, error) {
	return &client.PageResult{}, nil
}
func (s *stubClient) SprintIssueStats(_ int) (int, int, int, int, error) { return 0, 0, 0, 0, nil }
func (s *stubClient) Transitions(_ string) ([]jira.Transition, error)    { return nil, nil }
func (s *stubClient) TransitionIssue(_, _ string) error                  { return nil }
func (s *stubClient) ChildIssues(_ string) ([]jira.ChildIssue, error)    { return nil, nil }
func (s *stubClient) AssignIssue(_, _ string) error                      { return nil }
func (s *stubClient) EditIssue(_ string, _ *client.EditIssueRequest) error {
	return nil
}
func (s *stubClient) LinkIssue(_, _, _ string) error                   { return nil }
func (s *stubClient) GetIssueLinkTypes() ([]jira.IssueLinkType, error) { return nil, nil }
func (s *stubClient) DeleteIssue(_ string, _ bool) error               { return nil }
func (s *stubClient) SearchJQLPage(_ string, _ int, _ int, _ string) (*client.PageResult, error) {
	return &client.PageResult{}, nil
}
func (s *stubClient) BoardFilterJQL(_ int) (string, error) { return "", fmt.Errorf("no filter") }
func (s *stubClient) JQLMetadata() (*jira.JQLMetadata, error) {
	return &jira.JQLMetadata{}, nil
}
func (s *stubClient) Projects() ([]jira.Project, error)                          { return nil, nil }
func (s *stubClient) ResolveParents(_ []jira.Issue) map[string]client.ParentInfo { return nil }
func (s *stubClient) SearchUsers(_, _ string) ([]client.UserInfo, error)         { return nil, nil }
func (s *stubClient) CreateMetaFields(_, _ string) ([]jira.CustomFieldDef, error) {
	return nil, nil
}
func (s *stubClient) AddComment(_, _ string) error { return nil }

// --- Confluence stubs ---

func (s *stubClient) ConfluenceSpaces() ([]confluence.Space, error) { return nil, nil }
func (s *stubClient) ConfluencePage(_ string) (*confluence.Page, error) {
	return &confluence.Page{}, nil
}
func (s *stubClient) ConfluencePageAncestors(_ string) ([]confluence.PageAncestor, error) {
	return nil, nil
}
func (s *stubClient) ConfluenceSpacePages(_ string, _ int) ([]confluence.Page, error) {
	return nil, nil
}
func (s *stubClient) ConfluencePageURL(_ string) string { return "" }
func (s *stubClient) UpdateConfluencePage(_, _, _ string, _ int) (*confluence.Page, error) {
	return &confluence.Page{}, nil
}
func (s *stubClient) RemoteLinks(_ string) ([]jira.RemoteLink, error) {
	return nil, nil
}
func (s *stubClient) GetUserDisplayName(accountID string) string { return accountID }

// setStubClient injects a stub client and config into the package-level
// variables used by CLI commands. Returns a cleanup function that restores
// the original values.
func setStubClient(stub *stubClient) func() {
	origClient := cliClient
	origConfig := cliConfig
	cliClient = stub
	cliConfig = stub.cfg
	return func() {
		cliClient = origClient
		cliConfig = origConfig
	}
}
