# clientgo-get-k8s-resources
使用 clientgo 获取 k8s pod的requests与limit以及调用prometheus获取实际使用的情况


## 因为golang可能的内存复制问题，导致计算函数一直错误
解决方案，在Excel里面使用sumif函数来进行筛选

逻辑：CPU 当前Pod的CPU所在服务器的占比 + 当前的pod的Requests Cpu占当前服务器的Requests的占比 /2   
      内存 当前Pod的内存所在服务器的占比 + 当前的Pod的Requests 内存占当前服务器的Requests的占比 /2   

以A2这个字段举例  
分摊的服务器 CPU占比=((E2/(SUMIF(A:A,A2,E:E)))+(I2/(SUMIF(A:A,A2,I:I))))/2
分摊的服务器 内存占比=((F2/(SUMIF(A:A,A2,F:F)))+(J2/(SUMIF(A:A,A2,J:J))))/2

汇总的语句  
=(SUM(J2*K2)+SUM(J2*L2))/2  

## 执行命令  

新增了执行命令的 prometheusUrl，kubeconfigPath选项，一个是填写 http|https://prometheus的监控地址:端口  
kubeconfigPath 填写 kubeconfig的地址,不填默认是 /root/.kube/config   

```powershell
./clientgo-get-k8s-resources --prometheusUrl http://prometheus.test.newhopescm.com --kubeconfigPath /root/.kube/yyx-ali-qa.yaml
```



