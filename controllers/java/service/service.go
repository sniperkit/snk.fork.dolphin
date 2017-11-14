package service

import "we.com/dolphin/types"

/*
{"address":"10.10.10.59","type":1,"port":40341,"startTime":"2017-11-13 18:03:19","mainclass":"com.to8to.weixin.server.WeixinServer","pid":27417,"reconnectZK":0,"version":"97"}

{"address":"10.10.10.30","type":1,"port":40364,"time":"2017-06-22 00:21:26"}

{"type":3,"port":0,"time":"2017-07-17 09:24:42"}

{"pid":13943,"version":"7","bind_ip":"0.0.0.0","report_ip":"10.10.10.82","port":40080,"start_time":"2017-09-22 19:07:05","type":1,"method":["views.contractBill.generate","contractBill.query","accountItem.findById","views.contractItem.queryPage","accountItem.findByIds","contractBill.update","views.accountItem.getAccountItem","contractBill.findById","contractBill.create","contractBill.deleteByIds","views.contractBill.queryPage","contractBill.findByIds","views.contractBill.getContractAndItem","contractBill.deleteById","views.contractBill.getDetail","contractItem.query","accountItem.query","views.accountItem.queryPage","contractItem.findByIds","contractItem.findById"]}

{"pid":43024,"version":"7","report_ip":"10.10.10.51","start_time":"2017-10-28 15:35:53","type":0}
*/
type service struct {
	stage         types.Stage
	name          types.DeployName
	types         types.ServiceType
	status        string
	route         string
	newVersion    string
	lastVersion   string
	numOfInstance int
}

type esb struct {
	apiVerion string
	stage     types.Stage
	host      string
	port      string
}

/*
	service:
		num of instance running
		num of instance configed
		num of version
		num of instance started
		probe fail ratio
			num of timeout
			num of err

*/

type manager struct {
	stage types.Stage
	esbs  map[string][]*esb
}

func Probe() {}
