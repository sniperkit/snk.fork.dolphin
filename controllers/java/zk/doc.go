/*
Sniperkit-Bot
- Status: analyzed
*/

/*
	zk 相关的一些操作

	zkpath.go: 查询java服务的相关zk路径， 信息是从项目的配置文件上读取的
	zk.go: zk操作的简单客户端
	sync_zk.go: 监听zk上java服务状态及配置变化，并将状态同步到etcd
*/

package zk
