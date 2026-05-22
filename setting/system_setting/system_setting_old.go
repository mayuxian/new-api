package system_setting

var ServerAddress = "http://localhost:3000"
var RedirectDownloadUrl = ""
var WorkerUrl = ""
var WorkerValidKey = ""
var WorkerAllowHttpImageRequestEnabled = false

func EnableWorker() bool {
	return WorkerUrl != ""
}
