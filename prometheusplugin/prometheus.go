package prometheusplugin

import (
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/klog"

	"context"
	"fmt"
	"time"
)

const (
	// nodeMeasureQueryTemplate is the template string to get the query for the node used bandwidth
	// nodeMeasureQueryTemplate = "sum_over_time(node_network_receive_bytes_total{device=\"%s\"}[%ss])"
	nodeMeasureQueryTemplate = "sum_over_time(node_network_receive_bytes_total{device=\"%s\"}[%ss]) * on(instance) group_left(nodename) (node_uname_info{nodename=\"%s\"})"
	podMemoryUsageTemplate   = "container_memory_working_set_bytes{container=\"%s\"}"
	podCpuUsageTemplate      = "container_cpu_usage_seconds_total{container=\"%s\"}"
)

type PrometheusHandle struct {
	timeRange time.Duration
	ip        string
	Client    v1.API
}

func NewProme(ip string, timeRace time.Duration) *PrometheusHandle {
	client, err := api.NewClient(api.Config{Address: ip})
	if err != nil {
		klog.Fatalf("[NetworkTraffic Plugin] FatalError creating prometheus client: %s", err.Error())
	}
	return &PrometheusHandle{
		ip:        ip,
		timeRange: timeRace,
		Client:    v1.NewAPI(client),
	}
}

func (p *PrometheusHandle) GetCpuUsage(container string) (*model.Sample, error) {
	value, err := p.query(fmt.Sprintf(podCpuUsageTemplate, container))
	//fmt.Println(fmt.Sprintf(podCpuUsageTemplate, container))
	if err != nil {
		return nil, fmt.Errorf("[NetworkTraffic Plugin] Error querying prometheus: %w", err)
	}

	cpuMeasure := value.(model.Vector)
	if len(cpuMeasure) != 1 {
		return nil, fmt.Errorf("[NetworkTraffic Plugin] Invalid response, expected 1 value, got %d", len(cpuMeasure))
	}
	return cpuMeasure[0], err
}

func (p *PrometheusHandle) GetMemoryUsage(container string) (*model.Sample, error) {
	value, err := p.query(fmt.Sprintf(podMemoryUsageTemplate, container))
	//fmt.Println(fmt.Sprintf(podMemoryUsageTemplate, container))
	if err != nil {
		return nil, fmt.Errorf("[NetworkTraffic Plugin] Error querying prometheus: %w", err)
	}

	memoryMeasure := value.(model.Vector)
	if len(memoryMeasure) != 1 {
		return nil, fmt.Errorf("[NetworkTraffic Plugin] Invalid response, expected 1 value, got %d", len(memoryMeasure))
	}
	return memoryMeasure[0], err
}

// func (p *PrometheusHandle) GetGauge(node string) (*model.Sample, error) {

// 	value, err := p.query(fmt.Sprintf(nodeMeasureQueryTemplate, node, p.deviceName, p.timeRange))
// 	fmt.Println(fmt.Sprintf(nodeMeasureQueryTemplate, p.deviceName, p.timeRange, node))
// 	if err != nil {
// 		return nil, fmt.Errorf("[NetworkTraffic Plugin] Error querying prometheus: %w", err)
// 	}

// 	nodeMeasure := value.(model.Vector)
// 	if len(nodeMeasure) != 1 {
// 		return nil, fmt.Errorf("[NetworkTraffic Plugin] Invalid response, expected 1 value, got %d", len(nodeMeasure))
// 	}
// 	return nodeMeasure[0], nil
// }

func (p *PrometheusHandle) query(promQL string) (model.Value, error) {
	results, warnings, err := p.Client.Query(context.Background(), promQL, time.Now())
	if len(warnings) > 0 {
		klog.Warningf("[prometheus] Warnings: %v\n", warnings)
	}

	return results, err
}
