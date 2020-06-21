package controllers

import (
	"fmt"
	pttypes "github.com/kubeflow/pytorch-operator/pkg/apis/pytorch/v1"
	tfcommon "github.com/kubeflow/tf-operator/pkg/apis/common/v1"
	tftypes "github.com/kubeflow/tf-operator/pkg/apis/tensorflow/v1"
	mlhubv1 "github.com/nemoworks/dl-experiment/api/v1"
	"github.com/nemoworks/dl-experiment/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"strconv"
	"strings"
)

var (
	defaultTFJobTTL      int32 = 30 * 24 * 3600
	defaultBackoffLimits int32 = 5
)

const (
	volumeTypeShm             = "dshm"
	defaultVolumeMountPathShm = "/dev/shm"
)

func GenerateTfJob(experiment *mlhubv1.DLExperiment, trailID, reportURL, parameter string, index int) *tftypes.TFJob {
	tfjob := &tftypes.TFJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("tfjob-%s-exp-%s-trial-%s", experiment.Spec.Workspace, experiment.Spec.ExperimentID, trailID),
			Namespace: experiment.Namespace,
		},
		Spec: tftypes.TFJobSpec{
			TFReplicaSpecs: map[tftypes.TFReplicaType]*tfcommon.ReplicaSpec{},
		},
	}

	podSpec := generatePodSpec(experiment)
	addEnvToPodSpec("TRIAL_ID", trailID, &podSpec)
	addEnvToPodSpec("EXP_ID", experiment.Spec.ExperimentID, &podSpec)
	addEnvToPodSpec("DLKIT_PARAM", parameter, &podSpec)
	addEnvToPodSpec("SEQUENCE", fmt.Sprintf("%d", index), &podSpec)
	addEnvToPodSpec("REPORT_URL", reportURL, &podSpec)

	tfjob.Spec.TFReplicaSpecs[tftypes.TFReplicaTypeWorker] = &tfcommon.ReplicaSpec{
		Replicas: tftypes.Int32(1),
		Template: corev1.PodTemplateSpec{
			Spec: podSpec,
		},
	}

	tfjob.Spec.TTLSecondsAfterFinished = &defaultTFJobTTL
	tfjob.Spec.BackoffLimit = &defaultBackoffLimits

	return tfjob
}

func GeneratePytorchJob(experiment *mlhubv1.DLExperiment, trailID, reportURL, parameter string, index int) *pttypes.PyTorchJob {

	return nil
}

func generatePodSpec(experiment *mlhubv1.DLExperiment) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            experiment.Spec.Trainer,
				Image:           experiment.Spec.TrialImage,
				Command:         strings.Split(experiment.Spec.Command, " "),
				WorkingDir:      experiment.Spec.WorkingDir,
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: volumeTypeShm,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium:    corev1.StorageMediumMemory,
				SizeLimit: nil,
			},
		},
	})

	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeTypeShm,
		MountPath: defaultVolumeMountPathShm,
	})

	// Allocate GPU
	if experiment.Spec.GpuRequired > 0 {
		podSpec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse(strconv.Itoa(experiment.Spec.GpuRequired)),
			},
			Requests: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse(strconv.Itoa(experiment.Spec.GpuRequired)),
			},
		}
	}

	// Mount workspace pvc to job pod
	utils.Volume{
		Name:      utils.GeneralWorkspacePVC(experiment.Spec.Workspace),
		MountPath: utils.WorkspaceVolumeMountPath,
		ReadOnly:  true,
	}.AddToPodSpec(&podSpec)

	// Mount dataset pvc to job pod
	if len(experiment.Spec.Datasets) > 0 {
		for _, ds := range experiment.Spec.Datasets {
			utils.Volume{
				Name:      utils.GeneralDatasetPVC(ds),
				MountPath: path.Join(utils.WorkspaceVolumeMountPath, ds),
				ReadOnly:  true,
			}.AddToPodSpec(&podSpec)
		}
	}

	return podSpec
}

func addEnvToPodSpec(key, val string, podSpec *corev1.PodSpec) {
	podSpec.Containers[0].Env = append(podSpec.Containers[0].Env, corev1.EnvVar{
		Name:  key,
		Value: val,
	})
}
