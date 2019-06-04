package main

import (
	"flag"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	util "kubepractice/util"
	"log"
	"os"
)

func main() {
	namespace := flag.String("namespace", "", "namespace of deployment")
	deploymentName := flag.String("deployment", "", "deployment name")
	imageName := flag.String("image", "", "new image name")
	appName := flag.String("app", "app", "application name")

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

	//利用client-go来创建自己的资源
	client := util.GetClient()
	// 首先查看deployment是否存在
	deployment, err := client.AppsV1beta1().Deployments(*namespace).Get(*deploymentName, v1.GetOptions{})
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
		containers := deployment.Spec.Template.Spec.Containers
		// 找到指定的镜像容器名
		found := false
		for i := range containers {
			if containers[i].Name == *appName {
				// 找到容器
				found = true
				log.Printf("app:%s's old image:%s", *appName, containers[i].Image)
				log.Printf("app:%s's new image:%s", *appName, *imageName)
				// 进行更新
				containers[i].Image = *imageName
			}
		}
		if ! found {
			log.Fatalf("can't found app named:%s", *appName)
		}
		_, err = client.AppsV1beta1().Deployments(*namespace).Update(deployment)
		if err != nil {
			log.Fatal(err.Error())
		}
	}
}
