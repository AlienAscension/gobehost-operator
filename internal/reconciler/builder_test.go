package reconciler

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
	_ "github.com/gobehost/operator/internal/adapter"
)

func TestReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Builder Suite")
}

func makeTestGameServer() *gamesv1alpha1.GameServer {
	return &gamesv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: gamesv1alpha1.GameServerSpec{
			Game: gamesv1alpha1.GameSpec{
				Type:    "minecraft",
				Version: "1.21",
			},
			Runtime: gamesv1alpha1.RuntimeSpec{
				Image:           "itzg/minecraft-server:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			Storage: gamesv1alpha1.StorageSpec{
				Size: resource.MustParse("10Gi"),
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
			},
			Network: gamesv1alpha1.NetworkSpec{
				Ports: []gamesv1alpha1.PortSpec{
					{
						Name:     "minecraft",
						Port:     25565,
						Protocol: corev1.ProtocolTCP,
					},
				},
				ServiceType: corev1.ServiceTypeLoadBalancer,
			},
		},
	}
}

var _ = Describe("BuildPVC", func() {
	It("should create PVC with correct name and namespace", func() {
		gs := makeTestGameServer()
		pvc := BuildPVC(gs)
		Expect(pvc.Name).To(Equal("test-server-data"))
		Expect(pvc.Namespace).To(Equal("default"))
	})

	It("should set storage size from spec", func() {
		gs := makeTestGameServer()
		pvc := BuildPVC(gs)
		Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("10Gi")))
	})

	It("should set access modes from spec", func() {
		gs := makeTestGameServer()
		pvc := BuildPVC(gs)
		Expect(pvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
	})

	It("should set storage class when specified", func() {
		gs := makeTestGameServer()
		gs.Spec.Storage.StorageClass = ptr.To("local-path")
		pvc := BuildPVC(gs)
		Expect(pvc.Spec.StorageClassName).NotTo(BeNil())
		Expect(*pvc.Spec.StorageClassName).To(Equal("local-path"))
	})

	It("should leave storage class nil when not specified", func() {
		gs := makeTestGameServer()
		pvc := BuildPVC(gs)
		Expect(pvc.Spec.StorageClassName).To(BeNil())
	})

	It("should set labels from GameServerLabels", func() {
		gs := makeTestGameServer()
		pvc := BuildPVC(gs)
		Expect(pvc.Labels["app.kubernetes.io/name"]).To(Equal("gameserver"))
		Expect(pvc.Labels["app.kubernetes.io/instance"]).To(Equal("test-server"))
		Expect(pvc.Labels["games.gobehost.com/game-type"]).To(Equal("minecraft"))
	})
})

var _ = Describe("BuildService", func() {
	It("should create Service with correct name and namespace", func() {
		gs := makeTestGameServer()
		svc := BuildService(gs)
		Expect(svc.Name).To(Equal("test-server"))
		Expect(svc.Namespace).To(Equal("default"))
	})

	It("should set service type from spec", func() {
		gs := makeTestGameServer()
		svc := BuildService(gs)
		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
	})

	It("should set ports from spec", func() {
		gs := makeTestGameServer()
		svc := BuildService(gs)
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Name).To(Equal("minecraft"))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(25565)))
	})

	It("should set selector from GameServerLabels", func() {
		gs := makeTestGameServer()
		svc := BuildService(gs)
		Expect(svc.Spec.Selector["app.kubernetes.io/name"]).To(Equal("gameserver"))
	})

	It("should set annotations from spec", func() {
		gs := makeTestGameServer()
		gs.Spec.Network.Annotations = map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"}
		svc := BuildService(gs)
		Expect(svc.Annotations).To(HaveKeyWithValue("service.beta.kubernetes.io/aws-load-balancer-type", "nlb"))
	})
})

var _ = Describe("BuildHeadlessService", func() {
	It("should create headless Service with ClusterIP None", func() {
		gs := makeTestGameServer()
		svc := BuildHeadlessService(gs)
		Expect(svc.Name).To(Equal("test-server-headless"))
		Expect(svc.Spec.ClusterIP).To(Equal("None"))
		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
	})

	It("should set the same ports and selector as main service", func() {
		gs := makeTestGameServer()
		svc := BuildHeadlessService(gs)
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Selector["app.kubernetes.io/name"]).To(Equal("gameserver"))
	})
})

var _ = Describe("BuildStatefulSet", func() {
	It("should create StatefulSet with correct name and image", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(sts.Name).To(Equal("test-server"))
		Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("itzg/minecraft-server:latest"))
	})

	It("should set replicas to 1", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
	})

	It("should set service name to gs.Name + -headless", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(sts.Spec.ServiceName).To(Equal("test-server-headless"))
	})

	It("should set container name to server", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(sts.Spec.Template.Spec.Containers[0].Name).To(Equal("server"))
	})

	It("should include adapter env vars", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		env := sts.Spec.Template.Spec.Containers[0].Env
		Expect(envValue(env, "EULA")).To(Equal("TRUE"))
		Expect(envValue(env, "VERSION")).To(Equal("1.21"))
	})

	It("should include runtime env vars after adapter env vars", func() {
		gs := makeTestGameServer()
		gs.Spec.Runtime.Env = []corev1.EnvVar{{Name: "CUSTOM_VAR", Value: "custom"}}
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		env := sts.Spec.Template.Spec.Containers[0].Env
		Expect(envValue(env, "CUSTOM_VAR")).To(Equal("custom"))
	})

	It("should return error for unknown game type", func() {
		gs := makeTestGameServer()
		gs.Spec.Game.Type = "unknown-game"
		_, err := BuildStatefulSet(gs)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no adapter registered for game type"))
	})

	It("should set volume mount from adapter data path", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		vm := sts.Spec.Template.Spec.Containers[0].VolumeMounts[0]
		Expect(vm.Name).To(Equal("data"))
		Expect(vm.MountPath).To(Equal("/data"))
	})

	It("should set probes from adapter", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		container := sts.Spec.Template.Spec.Containers[0]
		Expect(container.ReadinessProbe).NotTo(BeNil())
		Expect(container.LivenessProbe).NotTo(BeNil())
	})

	It("should set termination grace period", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(*sts.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(int64(120)))
	})

	It("should have volume claim templates", func() {
		gs := makeTestGameServer()
		sts, err := BuildStatefulSet(gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(sts.Spec.VolumeClaimTemplates).To(HaveLen(1))
		Expect(sts.Spec.VolumeClaimTemplates[0].Name).To(Equal("data"))
	})
})

var _ = Describe("GameServerLabels", func() {
	It("should return correct labels", func() {
		gs := makeTestGameServer()
		labels := GameServerLabels(gs)
		Expect(labels["app.kubernetes.io/name"]).To(Equal("gameserver"))
		Expect(labels["app.kubernetes.io/instance"]).To(Equal("test-server"))
		Expect(labels["app.kubernetes.io/managed-by"]).To(Equal("gobehost-operator"))
		Expect(labels["games.gobehost.com/game-type"]).To(Equal("minecraft"))
		Expect(labels["games.gobehost.com/game-id"]).To(Equal("test-server"))
	})
})

func envValue(env []corev1.EnvVar, name string) string {
	for _, e := range env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

var _ = Describe("buildSecurityContext", func() {
	It("should drop ALL capabilities by default", func() {
		gs := makeTestGameServer()
		sc := buildSecurityContext(gs)
		Expect(sc.Capabilities).NotTo(BeNil())
		Expect(sc.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
	})

	It("should set RunAsNonRoot when specified", func() {
		gs := makeTestGameServer()
		gs.Spec.Security = &gamesv1alpha1.SecuritySpec{
			RunAsNonRoot: ptr.To(true),
		}
		sc := buildSecurityContext(gs)
		Expect(*sc.RunAsNonRoot).To(BeTrue())
	})
})

var _ = Describe("applyScheduling", func() {
	It("should apply nodeSelector", func() {
		gs := makeTestGameServer()
		gs.Spec.Scheduling = &gamesv1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
		}
		podSpec := &corev1.PodSpec{}
		applyScheduling(gs, podSpec)
		Expect(podSpec.NodeSelector).To(HaveKeyWithValue("kubernetes.io/os", "linux"))
	})

	It("should not modify pod spec when scheduling is nil", func() {
		gs := makeTestGameServer()
		podSpec := &corev1.PodSpec{}
		applyScheduling(gs, podSpec)
		Expect(podSpec.NodeSelector).To(BeEmpty())
	})
})
