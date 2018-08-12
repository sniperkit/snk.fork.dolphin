/*
Sniperkit-Bot
- Status: analyzed
*/

/*
	Package deply是agent基于git worktree的发布新服务及更新旧的服务的包， 包含下面几个功能：
		1. 发布，备份
		2. 还原到一个旧的版本（ 与回滚不完全相同）， 还原是reset, 回滚是revert
		3. 查看本地服务的更新日志
		4. 发布时根据配置模板自己生成配置文件 （参考confd)

	可以支持下面场景的发布：
		一台机器上要部署多个es节点，每个节点可能属于不同的集群， 每个结点的配置不同， 自动生成
		php网站， 文件大小1.5G，
		后台java服务


	实现 (由于worktree 是git 2.x以后才支持的， 安装会比较麻烦，这种集成了src-d/go-git的 go语言实现：
		对于每个项目， 它会在agent本地维护一个bare仓库，发布就是创建及更新worktree。
		回滚就是reset worktree
		更新记录： 就是git log
		备份： 就是在备份分支上： git add -A && git commit

	看上去很简单，但考虑我们支持三种不同的更新策略和java服务的部署每次可能会schedual到不同的机器，实际上还不有点麻烦，
	大概发布的过程是：
		更新本地repo


*/

package deploy
