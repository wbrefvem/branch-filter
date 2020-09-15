package branchfilter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/driver/bitbucket"
	"github.com/drone/go-scm/scm/driver/gitea"
	"github.com/drone/go-scm/scm/driver/github"
	"github.com/drone/go-scm/scm/driver/gitlab"
	"github.com/drone/go-scm/scm/driver/gogs"
	"github.com/drone/go-scm/scm/driver/stash"
	log "github.com/sirupsen/logrus"
)

// BranchFilterServeMux implements the branch filter server
type BranchFilterServeMux struct {
	*http.ServeMux
	URL             string
	GitClient       *scm.Client
	BindAddress     string
	Path            string
	Port            string
	AllowedBranches map[string]string
}

// NewBranchFilterServeMux creates a new branch filter server
func NewBranchFilterServeMux() (*BranchFilterServeMux, error) {

	gitClient, err := getGitClient()
	if err != nil {
		return nil, err
	}

	allowedBranchesString := os.Getenv("ALLOWED_BRANCHES")
	if allowedBranchesString == "" {
		return nil, fmt.Errorf("no branches allowed")
	}

	allowedBranches := make(map[string]string)
	allowedBranchesSlice := strings.Split(allowedBranchesString, ":")
	for _, b := range allowedBranchesSlice {
		allowedBranches[b] = ""
	}

	b := &BranchFilterServeMux{
		ServeMux:        http.NewServeMux(),
		GitClient:       gitClient,
		BindAddress:     "localhost",
		Path:            "/",
		Port:            "8080",
		AllowedBranches: allowedBranches,
	}

	b.Handle(b.Path, http.HandlerFunc(b.handleWebHookRequests))

	return b, nil
}

// handle request for pipeline runs
func (b *BranchFilterServeMux) handleWebHookRequests(w http.ResponseWriter, r *http.Request) {
	log.Infof("handling webhook request with method %s", r.Method)
	switch r.Method {
	case http.MethodPost:
		b.handlePost(w, r)
	default:
		log.Infof("branch filter received and ignored a request with unsupported method %s from %s", r.Method, r.RemoteAddr)
	}
}

func secretFunc(scm.Webhook) (string, error) {
	return "", nil
}

func (b *BranchFilterServeMux) handlePost(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	parsedWebhook, err := b.GitClient.Webhooks.Parse(req, secretFunc)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if parsedWebhook == nil {
		log.Error("parsed webhook is nil")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	prHook, ok := parsedWebhook.(*scm.PullRequestHook)
	if !ok {
		log.Error("error casting webhook to concrete type")
		log.Errorf("actual type is %v", reflect.TypeOf(parsedWebhook))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if prHook == nil {
		log.Error("webhook cast result was nil")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, ok := b.AllowedBranches[prHook.PullRequest.Target]; ok {
		for header, valSlice := range req.Header {
			for _, val := range valSlice {
				w.Header().Add(header, val)

			}
		}
		w.Write(body)
	}
}

func getGitClient() (*scm.Client, error) {

	gitProvider := strings.ToLower(os.Getenv("GIT_PROVIDER"))
	gitURL := os.Getenv("GIT_URL")

	switch gitProvider {
	case "bitbucket", "bitbucket-cloud", "bitbucketcloud":
		if gitURL != "" {
			return bitbucket.New(gitURL)
		}
		return bitbucket.NewDefault(), nil
	case "bitbucketserver", "bitbucket-server":
		log.Infof("initializing Bitbucket Server client with URL %s", gitURL)
		return stash.New(gitURL)
	case "gitea":
		return gitea.New(gitURL)
	case "gitlab":
		return gitlab.New(gitURL)
	case "gogs":
		return gogs.New(gitURL)
	default:
		if gitURL != "" {
			return github.New(gitURL)
		}
		return github.NewDefault(), nil
	}
}
