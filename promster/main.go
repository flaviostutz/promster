package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go.etcd.io/etcd/clientv3"

	"github.com/flaviostutz/etcd-registry/etcd-registry"
	"github.com/serialx/hashring"

	"github.com/sirupsen/logrus"
)

func main() {
	logLevel := flag.String("loglevel", "debug", "debug, info, warning, error")
	etcdURL0 := flag.String("etcd-url", "", "ETCD URLs. ex: http://etcd0:2379")
	etcdURLScrape0 := flag.String("etcd-url-scrape", "", "ETCD URLs for scrape source server. If empty, will be the same as --etcd-url. ex: http://etcd0:2379")
	etcdBase0 := flag.String("etcd-base", "/registry", "ETCD base path for services")
	etcdServiceName0 := flag.String("etcd-service-name", "", "Prometheus cluster service name. Ex.: proml1")
	etcdServiceTTL0 := flag.Int("etcd-node-ttl", -1, "Node registration TTL in ETCD. After killing Promster instance, it will vanish from ETCD registry after this time")
	scrapeEtcdPath0 := flag.String("scrape-etcd-path", "", "Base ETCD path for getting servers to be scrapped")
	flag.Parse()

	etcdURL := *etcdURL0
	etcdURLScrape := *etcdURLScrape0
	etcdBase := *etcdBase0
	etcdServiceName := *etcdServiceName0
	scrapeEtcdPath := *scrapeEtcdPath0
	etcdServiceTTL := *etcdServiceTTL0

	if etcdURL == "" {
		panic("--etcd-url should be defined")
	}
	if etcdURLScrape == "" {
		panic("--etcd-url-scrape should be defined")
	}

	if etcdBase == "" {
		panic("--etcd-base should be defined")
	}
	if etcdServiceName == "" {
		panic("--etcd-service-name should be defined")
	}
	if scrapeEtcdPath == "" {
		panic("--scrape-etcd-path should be defined")
	}
	if etcdServiceTTL == -1 {
		panic("--etcd-node-ttl should be defined")
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

	endpoints := strings.Split(etcdURL, ",")
	reg, err := etcdregistry.NewEtcdRegistry(endpoints, etcdBase, 10*time.Second)
	if err != nil {
		panic(err)
	}

	logrus.Debugf("Initializing ETCD client")
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 10 * time.Second})
	if err != nil {
		logrus.Errorf("Could not initialize ETCD client. err=%s", err)
		panic(err)
	}
	logrus.Infof("Etcd client initialized")
	servicePath := fmt.Sprintf("%s/%s/", etcdBase, etcdServiceName)

	logrus.Infof("Starting to watch registered prometheus nodes...")
	nodesChan := make(chan []string, 0)
	go watchRegisteredNodes(cli, servicePath, nodesChan)

	logrus.Infof("Starting to watch source scrape targets...")
	sourceTargetsChan := make(chan []string, 0)
	go watchSourceScrapeTargets(cli, scrapeEtcdPath, sourceTargetsChan)

	logrus.Infof("Keeping self node registered on ETCD...")
	go keepSelfNodeRegistered(reg, etcdServiceName, time.Duration(etcdServiceTTL)*time.Second)

	promNodes := make([]string, 0)
	scrapeTargets := make([]string, 0)
	go func() {
		for {
			logrus.Debugf("Prometheus nodes found: %s", promNodes)
			logrus.Debugf("Scrape targets found: %s", scrapeTargets)
			time.Sleep(5 * time.Second)
		}
	}()

	for {
		select {
		case promNodes = <-nodesChan:
			logrus.Debugf("updated promNodes: %s", promNodes)
		case scrapeTargets = <-sourceTargetsChan:
			logrus.Debugf("updated scapeTargets: %s", scrapeTargets)
		}
		err := updatePrometheusTargets(scrapeTargets, promNodes)
		if err != nil {
			logrus.Warnf("Couldn't update Prometheus scrape targets. err=%s", err)
		}
	}
}

func updatePrometheusTargets(scrapeTargets []string, promNodes []string) error {
	//Apply consistent hashing to determine which scrape endpoints will
	//be handled by this Prometheus instance
	ring := hashring.New(promNodes)
	selfNodeName := getSelfNodeName()
	selfScrapeTargets := make([]string, 0)
	for _, starget := range scrapeTargets {
		promNode, ok := ring.GetNode(starget)
		if !ok {
			return fmt.Errorf("Couldn't get prometheus node for %s in consistent hash", starget)
		}
		logrus.Debugf("Target %s - Prometheus %s", starget, promNode)
		if promNode == selfNodeName {
			selfScrapeTargets = append(selfScrapeTargets, starget)
		}
	}

	//generate json file
	servers := make([]map[string][]string, 0)
	targetsm := make(map[string][]string)
	targetsm["targets"] = selfScrapeTargets
	servers = append(servers, targetsm)

	contents, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	logrus.Debugf("Writing /servers.json: '%s'", string(contents))
	err = ioutil.WriteFile("/servers.json", contents, 0666)
	if err != nil {
		return err
	}

	//force Prometheus to update its configuration live
	_, err = ExecShell("wget --post-data='' http://localhost:9090/-/reload -O -")
	if err != nil {
		return nil
	}
	// output, err0 := ExecShell("kill -HUP $(ps | grep prometheus | awk '{print $1}' | head -1)")
	// if err0 != nil {
	// 	logrus.Warnf("Could not reload Prometheus configuration. err=%s. output=%s", err0, output)
	// }

	return nil
}

func keepSelfNodeRegistered(reg *etcdregistry.EtcdRegistry, etcdServiceName string, ttl time.Duration) {
	node := etcdregistry.Node{}
	node.Name = getSelfNodeName()
	logrus.Debugf("Registering Prometheus instance on ETCD registry. service=%; node=%s", etcdServiceName, node)
	err := reg.RegisterNode(context.TODO(), etcdServiceName, node, ttl)
	if err != nil {
		panic(err)
	}
}

func getSelfNodeName() string {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s:9090", hostname)
}

func watchSourceScrapeTargets(cli *clientv3.Client, sourceTargetsPath string, sourceTargetsChan chan []string) {
	logrus.Debugf("Getting source scrape targets from %s", sourceTargetsPath)

	watchChan := cli.Watch(context.TODO(), sourceTargetsPath, clientv3.WithPrefix())
	for {
		logrus.Debugf("Source scrape targets updated")
		rsp, err0 := cli.Get(context.TODO(), sourceTargetsPath, clientv3.WithPrefix())
		if err0 != nil {
			logrus.Warnf("Error retrieving source scrape targets. err=%s", err0)
		}

		if len(rsp.Kvs) == 0 {
			logrus.Debugf("No source scrape targets were found under %s", sourceTargetsPath)

		} else {
			sourceTargets := make([]string, 0)
			for _, kv := range rsp.Kvs {
				sourceTargets = append(sourceTargets, path.Base(string(kv.Key)))
			}
			sourceTargetsChan <- sourceTargets
			logrus.Debugf("Found source scrape targets: %s", sourceTargets)
		}
		<-watchChan
	}

	// logrus.Infof("Updating scrape targets for this shard to %s")
}

func watchRegisteredNodes(cli *clientv3.Client, servicePath string, nodesChan chan []string) {
	watchChan := cli.Watch(context.TODO(), servicePath, clientv3.WithPrefix())
	for {
		logrus.Debugf("Registered nodes updated")
		rsp, err0 := cli.Get(context.TODO(), servicePath, clientv3.WithPrefix())
		if err0 != nil {
			logrus.Warnf("Error retrieving service nodes. err=%s", err0)
		}

		if len(rsp.Kvs) == 0 {
			logrus.Debugf("No services nodes were found under %s", servicePath)

		} else {
			promNodes := make([]string, 0)
			for _, kv := range rsp.Kvs {
				promNodes = append(promNodes, path.Base(string(kv.Key)))
			}
			nodesChan <- promNodes
			logrus.Debugf("Found registered nodes %s", promNodes)
		}
		<-watchChan
	}
}
