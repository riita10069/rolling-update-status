package pkg

import (
	"fmt"
	v1 "k8s.io/api/apps/v1"
	"os"
)

const GithubStatusEnvName = "GithubStatus"

type StoreRepository interface {
	Create(state string, description string) error
}

func NewStore(dply v1.Deployment) StoreRepository {
	dbType := os.Getenv("DB_TYPE")
	switch dbType {
	case GithubStatusEnvName:
		return NewGithubStatus(dply)
	default:
		fmt.Println("not found DB_TYPE")
		return NewGithubStatus(dply)
	}
}
