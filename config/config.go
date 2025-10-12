package config

var Cfg = struct {
    Platform struct {
        IP   string
        Port int
        URL  string
    }
    Site1 struct {
        IP   string
        Port int
        URL  string
    }
    Site2 struct {
        IP   string
        Port int
        URL  string
    }
    SMA struct {
        IP   string
        Port int
        URL  string
    }
    PS struct {
        IP   string
        Port int
        URL  string
    }
    MonitoredSites []string
    Resource       struct {
        Site1Total int
        Site2Total int
    }
}{
    Platform: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "172.28.125.175",
        Port: 8080,
        URL:  "http://172.28.125.175:8080",
    },
    Site1: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "172.22.118.77",
        Port: 8081,
        URL:  "http://172.22.118.77:8081",
    },
    Site2: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "192.168.243.185",
        Port: 8082,
        URL:  "http://192.168.243.185:8082",
    },
    SMA: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "172.28.125.175",
        Port: 8083,
        URL:  "http://172.28.125.175:8083",
    },
    PS: struct {
        IP   string
        Port int
        URL  string
    }{
        IP:   "172.28.125.175",
        Port: 8084,
        URL:  "http://172.28.125.175:8084",
    },
    MonitoredSites: []string{
        "http://172.22.118.77:8081",
        "http://192.168.243.185:8082",
    },
    Resource: struct {
        Site1Total int
        Site2Total int
    }{
        Site1Total: 400,
        Site2Total: 500,
    },
}
