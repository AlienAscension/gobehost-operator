package adapter

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func TestAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Adapter Suite")
}

func makeGameServer(gameType, version, profile string, server *gamesv1alpha1.ServerSpec) *gamesv1alpha1.GameServer {
	return &gamesv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gs",
			Namespace: "default",
		},
		Spec: gamesv1alpha1.GameServerSpec{
			Game: gamesv1alpha1.GameSpec{
				Type:    gameType,
				Version: version,
				Profile: profile,
			},
			Runtime: gamesv1alpha1.RuntimeSpec{
				Image: "itzg/minecraft-server:latest",
			},
			Storage: gamesv1alpha1.StorageSpec{
				Size:        resource.MustParse("10Gi"),
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
			Network: gamesv1alpha1.NetworkSpec{
				Ports: []gamesv1alpha1.PortSpec{
					{Name: "minecraft", Port: 25565, Protocol: corev1.ProtocolTCP},
				},
			},
			Server: server,
		},
	}
}

var _ = Describe("Adapter Registry", func() {
	It("should have minecraft registered", func() {
		_gs := makeGameServer("minecraft", "1.21", "", nil)
		a, err := Get(_gs)
		Expect(err).NotTo(HaveOccurred())
		Expect(a).NotTo(BeNil())
		Expect(a.Name()).To(Equal("minecraft"))
	})

	It("should return error for unknown game type", func() {
		gs := makeGameServer("unknown-game", "1.0", "", nil)
		_, err := Get(gs)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no adapter registered for game type"))
	})

	It("should include minecraft in known games", func() {
		games := KnownGames()
		Expect(games).To(ContainElement("minecraft"))
	})
})

var _ = Describe("MinecraftAdapter", func() {
	var adapter *MinecraftAdapter

	BeforeEach(func() {
		adapter = &MinecraftAdapter{}
	})

	It("should return minecraft as name", func() {
		Expect(adapter.Name()).To(Equal("minecraft"))
	})

	It("should return /data as data path", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		Expect(adapter.DataPath(gs)).To(Equal("/data"))
	})

	It("should return nil for Command", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		Expect(adapter.Command(gs)).To(BeNil())
	})

	It("should return nil for Args", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		Expect(adapter.Args(gs)).To(BeNil())
	})

	It("should return readiness and liveness probes with correct ports", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		readiness, liveness := adapter.Probes(gs)
		Expect(readiness).NotTo(BeNil())
		Expect(liveness).NotTo(BeNil())
		Expect(readiness.ProbeHandler.TCPSocket).NotTo(BeNil())
		Expect(liveness.ProbeHandler.TCPSocket).NotTo(BeNil())
		Expect(readiness.InitialDelaySeconds).To(Equal(int32(30)))
		Expect(readiness.PeriodSeconds).To(Equal(int32(10)))
		Expect(readiness.FailureThreshold).To(Equal(int32(10)))
		Expect(liveness.InitialDelaySeconds).To(Equal(int32(120)))
		Expect(liveness.PeriodSeconds).To(Equal(int32(30)))
		Expect(liveness.FailureThreshold).To(Equal(int32(5)))
	})

	It("should set TYPE=PAPER for paper profile", func() {
		gs := makeGameServer("minecraft", "1.21", "paper", nil)
		env := adapter.Env(gs)
		names := envNames(env)
		Expect(names).To(ContainElement("TYPE"))
		Expect(envValue(env, "TYPE")).To(Equal("PAPER"))
	})

	It("should set EULA=TRUE and VERSION", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		env := adapter.Env(gs)
		Expect(envValue(env, "EULA")).To(Equal("TRUE"))
		Expect(envValue(env, "VERSION")).To(Equal("1.21"))
	})

	It("should set TYPE=VANILLA when profile is empty", func() {
		gs := makeGameServer("minecraft", "1.21", "", nil)
		env := adapter.Env(gs)
		Expect(envValue(env, "TYPE")).To(Equal("VANILLA"))
	})

	It("should map ServerSpec fields to env vars", func() {
		gs := makeGameServer("minecraft", "1.21", "", &gamesv1alpha1.ServerSpec{
			MaxPlayers: ptr.To(int32(20)),
			Motd:       "Welcome",
			Difficulty: "hard",
			GameMode:   "survival",
			Pvp:        ptr.To(true),
			OnlineMode: ptr.To(false),
		})
		env := adapter.Env(gs)
		Expect(envValue(env, "MAX_PLAYERS")).To(Equal("20"))
		Expect(envValue(env, "MOTD")).To(Equal("Welcome"))
		Expect(envValue(env, "DIFFICULTY")).To(Equal("hard"))
		Expect(envValue(env, "MODE")).To(Equal("survival"))
		Expect(envValue(env, "PVP")).To(Equal("true"))
		Expect(envValue(env, "ONLINE_MODE")).To(Equal("false"))
	})

	It("should set LEVEL env var from LevelName", func() {
		gs := makeGameServer("minecraft", "1.21", "", &gamesv1alpha1.ServerSpec{
			LevelName: "myworld",
		})
		env := adapter.Env(gs)
		Expect(envValue(env, "LEVEL")).To(Equal("myworld"))
	})
})

func envNames(env []corev1.EnvVar) []string {
	names := make([]string, 0, len(env))
	for _, e := range env {
		names = append(names, e.Name)
	}
	return names
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, e := range env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}
