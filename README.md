# clientgo-get-k8s-resources
使用 clientgo 获取 k8s pod的requests与limit以及调用prometheus获取实际使用的情况


## 因为golang可能的内存复制问题，导致计算函数一直错误
解决方案，在Excel里面使用sumif函数来进行筛选

逻辑：CPU 当前Pod的CPU所在服务器的占比 + 当前的pod的Requests Cpu占当前服务器的Requests的占比 /2   
      内存 当前Pod的内存所在服务器的占比 + 当前的Pod的Requests 内存占当前服务器的Requests的占比 /2   

以A4这个字段举例
分摊的服务器CPU占比=((D4/(SUMIF(A:A,A4,D:D)))+(H4/(SUMIF(A:A,A4,H:H))))/2  
分摊的服务器内存占比=((E4/(SUMIF(A:A,A4,E:E)))+(I4/(SUMIF(A:A,A4,I:I))))/2  

汇总的语句

