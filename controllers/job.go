package controllers

import (
	tfcommon "github.com/kubeflow/tf-operator/pkg/apis/common/v1"
	v1 "k8s.io/api/core/v1"
)

func hasCondition(status tfcommon.JobStatus, condType tfcommon.JobConditionType) bool {
	for _, condition := range status.Conditions {
		if condition.Type == condType && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isSucceeded(status tfcommon.JobStatus) bool {
	return hasCondition(status, tfcommon.JobSucceeded)
}

func isFailed(status tfcommon.JobStatus) bool {
	return hasCondition(status, tfcommon.JobFailed)
}

func isRunning(status tfcommon.JobStatus) bool {
	return hasCondition(status, tfcommon.JobRunning)
}

func isPending(status tfcommon.JobStatus) bool {
	if status.Conditions == nil {
		return true
	}
	return hasCondition(status, tfcommon.JobCreated) || hasCondition(status, tfcommon.JobRestarting)
}
