package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/api/batch/v1"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"os"
	"path/filepath"
)

type K8Client struct {
	cs *kubernetes.Clientset
	ns string
}

type ConfigMapFile struct {
	Name string
	Data []byte
}

type K8ClientI interface {
	StartJob(cs *kubernetes.Clientset, job []byte) (*v1.Job, error)
	PutScriptAsConfigMap(cs *kubernetes.Clientset, pyScriptHash string, pyScript []byte) (*v12.ConfigMap, error)
}

func ConnectToK8() *K8Client {
	var kubeconfig *string
	var config *rest.Config

	if c, err := rest.InClusterConfig(); err != nil {
		fmt.Println(fmt.Errorf("no internal-cluster config found: %s", err.Error()))
		if home := homeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		c, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			fmt.Println(fmt.Errorf("no external-cluster config found: %s", err.Error()))
			panic(fmt.Errorf("no cluster config found"))
		}
		config = c
	} else {
		config = c
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return &K8Client{cs: clientset, ns: "jhub"}
}

func (cs *K8Client) StartJob(job []byte) (*v1.Job, error) {
	k8Job := v1.Job{}
	jobReader := bytes.NewReader(job)
	if err := yaml.NewYAMLOrJSONDecoder(jobReader, 4096).Decode(&k8Job); err != nil {
		return nil, errors.WithStack(err)
	}
	if j, err := cs.cs.BatchV1().Jobs(cs.ns).Create(&k8Job); err != nil {
		return nil, errors.WithStack(err)
	} else {
		return j, nil
	}
}

func (cs *K8Client) PutConfigMap(name string, files []ConfigMapFile) (*v12.ConfigMap, error) {
	k8ConfigMap := v12.ConfigMap{}
	k8ConfigMap.Name = name
	k8ConfigMap.BinaryData = make(map[string][]byte)
	for _, f := range files {
		k8ConfigMap.BinaryData[f.Name] = f.Data
	}

	_, err := cs.cs.CoreV1().ConfigMaps(cs.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return cs.cs.CoreV1().ConfigMaps(cs.ns).Create(&k8ConfigMap)
	} else {
		return cs.updateConfigMap(name, &k8ConfigMap)
	}
}

func (cs *K8Client) updateConfigMap(name string, configMap *v12.ConfigMap) (*v12.ConfigMap, error) {
	var cm *v12.ConfigMap
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var updateErr error
		cm, updateErr = cs.cs.CoreV1().ConfigMaps(cs.ns).Update(configMap)
		return errors.WithStack(updateErr)
	})
	if retryErr != nil {
		return nil, errors.WithStack(retryErr)
	}
	return cm, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
