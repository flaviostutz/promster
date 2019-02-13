package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	logLevel := flag.String("loglevel", "debug", "debug, info, warning, error")
	prometheusConfigFile := flag.String("config-file", "/prometheus.yml", "prometheus.yml file that will be managed by Promster")
	etcdURL0 := flag.String("etcd-url", "", "ETCD URLs. ex: http://etcd0:2379")
	scrapeEtcdPath0 := flag.String("scrape-etcd-path", "", "Base ETCD path for getting servers to be scrapped")
	flag.Parse()

	fmt.Sprintf("%s %s", prometheusConfigFile, scrapeEtcdPath0)

	etcdURL := *etcdURL0
	// scrapeEtcdPath := *scrapeEtcdPath0

	if etcdURL == "" {
		panic("--etcd-url should be defined")
	}

	switch *logLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
		break
	case "warning":
		logrus.SetLevel(logrus.WarnLevel)
		break
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
		break
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	logrus.Infof("====Starting Promster====")
	// logrus.Infof("Generating prometheus.yml")
	// sourceCode, err := executeTemplate("/", "prometheus.tmpl", templateRulesMap)
	// if err != nil {
	// 	panic(err)
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	endpoints := strings.Split(etcdURL, ",")
	logrus.Debugf("Setting up ETCD client to %s", endpoints)

	etcdcli, err := newETCDClient(ctx, endpoints)
	defer etcdcli.close()
	if err != nil {
		panic(err)
	}

	err = etcdcli.setValueTTL("test/a", "aaa", 10)
	if err != nil {
		panic(err)
	}
	err = etcdcli.setValueTTL("test/b", "bbb", 10)
	if err != nil {
		panic(err)
	}
	err = etcdcli.setValueTTL("test/c", "ccc", 10)
	if err != nil {
		panic(err)
	}

	time.Sleep(12 * time.Second)

	v, err := etcdcli.getValues("test")
	if err != nil {
		panic(err)
	}
	logrus.Debugf(">>>>LIST %s", v)

	v, err = etcdcli.getValues("test/a")
	if err != nil {
		panic(err)
	}
	logrus.Debugf(">>>>VALUE %s", v)

}
