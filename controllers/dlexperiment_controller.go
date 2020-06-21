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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	DBClient       *sql.DB
	Log            logr.Logger
	Scheme         *runtime.Scheme
	TunerAddrCache map[string]string
}

func NewDLExperimentReconciler(mgr manager.Manager) (*DLExperimentReconciler, error) {

	dbClient, err := datastore.InitDBClient()
	if err != nil {
		return nil, err
	}

	reconciler := DLExperimentReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("DLExperiment"),
		Scheme:         mgr.GetScheme(),
		DBClient:       dbClient,
		TunerAddrCache: make(map[string]string),
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
		var count int
		experiment.Status.Count = &count
		if err := r.Status().Update(ctx, &experiment); err != nil {
			log.Error(err, "unable to update experiment", "name", req.Name)
			return ctrl.Result{}, err
		}
		log.Info("update experiment status", "status", mlhubv1.ExperimentCreated)
		log.Info("update count", "count", *experiment.Status.Count)
		return ctrl.Result{Requeue: true}, nil
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
		log.Info("try to test tuner:", "pod ip", tunerIP)
		if err := utils.TunerIsAvailable(tunerIP); err != nil {
			log.Info("waiting for tuner launching...")
			log.Error(err, "tuner is launching")
			return ctrl.Result{Requeue: true}, nil
		}

		// tuner is available
		experiment.Status.Status = mlhubv1.ExperimentRunning
		if _, ok := r.TunerAddrCache[tunerPodName]; !ok {
			r.TunerAddrCache[tunerPodName] = tunerIP
		}
		if err := r.Status().Update(ctx, &experiment); err != nil {
			log.Error(err, "unable to change experiment status to running")
			return ctrl.Result{}, err
		}
		log.Info("change experiment status to running")
		return ctrl.Result{Requeue: true}, nil
	} else if experiment.Status.Status == mlhubv1.ExperimentRunning {
		// judge whether to create a new trail according to the number of running trials.
		if experiment.Spec.Trainer == "tensorflow" {
			tfjobs := tftypes.TFJobList{}
			if err := r.List(context.TODO(), &tfjobs, client.MatchingLabels{"owner": req.Name}); err != nil {
				log.Error(err, "unable to list tfjobs owned by experiment "+req.Name)
				return ctrl.Result{}, err
			}
			// TODO: update each job status in DB
			log.Info("will update each job's status")

			runningJobs := 0
			pendingJobs := 0
			succeedJobs := 0
			failedJobs := 0
			for _, job := range tfjobs.Items {
				if isRunning(job.Status) {
					runningJobs += 1
				} else if isPending(job.Status) {
					pendingJobs += 1
				} else if isSucceeded(job.Status) {
					succeedJobs += 1
				} else if isFailed(job.Status) {
					failedJobs += 1
				}
			}
			log.Info("tfjobs status", "experiment", experiment.Name, "status", fmt.Sprintf("running:%d,pending:%d,failed:%d,succeeded:%d", runningJobs, pendingJobs, failedJobs, succeedJobs))

			if succeedJobs == experiment.Spec.MaxTrialNum {
				// change experiment status to completed
				experiment.Status.Status = mlhubv1.ExperimentCompleted
				if err := r.Status().Update(ctx, &experiment); err != nil {
					log.Error(err, "unable to change experiment status to completed")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else if runningJobs+pendingJobs < experiment.Spec.TrailConcurrency {
				log.Info("will create new trail", "index", *experiment.Status.Count)
				if err := r.submitNewTrial(&ctx, &experiment, log); err != nil {
					log.Error(err, "unable to submit new tensorflow job")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

		} else if experiment.Spec.Trainer == "pytorch" {
			ptjobs := pttypes.PyTorchJobList{}
			if err := r.List(context.TODO(), &ptjobs, client.MatchingLabels{"owner": req.Name}); err != nil {
				log.Error(err, "unable to list ptjobs owned by experiment "+req.Name)
			}
			runningJobs := 0
			pendingJobs := 0
			succeedJobs := 0
			failedJobs := 0
			for _, job := range ptjobs.Items {
				if isRunning(job.Status) {
					runningJobs += 1
				} else if isPending(job.Status) {
					pendingJobs += 1
				} else if isSucceeded(job.Status) {
					succeedJobs += 1
				} else if isFailed(job.Status) {
					failedJobs += 1
				}
			}

			log.Info("ptjobs status", "experiment", experiment.Name, "status", fmt.Sprintf("running:%d,pending:%d,failed:%d,succeeded:%d", runningJobs, pendingJobs, failedJobs, succeedJobs))

			if succeedJobs == experiment.Spec.MaxTrialNum {
				// change experiment status to completed
				experiment.Status.Status = mlhubv1.ExperimentCompleted
				if err := r.Status().Update(ctx, &experiment); err != nil {
					log.Error(err, "unable to change experiment status to completed")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else if runningJobs+pendingJobs < experiment.Spec.TrailConcurrency {
				log.Info("will create new trail", "index", *experiment.Status.Count)
				if err := r.submitNewTrial(&ctx, &experiment, log); err != nil {
					log.Error(err, "unable to submit new tensorflow job")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

		}
	}

	return ctrl.Result{}, nil
}

func (r *DLExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// watch sub resources
	return ctrl.NewControllerManagedBy(mgr).
		For(&mlhubv1.DLExperiment{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
			OwnerType:    &mlhubv1.DLExperiment{},
			IsController: true,
		}).
		Watches(&source.Kind{Type: &tftypes.TFJob{}}, &handler.EnqueueRequestForOwner{
			OwnerType:    &mlhubv1.DLExperiment{},
			IsController: true,
		}).
		Watches(&source.Kind{Type: &pttypes.PyTorchJob{}}, &handler.EnqueueRequestForOwner{
			OwnerType:    &mlhubv1.DLExperiment{},
			IsController: true,
		}).
		Complete(r)
}

func (r *DLExperimentReconciler) submitNewTrial(ctx *context.Context, experiment *mlhubv1.DLExperiment, log logr.Logger) error {
	// Get new param from tuner
	tunerPodName := fmt.Sprintf("%s-tuner", experiment.Name)
	addr, ok := r.TunerAddrCache[tunerPodName]
	if !ok {
		log.Info("tuner ip not in addr cache")
		var tunerPod corev1.Pod
		if err := r.Get(*ctx, client.ObjectKey{
			Namespace: experiment.Namespace,
			Name:      tunerPodName,
		}, &tunerPod); err != nil {
			return err
		}
		addr = tunerPod.Status.PodIP
	}

	newParam, err := utils.GetNewParamFromTuner(addr, *experiment.Status.Count)
	if err != nil {
		return err
	}
	log.Info("get new param for trial", "index", *experiment.Status.Count, "param", newParam)
	// Create new job instance
	trailID := utils.GenerateUniqueString(8)
	reportURL := fmt.Sprintf("http://%s:8001/api/v1/tuner", addr)
	if experiment.Spec.Trainer == "pytorch" {
		job := GeneratePytorchJob(experiment, trailID, reportURL, newParam, *experiment.Status.Count)
		if err := ctrl.SetControllerReference(experiment, job, r.Scheme); err != nil {
			log.Error(err, "uable to set controller reference")
			return err
		}
		log.Info("set controller reference successfully")
		if err := r.Create(*ctx, job); err != nil {
			log.Error(err, "unable to create pytorch job", "experiment", experiment.Name)
			return err
		}
		log.Info("create pytorch job successfully", "experiment", experiment.Name)
		*experiment.Status.Count += 1
		if err := r.Status().Update(*ctx, experiment); err != nil {
			log.Error(err, "unable to update status", "experiment", experiment.Name)
		}
		log.Info("update experiment count", "experiment", experiment.Name, "count", *experiment.Status.Count)
		// TODO: Store in database

	} else if experiment.Spec.Trainer == "tensorflow" {
		job := GenerateTfJob(experiment, trailID, reportURL, newParam, *experiment.Status.Count)
		if err := ctrl.SetControllerReference(experiment, job, r.Scheme); err != nil {
			log.Error(err, "uable to set controller reference")
			return err
		}
		log.Info("set controller reference successfully")
		if err := r.Create(*ctx, job); err != nil {
			log.Error(err, "unable to create tf job", "experiment", experiment.Name)
			return err
		}
		log.Info("create tf job successfully", "experiment", experiment.Name)
		*experiment.Status.Count += 1
		if err := r.Status().Update(*ctx, experiment); err != nil {
			log.Error(err, "unable to update status", "experiment", experiment.Name)
		}
		log.Info("update experiment count", "experiment", experiment.Name, "count", *experiment.Status.Count)
		// TODO: Store in database
		
	} else {
		return fmt.Errorf("unsupported framework: %s", experiment.Spec.Trainer)
	}

	return nil
}
