package controllers

import (
	"context"
	"fmt"
	mlhubv1 "github.com/nemoworks/dl-experiment/api/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *DLExperimentReconciler) CreateTunerPod(ctx *context.Context, experiment *mlhubv1.DLExperiment) error {
	tunerPod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tuner", experiment.Name),
			Namespace: experiment.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "tuner",
					Image:   experiment.Spec.TunerImage,
					Command: []string{"python3", "main.py"},
					Ports: []corev1.ContainerPort{{
						ContainerPort: 8001,
					}},
					Env: []corev1.EnvVar{
						{
							Name:  "TUNER",
							Value: experiment.Spec.Tuner,
						},
						{
							Name:  "TARGET",
							Value: experiment.Spec.Target,
						},
						{
							Name:  "SEARCH_SPACE",
							Value: experiment.Spec.SearchSpace,
						},
					},
					ImagePullPolicy: corev1.PullAlways,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if err := r.Create(*ctx, &tunerPod); err != nil {
		return err
	}

	return ctrl.SetControllerReference(experiment, &tunerPod, r.Scheme)
}
