package main

import (
	"context"
	"fmt"
	"get-resource/get-resource/prometheus"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const (
	podMemoryUsageTemplate = "container_memory_working_set_bytes{container=\"%s\"}"
	podCpuUsageTemplate    = "container_cpu_usage_seconds_total{container=\"%s\"}"
	podName                = "calico-kube-controllers"
)

func initClientGo() (*kubernetes.Clientset, error) {
	client, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Error(context.Background(), err.Error())
	}

	return kubernetes.NewForConfig(client)
}

func main() {
	ctx := context.Background()
	// client, err := initClientGo()
	// if err != nil {
	// 	klog.Error(ctx, err.Error())
	// }

	memorySql := fmt.Sprintf(podMemoryUsageTemplate, podName)
	//cpuSql := fmt.Sprintf(podCpuUsageTemplate, podName)

	prometheus_client := prometheus.NewProme("http://192.168.44.133:31666", 10)

	memoryMessage, _, _ := prometheus_client.Client.Query(ctx, memorySql, time.Now())
	fmt.Println(memoryMessage.String())

}
