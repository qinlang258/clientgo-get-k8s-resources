package main

import (
	"clientgo-get-k8s-resources/prometheusplugin"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	prov1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/xuri/excelize/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

/*
1 获取某一容器在 当前主机使用的 container_cpu_usage_seconds_total开销

2 使用当前容器 / 在当前主机的容器的CPU开销 = 当前容器使用的CPU开销

// TODO 用这个比例来分摊 没有充分使用的费用

*/

type PodInfo struct {
	PodName        string
	ContainerName  string
	LimitCpu       string
	LimitMemory    string
	RequestsCpu    string
	RequestsMemory string
	TotalCpu       string
	TotalMemory    string
	ShareCpu       float64
	ShareMemory    float64
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

const (
	podMemoryUsageTemplate          = "sum(container_memory_working_set_bytes{pod=\"%s\",namespace=\"%s\",container=\"%s\"})"
	podCpuUsageTemplate             = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",container=\"%s\"})"
	ContainerUseCpuTemplate         = "sum(container_cpu_usage_seconds_total)"
	ComputeShareCpuPodUsageTemplate = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"

	ComputeShareMemoryPodUsageTemplate = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"
)

func initClientGo() (*kubernetes.Clientset, error) {
	client, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Error(context.Background(), err.Error())
	}

	return kubernetes.NewForConfig(client)
}

func formatData(result model.Value, warnings prov1.Warnings, err error) string {
	var num_data string

	if result.String() == "" {
		return "0"
	}

	data := result.String()

	//提取 => 0.0031342189920170885 @[1711701880.602
	s1 := strings.Split(data, "=>")
	s2 := strings.Split(s1[1], "@")
	num_data = strings.ReplaceAll(s2[0], " ", "")

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

	prometheus_client := prometheusplugin.NewProme("http://prometheus.test.newhopescm.com:80", 10)

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
			podinfo.NodeName = podinfo.Node.NodeName
			podinfo.ContainerName = pod.Spec.Containers[0].Name

			memorySql := fmt.Sprintf(podMemoryUsageTemplate, podName, namespace, podinfo.ContainerName)
			shareCpuSql := fmt.Sprintf(ComputeShareCpuPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)
			shareMemorySql := fmt.Sprintf(ComputeShareMemoryPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)

			strmemorySize := formatData(prometheus_client.Client.Query(ctx, memorySql, time.Now()))

			memorySize, err1 := strconv.ParseFloat(strmemorySize, 64)
			if err1 != nil {
				klog.Error(ctx, err1.Error())
			}

			TotalMemory := fmt.Sprintf("%.2f", memorySize/1024/1024)

			strShareCpuSize := formatData(prometheus_client.Client.Query(ctx, shareCpuSql, time.Now()))
			shareCpu, err := strconv.ParseFloat(strShareCpuSize, 64)
			if err != nil {
				// Handle error if conversion fails
				klog.Error(ctx, err.Error())
			}

			strShareMemorySize := formatData(prometheus_client.Client.Query(ctx, shareMemorySql, time.Now()))
			shareMemory, err := strconv.ParseFloat(strShareMemorySize, 64)
			if err != nil {
				klog.Error(ctx, err.Error())
			}

			float64ShareMemory, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", shareMemory), 64)
			float64ShareCpu, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", shareCpu), 64)

			podinfo.TotalMemory = TotalMemory + "Mi"
			//podinfo.TotalCpu = strShareCpuSize
			podinfo.ShareMemory = float64ShareMemory
			podinfo.ShareCpu = float64ShareCpu
			podInfoList = append(podInfoList, *podinfo)
			fmt.Println(podinfo)
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
	file.SetCellValue(sheetName, "G1", "实际使用CPU在各自服务器的百分比")
	file.SetCellValue(sheetName, "H1", "实际使用内存在各自服务器的数值")
	file.SetCellValue(sheetName, "I1", "分摊服务器的空闲的CPU之后的占比")
	file.SetCellValue(sheetName, "J1", "分摊服务器的空闲的内存之后的占比")

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
		file.SetCellValue(sheetName, "I"+strconv.Itoa(row), app.ShareCpu)
		file.SetCellValue(sheetName, "J"+strconv.Itoa(row), app.ShareMemory)

	}
	if err := file.SaveAs("pod的资源使用情况.xlsx"); err != nil {
		panic(err)
	}

}
