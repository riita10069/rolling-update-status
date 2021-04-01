package pkg

import (
	"context"
	"fmt"
	"github.com/google/go-github/v34/github"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"regexp"
	"strings"
)

type GithubStatus struct {
	dply v1.Deployment
	rs   v1.ReplicaSet
}

const (
	ACCEPT   = "application/vnd.github.v3+json"
	DOMAIN   = "wantedly.com"
	USERNAME = "riita10069"
)

func NewGithubStatusWithDply(dply v1.Deployment) GithubStatus {
	return GithubStatus{dply: dply}
}

func NewGithubStatusWithRs(rs v1.ReplicaSet) GithubStatus {
	return GithubStatus{rs: rs}
}

func (gs GithubStatus) Create(state string, description string) error {
	containers := gs.dply.Spec.Template.Spec.Containers
	targetContainer := gs.dply.Annotations[DOMAIN+"/deploy-target-container"]
	sha, err := getDeployTargetContainerImageTag(containers, targetContainer)
	if err != nil {
		return err
	}
	if sha == "" {
		return nil
	}

	request := map[string]string{
		"accept":      ACCEPT,
		"owner":       strings.Split(strings.Split(gs.dply.Annotations[DOMAIN+"/github"], "/")[0], "=")[1],
		"repo":        strings.Split(gs.dply.Annotations[DOMAIN+"/github"], "/")[1],
		"sha":         sha,
		"state":       state,
		"description": description,
		"context":     gs.dply.Name,
	}

	fmt.Println("----------------------------------------------")
	fmt.Println(request)

	return nil
}

func (gs GithubStatus) CreatePending() error {
	rs := gs.rs
	if rs.Annotations == nil {
		rs.Annotations = make(map[string]string)
	}
	containers := rs.Spec.Template.Spec.Containers
	targetContainer := rs.Annotations[DOMAIN+"/deploy-target-container"]
	sha, err := getDeployTargetContainerImageTag(containers, targetContainer)
	if err != nil {
		return err
	}
	if sha == "" {
		return nil
	}
	fmt.Println("rs name", rs.Name)
	s := strings.Split(rs.Name, "-")
	s = s[:len(s)-1]
	deploymentName := strings.Join(s, "-")

	request := map[string]string{
		"accept":      ACCEPT,
		"owner":       strings.Split(strings.Split(rs.Annotations[DOMAIN+"/github"], "/")[0], "=")[1],
		"repo":        strings.Split(rs.Annotations[DOMAIN+"/github"], "/")[1],
		"sha":         sha,
		"state":       "pending",
		"description": "deploy start",
		"context":     deploymentName,
	}

	fmt.Println(request)
	return nil
}

func getDeployTargetContainerImageTag(containers []v12.Container, targetContainerName string) (string, error) {
	for _, container := range containers {
		if container.Name == targetContainerName {
			sha := strings.Split(container.Image, ":")[1]
			// TODO: delete commented out
			//if !isSHA(sha)  {
			//	return "", fmt.Errorf("image tag is not hash")
			//}
			return sha, nil
		}
	}
	return "", nil
}

func isSHA(s string) bool {
	return regexpMatcher("[0-9A-Fa-f]{40}", s)
}

func regexpMatcher(reg, str string) bool {
	return regexp.MustCompile(reg).Match([]byte(str))
}

func fetchOrganizations(username string) ([]*github.Organization, error) {
	client := github.NewClient(nil)
	orgs, _, err := client.Organizations.List(context.Background(), username, nil)
	return orgs, err
}
