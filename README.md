# clientgo-get-k8s-resources
使用 clientgo 获取 k8s pod的requests与limit以及调用prometheus获取实际使用的情况


## 因为golang可能的内存复制问题，导致计算函数一直错误
解决方案，在Excel里面使用sumif函数来进行筛选

逻辑：CPU 当前Pod的CPU使用情况所在服务器的占比(平均七天) + 当前的pod的Requests Cpu占当前服务器的Requests的占比 /2   
      内存 当前Pod的内存使用情况所在服务器的占比（平均七天） + 当前的Pod的Requests 内存占当前服务器的Requests的占比 /2  
注：未使用的CPU与内存会随着占比分摊至各Pod上       

以A2这个字段举例  
分摊的服务器 CPU占比=((E2/(SUMIF(A:A,A2,E:E)))+(I2/(SUMIF(A:A,A2,I:I))))/2
分摊的服务器 内存占比=((F2/(SUMIF(A:A,A2,F:F)))+(J2/(SUMIF(A:A,A2,J:J))))/2

汇总的语句  
=(SUM(L2*K2)+SUM(M2*K2))/2

## 执行命令  

新增了执行命令的 prometheusUrl，kubeconfigPath选项，一个是填写 http|https://prometheus的监控地址:端口  
kubeconfigPath 填写 kubeconfig的地址,不填默认是 /root/.kube/config   

```powershell
# QA环境
go run main.go --prometheusUrl http://prometheus.test.newhopescm.com --kubeconfigPath /root/.kube/yyx-ali-qa.yaml

#Prod环境
go run main.go --prometheusUrl http://prometheus.prod.newhopescm.com --kubeconfigPath /root/.kube/yyx-ali-prod.yaml
```



