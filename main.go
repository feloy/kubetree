package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	objtype := os.Args[1]
	objname := os.Args[2]

	level := 0
	GetDependents(clientset, objtype, objname, &level)
	GetOwner(clientset, objtype, objname, level)
}

func GetDependents(clientset *kubernetes.Clientset, objtype string, objname string, level *int) {
	pods, err := clientset.CoreV1().Pods("default").List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	found := false
	for _, pod := range pods.Items {
		for _, ownerRef := range pod.ObjectMeta.GetOwnerReferences() {
			if ownerRef.Name == objname {
				GetDependents(clientset, pod.Kind, pod.Name, level)
				fmt.Printf("%s- %s/%s\n", strings.Repeat(" ", 2**level), "Pod", pod.Name)
				found = true
			}
		}
	}
	if found {
		*level++
	}

	found = false
	rss, err := clientset.AppsV1().ReplicaSets("default").List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, rs := range rss.Items {
		for _, ownerRef := range rs.ObjectMeta.GetOwnerReferences() {
			if ownerRef.Name == objname {
				GetDependents(clientset, rs.Kind, rs.Name, level)
				fmt.Printf("%s- %s/%s\n", strings.Repeat(" ", 2**level), "ReplicaSet", rs.Name)
				found = true
			}
		}
	}
	if found {
		*level++
	}
}

func GetOwner(clientset *kubernetes.Clientset, objtype string, objname string, level int) {
	var owners []metav1.OwnerReference
	var found bool
	switch objtype {
	case "Pod":
		pods, err := clientset.CoreV1().Pods("default").List(metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", objname),
		})
		if err != nil {
			panic(err)
		}
		if len(pods.Items) > 0 {
			pod := pods.Items[0]
			found = true
			fmt.Printf("%s- %s/%s\n", strings.Repeat(" ", 2*level), objtype, objname)
			owners = pod.ObjectMeta.GetOwnerReferences()
		}
	case "ReplicaSet":
		rss, err := clientset.AppsV1().ReplicaSets("default").List(metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", objname),
		})
		if err != nil {
			panic(err)
		}
		if len(rss.Items) > 0 {
			rs := rss.Items[0]
			found = true
			owners = rs.ObjectMeta.GetOwnerReferences()
		}
	case "Deployment":
		deployments, err := clientset.AppsV1().Deployments("default").List(metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", objname),
		})
		if err != nil {
			panic(err)
		}
		if len(deployments.Items) > 0 {
			deployment := deployments.Items[0]
			found = true
			owners = deployment.ObjectMeta.GetOwnerReferences()
		}
	default:
		fmt.Printf("*** ERROR ***: Unknown type %s\n", objtype)
	}

	if found {
		fmt.Printf("%s- %s/%s\n", strings.Repeat(" ", 2*level), objtype, objname)

		for _, owner := range owners {
			GetOwner(clientset, owner.Kind, owner.Name, level+1)
		}
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
