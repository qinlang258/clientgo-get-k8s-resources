package compute

import (
	"clientgo-get-k8s-resources/prometheusplugin"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	prov1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

/*
1 获取某一容器在 当前主机使用的 container_cpu_usage_seconds_total开销

2 使用当前容器 / 在当前主机的容器的CPU开销 = 当前容器使用的CPU开销

// TODO 用这个比例来分摊 没有充分使用的费用

如果声明的资源大于实际的使用，那就取 resources.requests

*/

type PodInfo struct {
	PodName        string
	ContainerName  string
	LimitCpu       float64
	LimitMemory    float64
	RequestsCpu    float64
	RequestsMemory float64
	RealCpu        float64
	RealMemory     float64
	ShareCpu       float64
	ShareMemory    float64
	CompareCpu     float64
	CompareMemory  float64
	NodeName       string    //这里记录是哪一个Node
	Node           *NodeInfo //这里是记录Node的详细信息，这里的重复NodeName不能删除
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
	NodeType    string
}

type Compute struct {
	PodInfoList []*PodInfo
	NodeNameMap map[string]bool
}

func (c *Compute) conversionMemoryStrToFloat64(memorySize string) float64 {
	if memorySize == "" {
		return 0
	}

	re := regexp.MustCompile(`\d+`)

	var data float64

	number := re.FindString(memorySize)
	if strings.HasSuffix(memorySize, "Gi") {
		float64Data, _ := strconv.ParseFloat(number, 64)
		data = float64Data * 1024
	} else if strings.HasSuffix(memorySize, "Mi") {
		float64Data, _ := strconv.ParseFloat(number, 64)
		data = float64Data
	}

	return data
}

//TODO  kubectl top po获取到的内存和CPU在 Prometheus的2个字段		container_cpu_usage_seconds_total 	container_memory_usage_bytes

// container_memory_working_set_bytes{namespace="kube-system", pod="nginx-deployment-6697d74c5f-82g6v", container="nginx"} 需要提供 container与 pod

const (
	podMemoryUsageTemplate  = "sum(container_memory_working_set_bytes{pod=\"%s\",namespace=\"%s\",container=\"%s\"})"
	podCpuUsageTemplate     = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",container=\"%s\"})"
	ContainerUseCpuTemplate = "sum(container_cpu_usage_seconds_total)"

	ComputeShareCpuPodUsageTemplate    = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"
	ComputeShareMemoryPodUsageTemplate = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"
)

func NewCompute(ctx context.Context) *Compute {
	return &Compute{}
}

func (c *Compute) InitClientGo() (*kubernetes.Clientset, error) {
	client, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Error(context.Background(), err.Error())
	}

	return kubernetes.NewForConfig(client)
}

func (c *Compute) FormatData(result model.Value, warnings prov1.Warnings, err error) string {
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

func (c *Compute) GetNodeInfo(client *kubernetes.Clientset, ctx context.Context, name string) *NodeInfo {
	nodeMessage, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		klog.Error(ctx, err.Error())
	}

	nodeInfo := &NodeInfo{}
	nodeInfo.NodeName = nodeMessage.GetName()
	nodeInfo.NodeIp = nodeMessage.Status.Addresses[0].Address
	nodeInfo.NodeCpu = nodeMessage.Status.Capacity.Cpu().String()
	nodeInfo.NodeMemory = nodeMessage.Status.Capacity.Memory().String()
	nodeInfo.NodeType = nodeMessage.Labels["beta.kubernetes.io/instance-type"]

	return nodeInfo

}

func (c *Compute) InsertData(ctx context.Context) {
	nodeNameMap := make(map[string]bool)

	client, err := c.InitClientGo()
	if err != nil {
		klog.Error(ctx, err.Error())
	}

	namespacesItem, _ := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})

	nodesItem, _ := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	nodeMap := make(map[string]string, 0)

	for _, node := range nodesItem.Items {
		nodeMap[node.Status.Addresses[0].Address] = node.GetName()
		nodeNameMap[node.GetName()] = true
	}

	c.NodeNameMap = nodeNameMap

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

			// 获取所需CPU和内存
			podinfo.PodName = podName
			podinfo.RequestsCpu = pod.Spec.Containers[0].Resources.Requests.Cpu().AsApproximateFloat64()
			requestsMemory := pod.Spec.Containers[0].Resources.Requests.Memory().String()
			podinfo.RequestsMemory = c.conversionMemoryStrToFloat64(requestsMemory)

			//获取限制CPU内存
			podinfo.LimitCpu = pod.Spec.Containers[0].Resources.Limits.Cpu().AsApproximateFloat64()
			limitMemory := pod.Spec.Containers[0].Resources.Limits.Memory().String()
			podinfo.LimitMemory = c.conversionMemoryStrToFloat64(limitMemory)

			podinfo.Node = c.GetNodeInfo(client, ctx, nodeName)
			podinfo.NodeName = podinfo.Node.NodeName

			podinfo.ContainerName = pod.Spec.Containers[0].Name

			memorySql := fmt.Sprintf(podMemoryUsageTemplate, podName, namespace, podinfo.ContainerName)
			cpuSql := fmt.Sprintf(podCpuUsageTemplate, podName, namespace, podinfo.ContainerName)

			//shareCpuSql := fmt.Sprintf(ComputeShareCpuPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)
			//shareMemorySql := fmt.Sprintf(ComputeShareMemoryPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)

			strmemorySize := c.FormatData(prometheus_client.Client.Query(ctx, memorySql, time.Now()))

			memorySize, err1 := strconv.ParseFloat(strmemorySize, 64)
			if err1 != nil {
				klog.Error(ctx, err1.Error())
			}

			podinfo.RealMemory = memorySize / 1024 / 1024

			strcpuSize := c.FormatData(prometheus_client.Client.Query(ctx, cpuSql, time.Now()))
			fmt.Println(strcpuSize)
			cpuSize, err := strconv.ParseFloat(strcpuSize, 64)
			if err != nil {
				// Handle error if conversion fails
				klog.Error(ctx, err.Error())
			}
			podinfo.RealCpu = cpuSize

			// strShareMemorySize := c.FormatData(prometheus_client.Client.Query(ctx, shareMemorySql, time.Now()))
			// shareMemory, err := strconv.ParseFloat(strShareMemorySize, 64)
			// if err != nil {
			// 	klog.Error(ctx, err.Error())
			// }

			// float64ShareMemory, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", shareMemory), 64)
			// float64ShareCpu, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", shareCpu), 64)

			//podinfo.TotalCpu = strShareCpuSize
			// podinfo.ShareMemory = float64ShareMemory
			// podinfo.ShareCpu = float64ShareCpu
			c.PodInfoList = append(c.PodInfoList, podinfo)
			fmt.Println(podinfo)
		}
	}
}

func (c *Compute) ComputeShareSize(ctx context.Context, nodeNameMap map[string]bool) []*PodInfo {
	for nodename, _ := range nodeNameMap {
		var AllCpuSize float64
		var AllMemorySize float64

		for _, recordPod := range c.PodInfoList {
			if recordPod.NodeName == nodename {
				if recordPod.RequestsMemory > recordPod.RealMemory {
					recordPod.CompareMemory = recordPod.RequestsMemory
					fmt.Println(recordPod.CompareMemory, recordPod.RequestsMemory)
					AllMemorySize += recordPod.CompareMemory
				} else {
					recordPod.CompareMemory = recordPod.RealMemory
					AllMemorySize += recordPod.CompareMemory
				}

				if recordPod.RequestsCpu > recordPod.RealCpu {
					recordPod.CompareCpu = recordPod.RequestsCpu
					AllCpuSize += recordPod.CompareCpu
				} else {
					recordPod.CompareCpu = recordPod.RealCpu
					AllCpuSize += recordPod.CompareCpu
				}
			}

		}

		for _, pod := range c.PodInfoList {
			ShareMemory := pod.CompareMemory / AllMemorySize
			ShareCpu := pod.CompareCpu / AllCpuSize
			pod.ShareMemory, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", ShareMemory), 64)
			pod.ShareCpu, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", ShareCpu), 64)

			fmt.Printf("node %s pod-name %s CPU大小 %f 内存大小 %f \n", nodename, pod.PodName, pod.ShareCpu, pod.ShareMemory)
		}
	}

	return c.PodInfoList

}
