package controllers

import (
	pttypes "github.com/kubeflow/pytorch-operator/pkg/apis/pytorch/v1"
	tftypes "github.com/kubeflow/tf-operator/pkg/apis/tensorflow/v1"
	mlhubv1 "github.com/nemoworks/dl-experiment/api/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	defaultTFJobTTL      int32 = 30 * 24 * 3600
	defaultBackoffLimits int32 = 5
)

const (
	volumeTypeShm             = "dshm"
	defaultVolumeMountPathShm = "/dev/shm"
)

func GenerateTfJob(experiment mlhubv1.DLExperiment) *tftypes.TFJob {

	return nil
}

func GeneratePytorchJob(experiment mlhubv1.DLExperiment) *pttypes.PyTorchJob {
	return nil
}

func generatePodSpec(experiment mlhubv1.DLExperiment) *corev1.PodSpec {
	podSpec := &corev1.PodSpec{
		Volumes:       nil,
		Containers:    nil,
		RestartPolicy: "",
	}

	return podSpec
}
