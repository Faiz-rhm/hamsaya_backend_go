package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestE2E_Report_ReportPost(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	authorEmail := fmt.Sprintf("e2e-reportauthor-%d@test.local", ts)
	reporterEmail := fmt.Sprintf("e2e-reporter-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-reportauthor-%")
		env.cleanupTestData(t, "e2e-reporter-%")
	})

	author := register(t, env, authorEmail, "Password123!")
	reporter := register(t, env, reporterEmail, "Password123!")

	postID := createPost(t, env, author.AccessToken, "Post to be reported")

	body := `{"reason":"spam"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/report"), reporter.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "report post failed: %s", string(raw))
}

func TestE2E_Report_ReportComment(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	authorEmail := fmt.Sprintf("e2e-reportcmtauthor-%d@test.local", ts)
	reporterEmail := fmt.Sprintf("e2e-reportcmtreporter-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-reportcmtauthor-%")
		env.cleanupTestData(t, "e2e-reportcmtreporter-%")
	})

	author := register(t, env, authorEmail, "Password123!")
	reporter := register(t, env, reporterEmail, "Password123!")

	postID := createPost(t, env, author.AccessToken, "Post with reportable comment")
	commentID := createComment(t, env, author.AccessToken, postID, "Offensive comment")

	body := `{"reason":"harassment"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/comments/"+commentID+"/report"), reporter.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "report comment failed: %s", string(raw))
}

func TestE2E_Report_ReportUser(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	targetEmail := fmt.Sprintf("e2e-reportusertarget-%d@test.local", ts)
	reporterEmail := fmt.Sprintf("e2e-reportuserreporter-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-reportusertarget-%")
		env.cleanupTestData(t, "e2e-reportuserreporter-%")
	})

	target := register(t, env, targetEmail, "Password123!")
	reporter := register(t, env, reporterEmail, "Password123!")

	body := `{"reason":"fake_account"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+target.UserID+"/report"), reporter.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "report user failed: %s", string(raw))
}
