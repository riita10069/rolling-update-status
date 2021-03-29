package pkg

import (
	"fmt"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	"regexp"
	"strings"
)

type GithubStatus struct {
	dply v1.Deployment
}

const (
	ACCEPT = "application/vnd.github.v3+json"
	DOMAIN = "wantedly.com"
)

func NewGithubStatus(dply v1.Deployment) GithubStatus {
	return GithubStatus{dply: dply}
}

func (gs GithubStatus) Create(state string, description string) error {
	containers := gs.dply.Spec.Template.Spec.Containers
	targetContainer := gs.dply.Annotations[DOMAIN+"/deploy-target-container"]
	sha, err := getDeployTargetContainerImageTag(containers, targetContainer)
	if err != nil {
		return err
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
	return "", fmt.Errorf("connot find target container")
}

func isSHA(s string) bool {
	return regexpMatcher("[0-9A-Fa-f]{40}", s)
}

func regexpMatcher(reg, str string) bool {
	return regexp.MustCompile(reg).Match([]byte(str))
}
