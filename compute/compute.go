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
	v1 "k8s.io/api/core/v1"
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
	Namespace      string
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
	//查询过去7天的平均值
	podMemoryUsageTemplate = "sum(avg_over_time(container_memory_working_set_bytes{pod=\"%s\",namespace=\"%s\",container=\"%s\",node=\"%s\"}[7d]))"
	podCpuUsageTemplate    = "sum(avg_over_time(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",container=\"%s\",node=\"%s\"}[7d]))"

	// ComputeShareCpuPodUsageTemplate    = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"
	// ComputeShareMemoryPodUsageTemplate = "sum(container_cpu_usage_seconds_total{pod=\"%s\",namespace=\"%s\",node=\"%s\"}) / sum(container_cpu_usage_seconds_total{pod!='',node=\"%s\"})"
)

func NewCompute(ctx context.Context) *Compute {
	return &Compute{}
}

// 如果不填写kubeconfigPath的话就取默认的 ~/.kube/config
func (c *Compute) InitClientGo(kubeconfigPath string) (*kubernetes.Clientset, error) {
	if kubeconfigPath == "" {
		client, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			klog.Error(context.Background(), err.Error())
		}
		return kubernetes.NewForConfig(client)
	} else {
		client, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			klog.Error(context.Background(), err.Error())
		}
		return kubernetes.NewForConfig(client)
	}

}

func (c *Compute) FormatData(result model.Value, warnings prov1.Warnings, err error) string {
	var num_data string

	if err != nil {
		fmt.Println("prometheus没有获取到数据,请检查Prometheus是否能正常访问?")
		return ""
	}

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

func (c *Compute) InsertData(ctx context.Context, prometheusUrl, kubeconfigPath string) bool {
	nodeNameMap := make(map[string]bool)

	client, err := c.InitClientGo(kubeconfigPath)
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

	prometheus_client := prometheusplugin.NewProme(prometheusUrl, 10)
	//fmt.Println(prometheus_client.Client.Buildinfo(ctx))

	for _, values := range namespacesItem.Items {
		namespaces = append(namespaces, values.ObjectMeta.Name)
	}

	for _, namespace := range namespaces {
		podList, _ := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		for _, pod := range podList.Items {
			if pod.Status.Phase == v1.PodPhase("Running") {
				//node := &NodeInfo{}

				podinfo := &PodInfo{}
				podName := pod.GetName()
				nodeHost := pod.Status.HostIP
				nodeName := nodeMap[nodeHost]

				// 获取所需CPU和内存
				podinfo.PodName = podName
				podinfo.Namespace = namespace
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

				memorySql := fmt.Sprintf(podMemoryUsageTemplate, podName, namespace, podinfo.ContainerName, podinfo.NodeName)
				cpuSql := fmt.Sprintf(podCpuUsageTemplate, podName, namespace, podinfo.ContainerName, podinfo.NodeName)

				//shareCpuSql := fmt.Sprintf(ComputeShareCpuPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)
				//shareMemorySql := fmt.Sprintf(ComputeShareMemoryPodUsageTemplate, podName, namespace, podinfo.NodeName, podinfo.NodeName)

				strmemorySize := c.FormatData(prometheus_client.Client.Query(ctx, memorySql, time.Now()))

				memorySize, err1 := strconv.ParseFloat(strmemorySize, 64)
				if err1 != nil {
					klog.Error(ctx, err1.Error())
					return false
				}

				podinfo.RealMemory = memorySize / 1024 / 1024

				//fmt.Printf("RealMemory 的值为 %f Requests 的值为 %f \n", podinfo.RealMemory, podinfo.RequestsMemory)

				strcpuSize := c.FormatData(prometheus_client.Client.Query(ctx, cpuSql, time.Now()))
				cpuSize, err := strconv.ParseFloat(strcpuSize, 64)
				if err != nil {
					// Handle error if conversion fails
					klog.Error(ctx, err.Error())
					return false
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
			} else {
				fmt.Println(pod.GetName(), "is not running !!!!!!!")
			}

		}
	}

	return true
}

func (c *Compute) ComputeShareSize(ctx context.Context, nodeNameMap map[string]bool) []*PodInfo {
	for nodename, _ := range nodeNameMap {
		var AllRealCpuSize float64
		var AllRealMemorySize float64
		var AllRequestsCpuSize float64
		var AllrequestsMemorySize float64

		// 这里 RealCpu的值是 68323.56000254这样的，但是recordPod.RequestsCpu的值为 0.1 0.2类似的；
		// 需要考虑一个问题，怎么实现 按照 requests 和实际的值比较，当requests大于 实际值，则取requests
		// TODO 引入一个中间值

		//获取这个Node的所有 所需CPU所需内存，实际CPU实际内存
		for _, recordPod := range c.PodInfoList {

			if recordPod.NodeName == nodename {
				AllRealCpuSize += recordPod.RealCpu
				AllRealMemorySize += recordPod.RealMemory
				AllRequestsCpuSize += recordPod.RequestsCpu
				AllrequestsMemorySize += recordPod.RequestsMemory
			}

		}

		for _, pod := range c.PodInfoList {
			percentMemory := ((pod.RealMemory / AllRealMemorySize) + (pod.RequestsMemory / AllrequestsMemorySize)) / 2
			pod.ShareMemory, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", percentMemory), 64)

			percentCpu := ((pod.RealCpu / AllRealCpuSize) + (pod.RequestsCpu / AllRequestsCpuSize)) / 2
			pod.ShareCpu, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", percentCpu), 64)
			// ShareMemory := pod.CompareMemory / AllMemorySize
			// ShareCpu := pod.CompareCpu / AllCpuSize
			// pod.ShareMemory, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", ShareMemory), 64)
			// pod.ShareCpu, _ = strconv.ParseFloat(fmt.Sprintf("%.4f", ShareCpu), 64)

			//fmt.Printf("node %s pod-name %s CPU大小 %f 内存大小 %f \n", nodename, pod.PodName, pod.ShareCpu, pod.ShareMemory)
		}

	}

	return c.PodInfoList

}
