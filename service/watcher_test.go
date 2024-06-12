package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/njuptlzf/image-reloader/handler"
	"github.com/njuptlzf/image-reloader/model"
	"github.com/njuptlzf/image-reloader/service"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func TestHandleUpdateEvents(t *testing.T) {
	ws := handler.InitWatcherService()
	clientset := fake.NewSimpleClientset()
	imagePrefix := "core.harbor.domain/library/nginx"
	imageTag := "1.14.2"
	newTag := "1.14.2-1"
	deployName := "nginx"
	namespace := "nginx"
	completedName := fmt.Sprintf("%s:%s", imagePrefix, imageTag)
	completedNewName := fmt.Sprintf("%s:%s", imagePrefix, newTag)
	// 模拟一个deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: completedName,
						},
					},
				},
			},
		},
	}

	// // 模拟一个statefulset
	// statefulSet := &appsv1.StatefulSet{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "nginx-statefulset",
	// 		Namespace: "default",
	// 	},
	// 	Spec: appsv1.StatefulSetSpec{
	// 		Template: v1.PodTemplateSpec{
	// 			Spec: v1.PodSpec{
	// 				Containers: []v1.Container{
	// 					{
	// 						Name:  "nginx",
	// 						Image: completedName,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// 创建资源
	clientset.AppsV1().Deployments("default").Create(context.TODO(), deployment, metav1.CreateOptions{})
	// clientset.AppsV1().StatefulSets("default").Create(context.TODO(), statefulSet, metav1.CreateOptions{})

	// 向UpdateHandlerChan发送一个新的push event
	event := model.PushEvent{
		Data: model.Data{
			Resources: []model.Image{{Digest: "sha256:dummy", Tag: newTag, ResourceURL: completedNewName}},
			Repository: struct {
				DateCreated  int64  `json:"date_created"`
				Name         string `json:"name"`
				Namespace    string `json:"namespace"`
				RepoFullName string `json:"repo_full_name"`
				RepoType     string `json:"repo_type"`
			}{
				Name:      deployName,
				Namespace: namespace,
			},
		},
	}

	// 模拟缓存
	key := service.ResourceKey{
		Namespace:     namespace,
		ResourceName:  deployName,
		ResourceType:  "Deployment",
		ContainerName: "nginx",
		ImageName:     imagePrefix,
		ImageTag:      imageTag}
	ws.ImageCache[imagePrefix] = append(ws.ImageCache[imagePrefix], key)

	go func() {
		ws.UpdateHandlerChan <- event
	}()

	// 等待事件处理
	<-ws.UpdateHandlerDone

	time.Sleep(10 * time.Second)
	// 检查K8s资源更新
	checkDeploymentImage(t, clientset.AppsV1().Deployments(namespace), deployName, completedNewName)
}

func checkDeploymentImage(t *testing.T, d corev1.DeploymentInterface, name, expectedImage string) {
	dep, err := d.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Image != expectedImage {
			t.Errorf("Expected image %s but got %s", expectedImage, container.Image)
		}
	}
}

// func checkStatefulSetImage(t *testing.T, s corev1.StatefulSetInterface, name, expectedImage string) {
// 	sts, err := s.Get(context.TODO(), name, metav1.GetOptions{})
// 	if err != nil {
// 		t.Fatalf("Failed to get statefulset: %v", err)
// 	}
// 	for _, container := range sts.Spec.Template.Spec.Containers {
// 		if container.Image != expectedImage {
// 			t.Errorf("Expected image %s but got %s", expectedImage, container.Image)
// 		}
// 	}
// }
