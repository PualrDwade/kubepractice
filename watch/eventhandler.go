package main

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	util "kubepractice/util"
	"log"
	"time"
)

var stop = make(<-chan struct{})

// 创建服务队kubernetes特定资源（这里是pod）进行watch
func main() {
	// 首先需要创建客户端连接
	client := util.GetClient()
	// 创建Informer工厂，这里是工厂方法模式
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*10)
	// 创建pod informer
	podInformer := informerFactory.Core().V1().Pods()
	// 注册监听事件,ResourceEventHandlerFuncs起到适配器的作用
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// 注册自己需要的函数，这里是reactor编程模型，可以简化大量的代码
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			log.Printf("pod added:%v\n", (*pod).Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*v1.Pod)
			newPod := oldObj.(*v1.Pod)
			log.Printf("pod update:\nold pod:%v\nnew pod:%v\n", (*oldPod).Name, (*newPod).Name)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			log.Printf("pod delete:%v\n", *pod)
		},
	})
	// 启动监听器
	podInformer.Informer().Run(stop)
}
