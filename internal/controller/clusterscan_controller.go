package controller

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scanv1 "example.com/Clusterscan/api/v1"
)

type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ClusterScan instance
	var clusterScan scanv1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterScan resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterScan")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling ClusterScan", "ClusterScan.Namespace", clusterScan.Namespace, "ClusterScan.Name", clusterScan.Name)

	// Determine if it's a one-off or recurring scan
	if clusterScan.Spec.Schedule == "" {
		log.Info("Creating a one-off Job", "ClusterScan.Namespace", clusterScan.Namespace, "ClusterScan.Name", clusterScan.Name)
		// One-off Job
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterScan.Name + "-job",
				Namespace: clusterScan.Namespace,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:    "scan",
							Image:   "alpine",
							Command: clusterScan.Spec.Command,
						}},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}

		// Update ClusterScan status
		clusterScan.Status.JobName = job.Name
		clusterScan.Status.LastRun = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, &clusterScan); err != nil {
			log.Error(err, "Failed to update ClusterScan status")
			return ctrl.Result{}, err
		}
		log.Info("Updated ClusterScan status", "ClusterScan.Namespace", clusterScan.Namespace, "ClusterScan.Name", clusterScan.Name)
	} else {
		log.Info("Creating a recurring CronJob", "ClusterScan.Namespace", clusterScan.Namespace, "ClusterScan.Name", clusterScan.Name)
		// Recurring CronJob
		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterScan.Name + "-cronjob",
				Namespace: clusterScan.Namespace,
			},
			Spec: batchv1.CronJobSpec{
				Schedule: clusterScan.Spec.Schedule,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:    "scan",
									Image:   "alpine",
									Command: clusterScan.Spec.Command,
								}},
								RestartPolicy: corev1.RestartPolicyNever,
							},
						},
					},
				},
			},
		}

		// Update ClusterScan status
		clusterScan.Status.JobName = cronJob.Name
		if err := r.Status().Update(ctx, &clusterScan); err != nil {
			log.Error(err, "Failed to update ClusterScan status")
			return ctrl.Result{}, err
		}
		log.Info("Updated ClusterScan status", "ClusterScan.Namespace", clusterScan.Namespace, "ClusterScan.Name", clusterScan.Name)
	}

	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanv1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
