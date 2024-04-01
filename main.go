package main

import (
	"clientgo-get-k8s-resources/prometheus"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/xuri/excelize/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type PodInfo struct {
	PodName        string
	LimitCpu       string
	LimitMemory    string
	RequestsCpu    string
	RequestsMemory string
	TotalCpu       string
	TotalMemory    string
	NodeName       string   //这里记录是哪一个Node
	Node           NodeInfo //这里是记录Node的详细信息，这里的重复NodeName不能删除
}

type NodeInfo struct {
	NodeName string
	NodeIp   string
	NodeSize
}

type NodeSize struct {
	NodeMemory  string
	NodeCpu     string
	UsedCpu     string
	UsedMemory  string
	UnuseCpu    string
	UnuseMemory string
}

//TODO  kubectl top po获取到的内存和CPU在 Prometheus的2个字段		container_cpu_usage_seconds_total 	container_memory_usage_bytes

// container_memory_working_set_bytes{namespace="kube-system", pod="nginx-deployment-6697d74c5f-82g6v", container="nginx"} 需要提供 container与 pod

//TODO 获取的容器运行的CPU，算费用的时候，是否应该让容器都平摊这个服务器没有使用充分的资源

const (
	podMemoryUsageTemplate       = "container_memory_working_set_bytes{pod=\"%s\",namespace=\"%s\"}"
	podCpuUsageTemplate          = "container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\"}"
	ContainerUseCpuTemplate      = "sum(container_cpu_usage_seconds_total)"
	ContainerUnUseCpuTemplate    = "sum(container_cpu_usage_seconds_total{container=''}) "
	ContainerUseMemoryTemplate   = "sum(container_memory_working_set_bytes{container!=''})"
	ContainerUnUseMemoryTemplate = "sum(container_memory_working_set_bytes{container=''})"
)

func initClientGo() (*kubernetes.Clientset, error) {
	client, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Error(context.Background(), err.Error())
	}

	return kubernetes.NewForConfig(client)
}

func formatData(result model.Value, message []string, err error) string {

	data := result.String()

	//提取 => 0.0031342189920170885 @[1711701880.602
	s1 := strings.Split(data, "=>")
	s2 := strings.Split(s1[1], "@")
	num_data := strings.ReplaceAll(s2[0], " ", "")

	return num_data
}

func getNodeInfo(client *kubernetes.Clientset, ctx context.Context, name string) *NodeInfo {
	nodeMessage, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		klog.Error(ctx, err.Error())
	}

	nodeInfo := &NodeInfo{}
	nodeInfo.NodeName = nodeMessage.GetName()
	nodeInfo.NodeIp = nodeMessage.Status.Addresses[0].Address
	nodeInfo.NodeCpu = nodeMessage.Status.Capacity.Cpu().String()
	nodeInfo.NodeMemory = nodeMessage.Status.Capacity.Memory().String()

	return nodeInfo

}

func main() {

	var podInfoList []PodInfo
	ctx := context.Background()

	client, err := initClientGo()
	if err != nil {
		klog.Error(ctx, err.Error())
	}

	//restClient := client.CoreV1().RESTClient()

	namespacesItem, _ := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})

	nodesItem, _ := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	nodeMap := make(map[string]string, 0)

	Node := &NodeInfo{}

	for _, node := range nodesItem.Items {
		nodeMap[node.Status.Addresses[0].Address] = node.GetName()

		node.Status.Capacity.Memory()
		Node.NodeName = node.GetName()
		Node.NodeIp = node.Status.Addresses[0].Address
		Node.NodeCpu = node.Status.Capacity.Cpu().String()
		Node.NodeMemory = node.Status.Capacity.Memory().String()

	}

	// 获取所有的Node信息  fmt.Println(Node)

	var namespaces []string

	prometheus_client := prometheus.NewProme("http://192.168.44.133:31666", 10)

	for _, values := range namespacesItem.Items {
		namespaces = append(namespaces, values.ObjectMeta.Name)
	}

	for _, namespace := range namespaces {
		podList, _ := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		for _, pod := range podList.Items {
			//node := &NodeInfo{}

			podinfo := &PodInfo{}
			podName := pod.GetName()
			nodeHost := pod.Status.HostIP
			nodeName := nodeMap[nodeHost]
			// node.NodeIp = nodeHost
			// node.NodeName = nodeName

			podinfo.PodName = podName
			podinfo.RequestsCpu = pod.Spec.Containers[0].Resources.Requests.Cpu().String()
			podinfo.RequestsMemory = pod.Spec.Containers[0].Resources.Requests.Memory().String()
			podinfo.LimitCpu = pod.Spec.Containers[0].Resources.Limits.Cpu().String()
			podinfo.LimitMemory = pod.Spec.Containers[0].Resources.Limits.Memory().String()
			podinfo.Node = *getNodeInfo(client, ctx, nodeName)

			//TODO 有的服务没有这个数值
			memorySql := fmt.Sprintf(podMemoryUsageTemplate, podName, namespace)
			cpuSql := fmt.Sprintf(podCpuUsageTemplate, podName, namespace)
			strmemorySize := formatData(prometheus_client.Client.Query(ctx, memorySql, time.Now()))

			memorySize, err1 := strconv.ParseFloat(strmemorySize, 64)
			if err1 != nil {
				klog.Error(ctx, err1.Error())
			}

			TotalMemory := fmt.Sprintf("%.2f", memorySize/1024/1024)

			strcpuSize := formatData(prometheus_client.Client.Query(ctx, cpuSql, time.Now()))
			cpuSize, err2 := strconv.ParseFloat(strcpuSize, 64)
			if err2 != nil {
				klog.Error(ctx, err.Error())
			}

			TotalCpu := fmt.Sprintf("%.2f", cpuSize)

			podinfo.TotalMemory = TotalMemory + "Mi"
			podinfo.TotalCpu = TotalCpu + "m"

			podInfoList = append(podInfoList, *podinfo)
		}
	}

	file := excelize.NewFile()
	sheetName := "Sheet1"
	// 设置表头
	file.SetCellValue(sheetName, "A1", "节点名")
	file.SetCellValue(sheetName, "B1", "服务名")
	file.SetCellValue(sheetName, "C1", "所需CPU")
	file.SetCellValue(sheetName, "D1", "所需内存")
	file.SetCellValue(sheetName, "E1", "限制CPU")
	file.SetCellValue(sheetName, "F1", "限制内存")
	file.SetCellValue(sheetName, "G1", "实际使用CPU")
	file.SetCellValue(sheetName, "H1", "实际使用内存")

	for i, app := range podInfoList {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), app.NodeName)
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), app.PodName)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), app.RequestsCpu)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), app.RequestsMemory)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), app.LimitCpu)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(row), app.LimitMemory)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(row), app.TotalCpu)
		file.SetCellValue(sheetName, "H"+strconv.Itoa(row), app.TotalMemory)

	}
	if err := file.SaveAs("pod的资源使用情况.xlsx"); err != nil {
		panic(err)
	}

}
