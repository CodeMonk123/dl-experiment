/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"database/sql"
	"fmt"
	pttypes "github.com/kubeflow/pytorch-operator/pkg/apis/pytorch/v1"
	tftypes "github.com/kubeflow/tf-operator/pkg/apis/tensorflow/v1"
	"github.com/nemoworks/dl-experiment/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pytorchv1 "github.com/kubeflow/pytorch-operator/pkg/client/clientset/versioned/scheme"
	tfv1 "github.com/kubeflow/tf-operator/pkg/client/clientset/versioned/scheme"
	mlhubv1 "github.com/nemoworks/dl-experiment/api/v1"
	"github.com/nemoworks/dl-experiment/datastore"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ExperimentName string

const (
	OwnerName = "owner"
)

// DLExperimentReconciler reconciles a DLExperiment object
type DLExperimentReconciler struct {
	client.Client
	KubeClient client.Client
	DBClient   *sql.DB
	Log        logr.Logger
	Scheme     *runtime.Scheme
}

func NewDLExperimentReconciler(mgr manager.Manager) (*DLExperimentReconciler, error) {
	kubeClient, err := setupClient()
	if err != nil {
		return nil, err
	}

	dbClient, err := datastore.InitDBClient()
	if err != nil {
		return nil, err
	}

	reconciler := DLExperimentReconciler{
		Client:     mgr.GetClient(),
		KubeClient: kubeClient,
		Log:        ctrl.Log.WithName("controllers").WithName("DLExperiment"),
		Scheme:     mgr.GetScheme(),
		DBClient:   dbClient,
	}

	return &reconciler, nil
}

// +kubebuilder:rbac:groups=mlhub.njuics.cn,resources=dlexperiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mlhub.njuics.cn,resources=dlexperiments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=tfjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=pytorchjobs,verbs=get;list;watch;create;update;patch;delete

func (r *DLExperimentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("dlexperiment", req.NamespacedName)
	log.Info("starting reconcile", "experiment", req.Name)
	// your logic here

	// 1. Get the DLExperiment object
	targetObject := unstructured.Unstructured{}
	targetObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   mlhubv1.GroupVersion.Group,
		Version: mlhubv1.GroupVersion.Version,
		Kind:    "DLExperiment",
	})

	if err := r.Get(ctx, req.NamespacedName, &targetObject); err != nil {
		log.Error(err, "unable to fetch experiment object", "name", req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var experiment mlhubv1.DLExperiment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(targetObject.Object, &experiment); err != nil {
		log.Error(err, "unable to convert from unstructed object")
		return ctrl.Result{}, err
	}

	// 2. Update Status
	if experiment.Status.Status == "" {
		// Create tuner pod and change status to 'Created'
		log.Info("newly submitted experiment")
		if err := r.CreateTunerPod(&ctx, &experiment); err != nil {
			log.Error(err, "unable to create tuner pod")
			return ctrl.Result{}, err
		}
		log.Info("create tuner pod successfully")
		experiment.Status.Status = mlhubv1.ExperimentCreated
		if err := r.Update(ctx, &experiment); err != nil {
			log.Error(err, "unable to update experiment", "name", req.Name)
			return ctrl.Result{}, err
		}
		log.Info("update experiment status", "status", mlhubv1.ExperimentCreated)
		return ctrl.Result{}, nil
	} else if experiment.Status.Status == mlhubv1.ExperimentCreated {
		// Check whether the tuner pod is available
		// if not, do nothing
		// else change status to 'Running'
		tunerPodName := fmt.Sprintf("%s-tuner", req.Name)
		var tunerPod corev1.Pod
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: req.Namespace,
			Name:      tunerPodName,
		}, &tunerPod); err != nil {
			log.Error(err, "unable to get tuner pod")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		tunerIP := tunerPod.Status.PodIP
		if !utils.TunerIsAvailable(tunerIP) {
			log.Info("waiting for tuner launching...")
			return ctrl.Result{}, nil
		}

		// tuner is available
		experiment.Status.Status = mlhubv1.ExperimentRunning
		if err := r.Update(ctx, &experiment); err != nil {
			log.Error(err, "unable to change experiment status to running")
			return ctrl.Result{}, err
		}
		log.Info("change experiment status to ")
		return ctrl.Result{}, nil
	} else if experiment.Status.Status == mlhubv1.ExperimentRunning {
		// judge whether to create a new trail according to the number of running trials.
		if experiment.Spec.Trainer == "tensorflow" {
			tfjobs := tftypes.TFJobList{}
			if err := r.KubeClient.List(context.TODO(), &tfjobs, client.MatchingLabels{"owner": req.Name}); err != nil {
				log.Error(err, "unable to list tfjobs owned by experiment "+req.Name)
				return ctrl.Result{}, err
			}
			// TODO: update each job status in DB
			log.Info("will update each job's status")

			runningJobs := 0
			pendingJobs := 0
			succeedJobs := 0
			for _, job := range tfjobs.Items {
				if isRunning(job.Status) {
					runningJobs += 1
				} else if isPending(job.Status) {
					pendingJobs += 1
				} else if isSucceeded(job.Status) {
					succeedJobs += 1
				}
			}

			if succeedJobs == experiment.Spec.MaxTrialNum {
				// change experiment status to completed
				experiment.Status.Status = mlhubv1.ExperimentCompleted
				if err := r.Update(ctx, &experiment); err != nil {
					log.Error(err, "unable to change experiment status to completed")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else if runningJobs+pendingJobs < experiment.Spec.TrailConcurrency {
				// TODO: create a new trial
				nextIndex := runningJobs + pendingJobs
				log.Info("will create new trail", "index", nextIndex)
			}

		} else if experiment.Spec.Trainer == "pytorch" {
			ptjobs := pttypes.PyTorchJobList{}
			if err := r.KubeClient.List(context.TODO(), &ptjobs, client.MatchingLabels{"owner": req.Name}); err != nil {
				log.Error(err, "unable to list ptjobs owned by experiment "+req.Name)
			}
			runningJobs := 0
			pendingJobs := 0
			succeedJobs := 0
			for _, job := range ptjobs.Items {
				if isRunning(job.Status) {
					runningJobs += 1
				} else if isPending(job.Status) {
					pendingJobs += 1
				} else if isSucceeded(job.Status) {
					succeedJobs += 1
				}
			}

			if succeedJobs == experiment.Spec.MaxTrialNum {
				// change experiment status to completed
				experiment.Status.Status = mlhubv1.ExperimentCompleted
				if err := r.Update(ctx, &experiment); err != nil {
					log.Error(err, "unable to change experiment status to completed")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else if runningJobs+pendingJobs < experiment.Spec.TrailConcurrency {
				// TODO: create a new trial
				nextIndex := runningJobs + pendingJobs
				log.Info("will create new trail", "index", nextIndex)
			}

		}
	}

	return ctrl.Result{}, nil
}

func (r *DLExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mlhubv1.DLExperiment{}).
		Complete(r)
}

func setupClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	scheme := newScheme()
	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	tfv1.AddToScheme(scheme)
	pytorchv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	return scheme
}
