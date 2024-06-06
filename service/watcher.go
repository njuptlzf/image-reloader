package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/njuptlzf/image-reloader/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ResourceKey struct {
	Namespace     string
	ResourceName  string
	ResourceType  string
	ContainerName string
	ImageName     string
	ImageTag      string
}

type WatcherService struct {
	clientset         *kubernetes.Clientset
	ImageCache        map[string][]ResourceKey
	cacheMutex        sync.RWMutex
	UpdateHandlerChan chan model.PushEvent
	// 定义信号管道，UpdateHandlerChan处理完就置传入true
	UpdateHandlerDone chan bool
}

// NewKubernetesClient 创建一个新的 Kubernetes 客户端
func NewKubernetesClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	// 优先尝试 InCluster 配置
	config, err = rest.InClusterConfig()
	if err != nil {
		// 如果 InCluster 配置获取失败，则尝试环境变量和 kubeconfig 文件
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create Kubernetes client config: %v", err)
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes Clientset: %v", err)
		return nil, err
	}

	return clientset, nil
}

func NewWatcherService() *WatcherService {
	clientset, err := NewKubernetesClient()
	if err != nil {
		panic(err.Error())
	}

	return &WatcherService{
		clientset:         clientset,
		ImageCache:        make(map[string][]ResourceKey),
		UpdateHandlerChan: make(chan model.PushEvent),
		UpdateHandlerDone: make(chan bool),
	}
}

func (ws *WatcherService) StartWatcher() {
	factory := informers.NewSharedInformerFactory(ws.clientset, 30*time.Second)
	deployInformer := factory.Apps().V1().Deployments().Informer()
	statefulsetInformer := factory.Apps().V1().StatefulSets().Informer()

	stop := make(chan struct{})

	deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ws.onAddOrUpdateDeployment,
		UpdateFunc: func(oldObj, newObj interface{}) {
			ws.onAddOrUpdateDeployment(newObj)
		},
		DeleteFunc: ws.onDeleteDeployment,
	})

	statefulsetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: ws.onAddOrUpdateStatefulSet,
		UpdateFunc: func(oldObj, newObj interface{}) {
			ws.onAddOrUpdateStatefulSet(newObj)
		},
		DeleteFunc: ws.onDeleteStatefulSet,
	})

	factory.Start(stop)
	factory.WaitForCacheSync(stop)

	// 启动事件处理协程
	go ws.handleUpdateEvents()

	<-stop
}

func (ws *WatcherService) onAddOrUpdateDeployment(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range deploy.Spec.Template.Spec.Containers {
		// 将镜像拆分成name和tag，保存到key
		list := strings.Split(container.Image, ":")
		if len(list) != 2 {
			list = strings.Split(container.Image, "@")
			if len(list) != 2 {
				log.Printf("Invalid image format: %s", container.Image)
				continue
			}
		}
		imgName, imgTag := list[0], list[1]
		key := ResourceKey{
			Namespace:     deploy.Namespace,
			ResourceName:  deploy.Name,
			ResourceType:  "Deployment",
			ContainerName: container.Name,
			ImageName:     imgName,
			ImageTag:      imgTag,
		}
		for i, cache := range ws.ImageCache[imgName] {
			if cache.ImageName == imgName && cache.ImageTag != imgTag {
				ws.ImageCache[imgName][i] = key
				break
			}
		}
	}
}

func (ws *WatcherService) onDeleteDeployment(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range deploy.Spec.Template.Spec.Containers {
		list := strings.Split(container.Image, ":")
		if len(list) != 2 {
			list = strings.Split(container.Image, "@")
			if len(list) != 2 {
				log.Printf("Invalid image format: %s", container.Image)
				continue
			}
		}
		imgName, imgTag := list[0], list[1]
		ws.ImageCache[imgName] = removeElement(ws.ImageCache[imgName], imgTag)
	}
}

func (ws *WatcherService) onAddOrUpdateStatefulSet(obj interface{}) {
	ss := obj.(*appsv1.StatefulSet)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range ss.Spec.Template.Spec.Containers {
		// 将镜像拆分成name和tag，保存到key
		list := strings.Split(container.Image, ":")
		if len(list) != 2 {
			list = strings.Split(container.Image, "@")
			if len(list) != 2 {
				log.Printf("Invalid image format: %s", container.Image)
				continue
			}
		}
		imgName, imgTag := list[0], list[1]
		key := ResourceKey{
			Namespace:     ss.Namespace,
			ResourceName:  ss.Name,
			ResourceType:  "StatefulSet",
			ContainerName: container.Name,
			ImageName:     imgName,
			ImageTag:      imgTag,
		}
		for i, cache := range ws.ImageCache[imgName] {
			if cache.ImageName == imgName && cache.ImageTag != imgTag {
				ws.ImageCache[imgName][i] = key
				break
			}
		}
	}
}

func (ws *WatcherService) onDeleteStatefulSet(obj interface{}) {
	ss := obj.(*appsv1.StatefulSet)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range ss.Spec.Template.Spec.Containers {
		list := strings.Split(container.Image, ":")
		if len(list) != 2 {
			list = strings.Split(container.Image, "@")
			if len(list) != 2 {
				log.Printf("Invalid image format: %s", container.Image)
				continue
			}
		}
		imgName, imgTag := list[0], list[1]
		ws.ImageCache[imgName] = removeElement(ws.ImageCache[imgName], imgTag)
	}
}

func (ws *WatcherService) handleUpdateEvents() {
	defer func() {
		ws.UpdateHandlerDone <- true
	}()
	for event := range ws.UpdateHandlerChan {
		for _, image := range event.Resources {
			list := strings.Split(image.ResourceURL, ":")
			if len(list) != 2 {
				list = strings.Split(image.ResourceURL, "@")
				if len(list) != 2 {
					log.Printf("Invalid image format: %s", image.ResourceURL)
					continue
				}
			}
			// 目前只要前缀相同且tag不同就更新
			newName, newTag := list[0], list[1]
			// deployment滚动更新后缓存会更新
			keys, exists := ws.ImageCache[newName]
			if exists {
				for _, key := range keys {
					if key.ImageTag == newTag {
						fmt.Printf("Image already exists: %s/%s:%s\n", key.Namespace, key.ResourceName, key.ContainerName)
						continue
					}

					// 滚动更新K8s实例
					ws.updateK8sInstance(key, image.ResourceURL)
					fmt.Printf("Image updated: %s/%s:%s -> %s\n", key.Namespace, key.ResourceName, key.ContainerName, image.ResourceURL)
				}
			}
		}
		ws.UpdateHandlerDone <- true
	}
}

func (ws *WatcherService) updateK8sInstance(key ResourceKey, newImage string) {
	if key.ResourceType == "Deployment" {
		deployment, err := ws.clientset.AppsV1().Deployments(key.Namespace).Get(context.TODO(), key.ResourceName, metav1.GetOptions{})
		if err == nil {
			for i, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == key.ContainerName {
					deployment.Spec.Template.Spec.Containers[i].Image = newImage
					break
				}
			}
			ws.clientset.AppsV1().Deployments(key.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		}
	}

	if key.ResourceType == "StatefulSet" {
		statefulSet, err := ws.clientset.AppsV1().StatefulSets(key.Namespace).Get(context.TODO(), key.ResourceName, metav1.GetOptions{})
		if err == nil {
			for i, container := range statefulSet.Spec.Template.Spec.Containers {
				if container.Name == key.ContainerName {
					statefulSet.Spec.Template.Spec.Containers[i].Image = newImage
					break
				}
			}
			ws.clientset.AppsV1().StatefulSets(key.Namespace).Update(context.TODO(), statefulSet, metav1.UpdateOptions{})
		}
	}
}

// 删除等于指定值的元素
func removeElement(arr []ResourceKey, value string) []ResourceKey {
	// 创建一个新的切片，用于存储不等于指定值的元素
	result := []ResourceKey{}

	// 遍历原始切片，检查每个元素
	for _, v := range arr {
		// 如果元素不等于指定值，则将其添加到结果切片中
		if v.ImageTag != value {
			result = append(result, v)
		}
	}

	// 返回结果切片
	return result
}
