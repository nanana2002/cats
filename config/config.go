package config

// LOCAL_LISTEN_IP 供本地服务 (Platform, C-SMA, C-PS) 实际监听使用 (应为 0.0.0.0 或 127.0.0.1)
var LOCAL_LISTEN_IP = "127.0.0.1"

var Cfg = struct {
    Platform struct{ IP string; Port int; URL string }
    Site1    struct{ IP string; Port int; URL string }
    Site2    struct{ IP string; Port int; URL string }
    SMA      struct{ IP string; Port int; URL string }
    PS       struct{ IP string; Port int; URL string }
    MonitoredSites []string
    Resource struct{ Site1Total, Site2Total int }
}{
    Platform: struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8080, URL: "http://192.168.67.185:8080"},
    Site1:    struct{ IP string; Port int; URL string }{IP: "192.168.235.48", Port: 8081, URL: "http://192.168.235.48:8081"},
    Site2:    struct{ IP string; Port int; URL string }{IP: "192.168.67.159", Port: 8085, URL: "http://192.168.67.159:8085"},
    SMA:      struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8083, URL: "http://192.168.67.185:8083"},
    PS:       struct{ IP string; Port int; URL string }{IP: "192.168.67.185", Port: 8084, URL: "http://192.168.67.185:8084"},
    MonitoredSites: []string{
        "http://192.168.235.48:8081",
        "http://192.168.67.159:8085",
    },
    Resource: struct{ Site1Total, Site2Total int }{Site1Total: 400, Site2Total: 500},
}

// GetAllSiteURLs 用于 C-SMA 模块，它需要一个函数来获取所有监控站点的URL
func GetAllSiteURLs() []string {
    return Cfg.MonitoredSites
}
