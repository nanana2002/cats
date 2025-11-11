package config

import (
	"reflect"
	"sort"
	"strings"
)

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
	Site3 struct {
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
		IP:   "192.168.235.48",
		Port: 8081,
		URL:  "http://192.168.235.48:8081",
	},
	Site2: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "172.25.21.118",
		Port: 8082,
		URL:  "http://172.25.21.118:8082",
	},
	Site3: struct {
		IP   string
		Port int
		URL  string
	}{
		IP:   "172.22.118.77",
		Port: 8085,
		URL:  "http://172.22.118.77:8085",
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
		"http://192.168.235.48:8081",
		"http://172.25.21.118:8082",
		"http://172.22.118.77:8085",
	},
	Resource: struct {
		Site1Total int
		Site2Total int
	}{
		Site1Total: 400,
		Site2Total: 500,
	},
}

func GetAllSiteURLs() []string {
	cfgVal := reflect.ValueOf(Cfg)
	cfgType := reflect.TypeOf(Cfg)

	var urls []string
	for i := 0; i < cfgVal.NumField(); i++ {
		field := cfgType.Field(i)
		fieldName := field.Name

		if strings.HasPrefix(fieldName, "Site") {
			siteVal := cfgVal.Field(i)
			if !siteVal.IsValid() || siteVal.Kind() != reflect.Struct {
				continue
			}

			urlField := siteVal.FieldByName("URL")
			if !urlField.IsValid() || urlField.Kind() != reflect.String {
				continue
			}

			urlStr := urlField.String()
			if urlStr == "" {
				continue
			}

			base := strings.TrimRight(urlStr, "/")
			metricsURL := base + "/metrics"
			urls = append(urls, metricsURL)
		}
	}

	seen := make(map[string]bool)
	var result []string
	for _, u := range urls {
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}
	sort.Strings(result)
	return result
}
