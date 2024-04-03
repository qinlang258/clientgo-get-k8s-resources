package main

import (
	"clientgo-get-k8s-resources/compute"
	"clientgo-get-k8s-resources/excel"
	"context"
)

func main() {
	ctx := context.Background()
	c := compute.NewCompute(ctx)

	c.InsertData(ctx)
	c.ComputeShareSize(ctx, c.NodeNameMap)

	excel.ExportXlsx(ctx, c.PodInfoList)
}
