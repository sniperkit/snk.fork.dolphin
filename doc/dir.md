# 目录结构

- api 计划提供的api接口
- cmd 各种程序的入口
- conf 配置文件sample
- controllers 业务逻辑们, 实例调试，java服务状态， 主机状态监控等
- deploy 发布相关，当前没有使用
- doc 文档目录
- logger 日志相关
- process agent在使用，用到监控当前主机上运行各类服务的进程，及进程的资源使用情况， 进程的状态等
- registry etcd 相关的操作
- report 数据上报到influxdb的辅助类
- types 基础的数据结构及模型，及相关的配置原型
- vendor 依赖的第三方包