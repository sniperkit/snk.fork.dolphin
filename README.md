# dolphin
简单的发布部署系统后端(WIP)

计划：
  1. CI 基于drone, pipeline 支持非docker镜像打包（直接调用本地命令）
  2. 打包的release包，放在git仓库中， 但只保留最近的几次的提交历史 （shadow)
  3. 支持配置行动生成，可以参考（https://github.com/kelseyhightower/confd, https://github.com/kubernetes/helm)
  4. 发布与进行监控集成，控制服务的生命周期。
  5. 提交命令行及api两种接口
