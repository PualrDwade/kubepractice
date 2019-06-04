# Kubernetes client-go实战

kubernetes 提供了丰富的客户端操作选项，我们可以直接通过http请求来与API Server交互，也可以使用kubectl。这两种方式使用简单，但是缺乏可定制性。如果我们需要在自己的业务逻辑中与kubernetes集群进行通信等操作，我们希望一个本地客户端充当代码调用stub，对于go语言而言，由于kubernetes本身就是利用golang编写而成，因此官方就提供了客户端SDK:client-go。本质上其实就是对REST以及其他请求的封装。

在代码中，与kubernetes通信主要具有两个部分，对应两种不同的编程模型

- 主动对kubernetes进行操作，自定义业务逻辑。
- 被动监听kubernetes资源状态变化，基于事件的隐式调用。

## 主动对集群操作

首先我们要想与kubernetes通信，必然需要创建出一个客户端的连接才行，我们需要告诉它到底和哪个集群相连。默认会使用HOME目录下的.kube配置,创建客户端连接代码如下:

*kubepractice/util/util.go*

```go
var (
	once   sync.Once //实例化一次
	client *kubernetes.Clientset
)

// 返回home目录
func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h //linux
	}
	return os.Getenv("USERPROFILE") // windows
}

// 使用单例模式提供全局客户端连接
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
```

`GetClient()`方法返回客户端连接的指针，之后我们便可以通过client操作kubernetes集群了。

为了简单，这里演示如何通过client来操作kubernetes中的deployment资源,实现如下功能:

- 控制台传入相关参数，更新指定deployment中选定容器的镜像。
- 通过编码实现deployment中容器镜像的更新。

首先我们需要解析控制台必要的配置，这里使用了golang内置的flag框架。

*kubepractice/operate/update.go*

```go
func main() {
    // 定义参数
	namespace := flag.String("namespace", "", "namespace of deployment")
	deploymentName := flag.String("deployment", "", "deployment name")
	imageName := flag.String("image", "", "new image name")
	appName := flag.String("app", "app", "application name")
	// 解析参数
	flag.Parse()
	if *namespace == "" {
		log.Println("you must specify the namespace of the deployment")
		os.Exit(0)
	}
	if *deploymentName == "" {
		log.Println("You must specify the deployment name.")
		os.Exit(0)
	}
	if *appName == "" {
		log.Println("You must specify the application name.")
		os.Exit(0)
	}
	if *imageName == "" {
		log.Println("You must specify the new image name.")
		os.Exit(0)
	}
    .....
}
```

上面的代码主要是获取控制台的程序运行参数，用户需要指定namespace，deployment名字，image名字，app(container)名字。

随后便是根据信息来获取资源，更新资源等操作，代码如下:

kubepractice/operate/update.go

```go
func main() {
    .......
	// 获取客户端连接stub
	clientset := uitl.GetClient()
	// 首先查看deployment是否存在
	deployment, err := clientset.AppsV1beta1().Deployments(*namespace).Get(*deploymentName, v1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Printf("Deployment not found\n")
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		log.Printf("Error getting deployment%v\n", statusError.ErrStatus.Message)
	} else if err != nil {
		log.Fatal(err.Error())
	} else {
		// 得到deployment
		log.Printf("get deployment:%s\n", *deploymentName)
		// 得到之后，更新deployment信息然后更新
		log.Printf("name:%s\n", deployment.GetName())
		// 得到容器信息
		containners := deployment.Spec.Template.Spec.Containers
		// 找到指定的镜像容器名
		found := false
		for i := range containners {
			if containners[i].Name == *appName {
				// 找到容器
				found = true
				log.Printf("app:%s's old image:%s", *appName, containners[i].Image)
				log.Printf("app:%s's new image:%s", *appName, *imageName)
				// 进行更新
				containners[i].Image = *imageName
			}
		}
		if ! found {
			log.Fatalf("can't found app named:%s", *appName)
		}
		_, err = clientset.AppsV1beta1().Deployments(*namespace).Update(deployment)
		if err != nil {
			log.Fatal(err.Error())
		}
	}
}
```

以上过程很简单，代码即文档。对于其他资源的操作与上面都类似。下面再来看一下第二种交互方式。

## 基于Watch机制对集群操作

Watch机制是API Server提供的一个很重要的机制，我们可以通过编写代码来监听资源的增删改查事件、并且为之注册事件定制的handler回调来实现钩子方法，在资源的生命周期状态中切入额外的定制逻辑。Watch机制的实现需要借助Informer，这是一个**消息通知器**，它会监听对应资源的状态变化，调用我们为之定义好的回调handler。

这里首先还是需要一个客户端来连接到指定的server，代码同上`GetClient()`。

随后我们需要通过client来获取到一个SharedInformerFactory,这里体现了工厂方法的设计模式，利用这个factory，我们可以基于它来得到自己需要的资源Informmer。

```go
client := uitl.GetClient()
// 创建Informer工厂，这里是工厂方法模式
informerFactory := informers.NewSharedInformerFactory(client, time.Second*10)
```

下面的例子是简单的监听**Pod**资源的变化，因此需要利用Informer工厂来创建一个PodInformer,以下展示了如何去设计基于Watch机制的代码骨架：

*kubepractice/watch/eventhandler.go*

```go
var stop = make(<-chan struct{})

// 创建服务队kubernetes特定资源（这里是pod）进行watch
func main() {
	// 首先需要创建客户端连接
	client := uitl.GetClient()
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
```

## 集成测试

现在我们将两个例子联系起来进行测试，我这里默认集群中已经存在了一个deployment,它会部署两个`go-test `pod,它是一个简单的go-web helloworld项目，这里贴出对应的yaml:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: go-test
spec:
  replicas: 2 #创建规模为两个pod
  selector:
    matchLabels:
      app: go-test
  template:
    metadata:
      labels:
        app: go-test
    spec:
      containers:
        - name: go-test
          image: pualrdwade/go-test:1.1 #首先使用1.1版本，便于后续升级测试
          ports:
            - containerPort: 8080
```



下面我们首先启动第二部分的watcher监听器:

```shell
cd $GOPATH/src/kubepractice/watch
go run eventhandler.go
```

启动之后会看到日志信息：
```
2019/06/04 17:48:30 pod added:kube-addon-manager-minikube
2019/06/04 17:48:30 pod added:etcd-minikube
2019/06/04 17:48:30 pod added:kube-controller-manager-minikube
2019/06/04 17:48:30 pod added:kube-scheduler-minikube
2019/06/04 17:48:30 pod added:kubernetes-dashboard-79dd6bfc48-6fvc2
2019/06/04 17:48:30 pod added:coredns-fb8b8dccf-hbg6t
2019/06/04 17:48:30 pod added:coredns-fb8b8dccf-kqtx8
2019/06/04 17:48:30 pod added:kube-apiserver-minikube
2019/06/04 17:48:30 pod added:go-test-698448947d-dgpsc
2019/06/04 17:48:30 pod added:go-test-698448947d-szb4b
2019/06/04 17:48:30 pod added:kube-proxy-78d2q
2019/06/04 17:48:30 pod added:storage-provisioner
.....
```

这里监听到了pod创建，其实是初始化过程。

随后启动update:

```shell
cd $GOPATH/src/kubepractice/operate
go run update.go -namespace default -deployment go-test -app go-test -image pualrdwade/go-test:1.3
```

注意，上面参数传入的image变为了1.3，之前是1.1,这里测试第一部分的更新功能：

启动后看到日志信息：

```
2019/06/04 17:54:50 get deployment:go-test
2019/06/04 17:54:50 name:go-test
2019/06/04 17:54:50 app:go-test's old image:pualrdwade/go-test:1.1
2019/06/04 17:54:50 app:go-test's new image:pualrdwade/go-test:1.3
```

使用kubernetes查看pod，发现更新成功。**重点来了，我们再看看启动的watch**

日志：

```
2019/06/04 17:56:53 pod update:
old pod:go-test-698448947d-tfzfg
new pod:go-test-698448947d-tfzfg
2019/06/04 17:56:56 pod update:
old pod:go-test-67f885bc88-tlr49
new pod:go-test-67f885bc88-tlr49
```

可以看到update更新了deployment之后，deployment改变pod状态，从而触发了监听的事件。