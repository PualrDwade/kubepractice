package uitl

import (
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	once   sync.Once
	client *kubernetes.Clientset
)

// 返回home目录
func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// 使用单例模式提供全局唯一客户端
func GetClient() *kubernetes.Clientset {
	once.Do(func() {
		var kubeconfig *string
		if home := HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()
		config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatal(err.Error())
		}
		//随后利用client-go来创建自己的资源
		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatal(err.Error())
		}
	})
	return client
}
