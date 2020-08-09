package k8shandler

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	loggingv1 "github.com/openshift/cluster-logging-operator/pkg/apis/logging/v1"
	"github.com/openshift/cluster-logging-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Reconciling", func() {
	defer GinkgoRecover()

	_ = loggingv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	_ = monitoringv1.AddToScheme(scheme.Scheme)

	var (
		cluster = &loggingv1.ClusterLogging{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance",
				Namespace: constants.OpenshiftNS,
			},
			Spec: loggingv1.ClusterLoggingSpec{
				ManagementState: loggingv1.ManagementStateManaged,
				Collection: &loggingv1.CollectionSpec{
					Logs: loggingv1.LogCollectionSpec{
						Type:        loggingv1.LogCollectionTypeFluentd,
						FluentdSpec: loggingv1.FluentdSpec{},
					},
				},
			},
		}
		fluentdSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fluentd",
				Namespace: cluster.GetNamespace(),
			},
		}
		fluentdCABundle = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.FluentdTrustedCAName,
				Namespace: cluster.GetNamespace(),
				Labels: map[string]string{
					constants.InjectTrustedCABundleLabel: "true",
				},
			},
			Data: map[string]string{
				constants.TrustedCABundleKey: `
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT>
                  -----END CERTIFICATE-------
                `,
			},
		}
		proxy = &configv1.Proxy{
			Spec: configv1.ProxySpec{
				TrustedCA: configv1.ConfigMapNameReference{
					Name: "custom-ca-bundle",
				},
			},
		}
	)

	Describe("Collection", func() {
		var (
			client         client.Client
			clusterRequest *ClusterLoggingRequest
		)

		Context("when cluster proxy present", func() {
			var (
				customCABundle = `
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT1>
                  -----END CERTIFICATE-------
                  -----BEGIN CERTIFICATE-----
                  <PEM_ENCODED_CERT2>
                  -----END CERTIFICATE-------
                `
				trustedCABundleVolume = corev1.Volume{
					Name: constants.FluentdTrustedCAName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.FluentdTrustedCAName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  constants.TrustedCABundleKey,
									Path: constants.TrustedCABundleMountFile,
								},
							},
						},
					},
				}
				trustedCABundleVolumeMount = corev1.VolumeMount{
					Name:      constants.FluentdTrustedCAName,
					ReadOnly:  true,
					MountPath: constants.TrustedCABundleMountDir,
				}
			)
			BeforeEach(func() {
				client = fake.NewFakeClient(
					cluster,
					fluentdSecret,
					fluentdCABundle,
				)
				clusterRequest = &ClusterLoggingRequest{
					Client:  client,
					Cluster: cluster,
				}
			})

			It("should use the default CA bundle in fluentd", func() {
				Expect(clusterRequest.CreateOrUpdateCollection(proxy)).Should(Succeed())

				key := types.NamespacedName{Name: constants.FluentdTrustedCAName, Namespace: cluster.GetNamespace()}
				fluentdCaBundle := &corev1.ConfigMap{}
				Expect(client.Get(context.TODO(), key, fluentdCaBundle)).Should(Succeed())
				Expect(fluentdCABundle.Data).To(Equal(fluentdCaBundle.Data))

				key = types.NamespacedName{Name: constants.FluentdName, Namespace: cluster.GetNamespace()}
				ds := &appsv1.DaemonSet{}
				Expect(client.Get(context.TODO(), key, ds)).Should(Succeed())

				trustedCABundleHash := ds.Spec.Template.Annotations[constants.TrustedCABundleHashName]
				Expect(calcTrustedCAHashValue(fluentdCABundle)).To(Equal(trustedCABundleHash))
				Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(trustedCABundleVolume))
				Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(trustedCABundleVolumeMount))
			})

			It("should use the injected custom CA bundle in fluentd", func() {
				// Reconcile w/o custom CA bundle
				Expect(clusterRequest.CreateOrUpdateCollection(proxy)).To(Succeed())

				// Inject custom CA bundle into fluentd config map
				injectedCABundle := fluentdCABundle.DeepCopy()
				injectedCABundle.Data[constants.TrustedCABundleKey] = customCABundle
				Expect(client.Update(context.TODO(), injectedCABundle)).Should(Succeed())

				// Reconcile with injected custom CA bundle
				Expect(clusterRequest.CreateOrUpdateCollection(proxy)).Should(Succeed())

				key := types.NamespacedName{Name: constants.FluentdName, Namespace: cluster.GetNamespace()}
				ds := &appsv1.DaemonSet{}
				Expect(client.Get(context.TODO(), key, ds)).Should(Succeed())

				trustedCABundleHash := ds.Spec.Template.Annotations[constants.TrustedCABundleHashName]
				Expect(calcTrustedCAHashValue(injectedCABundle)).To(Equal(trustedCABundleHash))
				Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(trustedCABundleVolume))
				Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(trustedCABundleVolumeMount))
			})
		})
	})
})
