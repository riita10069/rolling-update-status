package pkg

import (
	"context"
	"errors"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strconv"
	"strings"
)

const (
	ReplicaSetStatusAnnotation = "wantedly.com/deploy"
	RevisionAnnotation         = "deployment.kubernetes.io/revision"
	TimedOutReason             = "ProgressDeadlineExceeded"
	RevisionNotFound           = "revision not found"
)

var (
	NewReplicaSetNotFound = errors.New("new replicaset hasn't been made yet")
)

type RsListFunc func(string) ([]*appsv1.ReplicaSet, error)
type ReplicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o ReplicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o ReplicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReplicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

func RolloutStatus(dply appsv1.Deployment) (string, bool, error) {
	revision, err := GetRevision(dply)
	if err != nil {
		return "Failed to Get Revision", false, err
	}
	if revision > 0 {
		condition := GetDeploymentCondition(dply.Status, appsv1.DeploymentProgressing)
		if condition != nil && condition.Reason == TimedOutReason {
			return TimedOutReason, false, nil
		}
		if dply.Status.UpdatedReplicas == 0 {
			return "pending", false, nil
		}
		if *dply.Spec.Replicas != dply.Status.UpdatedReplicas {
			return "pending", false, nil
		}
		if dply.Status.Replicas != dply.Status.UpdatedReplicas {
			return "pending", false, nil
		}
		if dply.Status.AvailableReplicas != dply.Status.UpdatedReplicas {
			return "pending", false, nil
		}
	} else {
		return RevisionNotFound, false, nil
	}
	return "success", true, err
}

func IsJustDeployStarted(dply appsv1.Deployment, cli client.Client) (bool, error) {
	newReplicaSet, err := GetNewReplicaSet(dply, cli)
	if err != nil {
		return false, err
	}
	if newReplicaSet == nil {
		return false, NewReplicaSetNotFound

	}
	if newReplicaSet.Annotations == nil {
		newReplicaSet.Annotations = make(map[string]string)
	}
	rsAnnotations := newReplicaSet.Annotations[ReplicaSetStatusAnnotation]
	if rsAnnotations == "" || rsAnnotations == "success" {
		if SetReplicaSetStatusAnnotation(*newReplicaSet, "pending") {
			if err = cli.Update(context.TODO(), newReplicaSet); err != nil {
				return false, err
			} else {
				return true, nil
			}
		}
	} else if rsAnnotations == "pending" {
		// do nothing.
		return false, nil
	}
	return false, fmt.Errorf("newest rs's annotation is strange value")
}

func IsJustDeployFinished(dply appsv1.Deployment, cli client.Client) (bool, error) {
	newReplicaSet, err := GetNewReplicaSet(dply, cli)
	if err != nil {
		return false, err
	}
	if newReplicaSet == nil {
		return false, NewReplicaSetNotFound
	}
	if newReplicaSet.Annotations == nil {
		newReplicaSet.Annotations = make(map[string]string)
	}
	rsAnnotations := newReplicaSet.Annotations[ReplicaSetStatusAnnotation]
	if rsAnnotations == "pending" || rsAnnotations == "" {
		if SetReplicaSetStatusAnnotation(*newReplicaSet, "success") {
			if err = cli.Update(context.TODO(), newReplicaSet); err != nil {
				return false, err
			} else {
				return true, err
			}
		}
	} else if rsAnnotations == "success" {
		// do nothing
		return false, err
	}
	return false, fmt.Errorf("newest rs's annotation is strange value when success")
}

func IsJustDeployFailed(dply appsv1.Deployment, cli client.Client) (bool, error) {
	newReplicaSet, err := GetNewReplicaSet(dply, cli)
	if err != nil {
		return false, err
	}
	if newReplicaSet == nil {
		return false, NewReplicaSetNotFound
	}
	if newReplicaSet.Annotations == nil {
		newReplicaSet.Annotations = make(map[string]string)
	}
	rsAnnotations := newReplicaSet.Annotations[ReplicaSetStatusAnnotation]
	if rsAnnotations == "failed" {
		return false, err
	} else {
		if SetReplicaSetStatusAnnotation(*newReplicaSet, "failed") {
			if err = cli.Update(context.TODO(), newReplicaSet); err != nil {
				return false, err
			} else {
				return true, err
			}
		}
	}
	return false, nil
}

func GetRevision(dply appsv1.Deployment) (int64, error) {
	v, ok := dply.Annotations[RevisionAnnotation]
	if !ok {
		return 0, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

func GetDeploymentCondition(status appsv1.DeploymentStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for conditionKey := range status.Conditions {
		conditionStatus := status.Conditions[conditionKey]
		if conditionStatus.Type == condType {
			return &conditionStatus
		}
	}
	return nil
}

func GetNewReplicaSet(deployment appsv1.Deployment, c client.Client) (*appsv1.ReplicaSet, error) {
	rsList, err := ListReplicaSets(deployment, RsListFromClient(c))
	if err != nil || len(rsList) == 0 {
		return nil, err
	}

	return FindNewReplicaSet(deployment, rsList), nil
}

func RsListFromClient(c client.Client) RsListFunc {
	return func(namespace string) ([]*appsv1.ReplicaSet, error) {
		var rsList appsv1.ReplicaSetList
		c.List(context.Background(), &rsList, &client.ListOptions{
			Namespace: namespace,
		})
		var ret []*appsv1.ReplicaSet
		for i := range rsList.Items {
			ret = append(ret, &rsList.Items[i])
		}
		return ret, nil
	}
}

func ListReplicaSets(deployment appsv1.Deployment, getRSList RsListFunc) ([]*appsv1.ReplicaSet, error) {
	namespace := deployment.Namespace
	all, err := getRSList(namespace)
	if err != nil {
		return nil, err
	}
	var owned []*appsv1.ReplicaSet
	for _, rs := range all {
		s := strings.Split(rs.Name, "-")
		s = s[:len(s)-1]
		deploymentName := strings.Join(s, "-")

		if deployment.Name == deploymentName {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

func FindNewReplicaSet(deployment appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(ReplicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if EqualIgnoreHash(&rsList[i].Spec.Template, &deployment.Spec.Template) {
			return rsList[i]
		}
	}

	return nil
}

func EqualIgnoreHash(template1, template2 *v1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

func SetReplicaSetStatusAnnotation(rs appsv1.ReplicaSet, status string) bool {
	updated := false

	if rs.Annotations == nil {
		rs.Annotations = make(map[string]string)
	}

	if rs.Annotations[ReplicaSetStatusAnnotation] != status {
		rs.Annotations[ReplicaSetStatusAnnotation] = status
		updated = true
	}

	return updated
}
