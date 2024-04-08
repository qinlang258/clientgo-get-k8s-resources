package main

import (
	"clientgo-get-k8s-resources/compute"
	"clientgo-get-k8s-resources/excel"
	"context"
	"flag"
)

func getCommands() {
	flag.StringVar(&prometheusUrl, "prometheusUrl", "", "请输入prometheus的地址")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", "", "请输入服务器的kubeconfig文件地址,如果不输入的话就自动获取/root/.kube/config的配置文件")
	flag.Parse()
}

var (
	prometheusUrl  string
	kubeconfigPath string
)

func main() {
	getCommands()
	ctx := context.Background()
	c := compute.NewCompute(ctx)

	flag.Parse()
	c.InsertData(ctx, prometheusUrl, kubeconfigPath)
	//data := c.ComputeShareSize(ctx, c.NodeNameMap)
	excel.ExportXlsx(ctx, c.PodInfoList)
}
