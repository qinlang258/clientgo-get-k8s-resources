package excel

import (
	"clientgo-get-k8s-resources/compute"
	"context"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func ExportXlsx(ctx context.Context, PodInfoList []*compute.PodInfo) {
	file := excelize.NewFile()
	sheetName := "Sheet1"
	// 设置表头
	file.SetCellValue(sheetName, "A1", "节点名")
	file.SetCellValue(sheetName, "B1", "实例类型")
	file.SetCellValue(sheetName, "C1", "命名空间")
	file.SetCellValue(sheetName, "D1", "服务名")
	file.SetCellValue(sheetName, "E1", "所需CPU")
	file.SetCellValue(sheetName, "F1", "所需内存")
	file.SetCellValue(sheetName, "G1", "限制CPU")
	file.SetCellValue(sheetName, "H1", "限制内存")
	file.SetCellValue(sheetName, "I1", "实际使用CPU在各自服务器的数值")
	file.SetCellValue(sheetName, "J1", "实际使用内存在各自服务器的数值")
	file.SetCellValue(sheetName, "K1", "服务器费用")
	file.SetCellValue(sheetName, "L1", "分摊服务器的空闲的CPU之后的占比")
	file.SetCellValue(sheetName, "M1", "分摊服务器的空闲的内存之后的占比")
	file.SetCellValue(sheetName, "N1", "分摊之后的预估费用")

	for i, app := range PodInfoList {
		row := i + 2
		file.SetCellValue(sheetName, "A"+strconv.Itoa(row), app.NodeName)
		file.SetCellValue(sheetName, "B"+strconv.Itoa(row), app.Node.NodeType)
		file.SetCellValue(sheetName, "C"+strconv.Itoa(row), app.Namespace)
		file.SetCellValue(sheetName, "D"+strconv.Itoa(row), app.PodName)
		file.SetCellValue(sheetName, "E"+strconv.Itoa(row), app.RequestsCpu)
		file.SetCellValue(sheetName, "F"+strconv.Itoa(row), app.RequestsMemory)
		file.SetCellValue(sheetName, "G"+strconv.Itoa(row), app.LimitCpu)
		file.SetCellValue(sheetName, "H"+strconv.Itoa(row), app.LimitMemory)
		file.SetCellValue(sheetName, "I"+strconv.Itoa(row), app.RealCpu)
		file.SetCellValue(sheetName, "J"+strconv.Itoa(row), app.RealMemory)
		file.SetCellValue(sheetName, "L"+strconv.Itoa(row), app.ShareCpu)
		file.SetCellValue(sheetName, "M"+strconv.Itoa(row), app.ShareMemory)

	}
	if err := file.SaveAs("pod的资源使用情况.xlsx"); err != nil {
		panic(err)
	}
}
