package minideploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeploymentMetadataPreservesSafeExactProvenance(t *testing.T) {
	commit := strings.Repeat("a", 40)
	imageID := "sha256:" + strings.Repeat("b", 64)
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet ||
				request.URL.Path != "/deployments" {

				t.Fatalf("request = %s %s", request.Method, request.URL.Path)
			}
			_, _ = response.Write([]byte(`[{
				"app":"portfolio",
				"repoUrl":"https://legacy:secret@github.com/owner/repo.git",
				"strategy":"fullstack-vite-node",
				"status":"running",
				"imageId":"` + imageID + `",
				"source":{
					"provider":"github",
					"repository":"owner/repo",
					"branch":"main",
					"requestedRef":"refs/heads/main",
					"commitSha":"` + commit + `"
				},
				"activatedAt":"2026-09-22T10:00:00-04:00",
				"environmentVariables":["SECRET_VALUE_MUST_NOT_PASS"],
				"services":[{
					"name":"backend",
					"strategy":"node-express",
					"status":"running",
					"imageId":"` + imageID + `",
					"path":"/private/path"
				}],
				"databaseAttachments":[{
					"databaseId":"database_11111111111111111111111111111111",
					"displayName":"Portfolio",
					"bindingName":"primary"
				}]
			}]`))
		},
	))
	defer server.Close()

	deployments, err := NewClient(
		server.URL,
		time.Second,
	).DeploymentMetadata(context.Background())
	if err != nil {
		t.Fatalf("DeploymentMetadata() error = %v", err)
	}
	if len(deployments) != 1 ||
		deployments[0].Source == nil ||
		deployments[0].Source.CommitSHA != commit ||
		deployments[0].Source.Branch != "main" ||
		deployments[0].ActivatedAt == nil ||
		deployments[0].ActivatedAt.Location() != time.UTC ||
		len(deployments[0].Services) != 1 ||
		deployments[0].Services[0].ImageID != imageID {

		t.Fatalf("metadata = %#v", deployments)
	}
	encoded, err := json.Marshal(deployments)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"repoUrl",
		"legacy:secret",
		"SECRET_VALUE_MUST_NOT_PASS",
		"/private/path",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("safe metadata leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestDeploymentHistoryPreservesVersionIdentityAndLegacyUnknown(t *testing.T) {
	firstCommit := strings.Repeat("a", 40)
	secondCommit := strings.Repeat("b", 40)
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/deployments/portfolio/history" {
				t.Fatalf("path = %q", request.URL.Path)
			}
			_, _ = response.Write([]byte(`{
				"app":"portfolio",
				"versions":[
					{
						"app":"portfolio",
						"repoUrl":"https://secret@example.test/repo.git",
						"strategy":"vite-static",
						"image":"/private/minideploy-portfolio:v2",
						"source":{"branch":"release","commitSha":"` + secondCommit + `"},
						"activatedAt":"2026-09-22T15:00:00Z",
						"deployedAt":"2026-09-22T16:00:00Z"
					},
					{
						"app":"portfolio",
						"strategy":"vite-static",
						"image":"minideploy-portfolio:v1",
						"source":{"branch":"main","commitSha":"` + firstCommit + `"},
						"activatedAt":"2026-09-21T12:00:00Z",
						"deployedAt":"2026-09-22T15:00:00Z"
					},
					{
						"app":"portfolio",
						"strategy":"vite-static",
						"image":"minideploy-portfolio:legacy",
						"deployedAt":"2026-09-20T12:00:00Z"
					}
				]
			}`))
		},
	))
	defer server.Close()

	history, err := NewClient(
		server.URL,
		time.Second,
	).DeploymentHistory(context.Background(), "portfolio")
	if err != nil {
		t.Fatalf("DeploymentHistory() error = %v", err)
	}
	if len(history.Versions) != 3 ||
		history.Versions[0].Source.CommitSHA != secondCommit ||
		history.Versions[1].Source.CommitSHA != firstCommit ||
		history.Versions[2].Source != nil ||
		history.Versions[2].ActivatedAt != nil ||
		history.Versions[0].ActivatedAt.Equal(
			history.Versions[0].ArchivedAt,
		) {

		t.Fatalf("history = %#v", history)
	}
	encoded, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "repoUrl") ||
		strings.Contains(string(encoded), "secret@example") ||
		strings.Contains(string(encoded), "/private/") ||
		strings.Contains(string(encoded), "deployedAt") {

		t.Fatalf("safe history leaked upstream fields: %s", encoded)
	}
	if !strings.Contains(string(encoded), "archivedAt") {
		t.Fatalf("safe history lacks explicit archive time: %s", encoded)
	}
}

func TestDeploymentMetadataRejectsMalformedProvenance(t *testing.T) {
	for _, body := range []string{
		`[{"app":"portfolio","strategy":"vite-static","status":"running","source":{"branch":"main","commitSha":"short"}}]`,
		`[{"app":"portfolio","strategy":"vite-static","status":"running","source":{"provider":"github","repository":"https://user:secret@github.com/owner/repo","commitSha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]`,
		`[{"app":"portfolio","strategy":"vite-static","status":"running","imageId":"sha256:short"}]`,
		`[{"app":"portfolio","strategy":"vite-static","status":"running"}] {}`,
		`[]` + strings.Repeat(" ", maxMetadataResponseBytes),
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(response http.ResponseWriter, _ *http.Request) {
					_, _ = response.Write([]byte(body))
				},
			))
			defer server.Close()
			if _, err := NewClient(
				server.URL,
				time.Second,
			).DeploymentMetadata(context.Background()); err == nil {

				t.Fatal("DeploymentMetadata() error = nil")
			}
		})
	}
}
