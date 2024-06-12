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
		imgName, imgTag := SplitImageNameAndTag(container.Image)
		if len(imgTag) == 0 {
			log.Printf("Invalid image format: %s", container.Image)
			continue
		}
		log.Printf("onAddOrUpdateDeployment: %s", container.Image)
		key := ResourceKey{
			Namespace:     deploy.Namespace,
			ResourceName:  deploy.Name,
			ResourceType:  "Deployment",
			ContainerName: container.Name,
			ImageName:     imgName,
			ImageTag:      imgTag,
		}
		caches, ok := ws.ImageCache[imgName]
		if ok {
			for i, cache := range caches {
				if cache.ImageName == imgName && cache.ImageTag != imgTag {
					ws.ImageCache[imgName][i] = key
					break
				}
			}
		} else {
			ws.ImageCache[imgName] = append(ws.ImageCache[imgName], key)
		}
	}
}

func (ws *WatcherService) onDeleteDeployment(obj interface{}) {
	deploy := obj.(*appsv1.Deployment)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range deploy.Spec.Template.Spec.Containers {
		imgName, imgTag := SplitImageNameAndTag(container.Image)
		if len(imgTag) == 0 {
			log.Printf("Invalid image format: %s", container.Image)
			continue
		}
		log.Printf("onDeleteDeployment: %s", container.Image)
		cache := removeElement(ws.ImageCache[imgName], imgTag)
		if len(cache) != 0 {
			ws.ImageCache[imgName] = cache
			continue
		}
		delete(ws.ImageCache, imgName)
	}
}

func (ws *WatcherService) onAddOrUpdateStatefulSet(obj interface{}) {
	ss := obj.(*appsv1.StatefulSet)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range ss.Spec.Template.Spec.Containers {
		imgName, imgTag := SplitImageNameAndTag(container.Image)
		if len(imgTag) == 0 {
			log.Printf("Invalid image format: %s", container.Image)
			continue
		}
		log.Printf("AddOrUpdateStatefulSet: %s", container.Image)
		key := ResourceKey{
			Namespace:     ss.Namespace,
			ResourceName:  ss.Name,
			ResourceType:  "StatefulSet",
			ContainerName: container.Name,
			ImageName:     imgName,
			ImageTag:      imgTag,
		}
		caches, ok := ws.ImageCache[imgName]
		if ok {
			for i, cache := range caches {
				if cache.ImageName == imgName && cache.ImageTag != imgTag {
					ws.ImageCache[imgName][i] = key
					break
				}
			}
		} else {
			ws.ImageCache[imgName] = append(ws.ImageCache[imgName], key)
		}
	}
}

func (ws *WatcherService) onDeleteStatefulSet(obj interface{}) {
	ss := obj.(*appsv1.StatefulSet)
	ws.cacheMutex.Lock()
	defer ws.cacheMutex.Unlock()
	for _, container := range ss.Spec.Template.Spec.Containers {
		imgName, imgTag := SplitImageNameAndTag(container.Image)
		if len(imgTag) == 0 {
			log.Printf("Invalid image format: %s", container.Image)
			continue
		}
		log.Printf("onDeleteStatefulSet: %s", container.Image)
		cache := removeElement(ws.ImageCache[imgName], imgTag)
		if len(cache) != 0 {
			ws.ImageCache[imgName] = cache
			continue
		}
		delete(ws.ImageCache, imgName)
	}
}

func (ws *WatcherService) handleUpdateEvents() {
	defer func() {
		ws.UpdateHandlerDone <- true
	}()
	for event := range ws.UpdateHandlerChan {
		for _, image := range event.Data.Resources {
			newName, newTag := SplitImageNameAndTag(image.ResourceURL)
			if len(newTag) == 0 {
				log.Printf("Invalid image format: %s", image.ResourceURL)
				continue
			}
			// deployment滚动更新后缓存就会更新
			keys, exists := ws.ImageCache[newName]
			if exists {
				for _, key := range keys {
					if key.ImageTag == newTag {
						fmt.Printf("(%s)Image already exists: %s:%s\n", key.ContainerName, key.ImageName, key.ImageTag)
						continue
					}

					// 滚动更新K8s实例
					ws.updateK8sInstance(key, image.ResourceURL)
					fmt.Printf("(%s)Image updated: %s:%s -> %s\n", key.ContainerName, key.ImageName, key.ImageTag, image.ResourceURL)
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
	var result []ResourceKey

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

// SplitImageNameAndTag 根据最后一个':'分隔符分割镜像字符串，返回镜像名称和标签。
func SplitImageNameAndTag(image string) (string, string) {
	// 检查输入是否合法
	if image == "" {
		return "", ""
	}

	// 找到最后一个':'的位置
	lastColonIndex := strings.LastIndex(image, ":")

	// 如果没有找到':'，则可能是hash结尾, 即 @ 为分隔符
	if lastColonIndex == -1 {
		lastAtIndex := strings.LastIndex(image, "@")
		if lastAtIndex == -1 {
			return image, ""
		}
		name := image[:lastAtIndex]
		tag := image[lastAtIndex+1:]
		return name, tag
	}

	// 分别获取name和tag
	name := image[:lastColonIndex]
	tag := image[lastColonIndex+1:]

	return name, tag
}
