/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciler

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

func newFakeClient() client.Client {
	scheme := runtime.NewScheme()
	Expect(gamesv1alpha1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

var _ = Describe("Finalizer helpers", func() {
	Describe("HasFinalizer", func() {
		It("should return true when finalizer is present", func() {
			gs := &gamesv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{gamesv1alpha1.GameServerFinalizer},
				},
			}
			Expect(HasFinalizer(gs)).To(BeTrue())
		})

		It("should return false when finalizer is not present", func() {
			gs := &gamesv1alpha1.GameServer{}
			Expect(HasFinalizer(gs)).To(BeFalse())
		})
	})

	Describe("AddFinalizer", func() {
		It("should add the finalizer", func() {
			c := newFakeClient()
			gs := &gamesv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: "default",
				},
			}
			Expect(c.Create(context.Background(), gs)).To(Succeed())

			added, err := AddFinalizer(context.Background(), c, gs)
			Expect(err).NotTo(HaveOccurred())
			Expect(added).To(BeTrue())

			updated := &gamesv1alpha1.GameServer{}
			Expect(c.Get(context.Background(), client.ObjectKeyFromObject(gs), updated)).To(Succeed())
			Expect(HasFinalizer(updated)).To(BeTrue())
		})

		It("should be idempotent and return false if already present", func() {
			c := newFakeClient()
			gs := &gamesv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-server",
					Namespace:  "default",
					Finalizers: []string{gamesv1alpha1.GameServerFinalizer},
				},
			}
			Expect(c.Create(context.Background(), gs)).To(Succeed())

			added, err := AddFinalizer(context.Background(), c, gs)
			Expect(err).NotTo(HaveOccurred())
			Expect(added).To(BeFalse())
		})
	})

	Describe("RemoveFinalizer", func() {
		It("should remove the finalizer", func() {
			c := newFakeClient()
			gs := &gamesv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-server",
					Namespace:  "default",
					Finalizers: []string{gamesv1alpha1.GameServerFinalizer},
				},
			}
			Expect(c.Create(context.Background(), gs)).To(Succeed())

			err := RemoveFinalizer(context.Background(), c, gs)
			Expect(err).NotTo(HaveOccurred())

			updated := &gamesv1alpha1.GameServer{}
			Expect(c.Get(context.Background(), client.ObjectKeyFromObject(gs), updated)).To(Succeed())
			Expect(HasFinalizer(updated)).To(BeFalse())
		})

		It("should do nothing if finalizer is not present", func() {
			c := newFakeClient()
			gs := &gamesv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: "default",
				},
			}
			Expect(c.Create(context.Background(), gs)).To(Succeed())

			err := RemoveFinalizer(context.Background(), c, gs)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
