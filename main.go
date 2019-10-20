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

	"github.com/coreos/etcd/clientv3"
	"github.com/flaviostutz/etcd-registry/etcd-registry"
	"github.com/serialx/hashring"

	"github.com/sirupsen/logrus"
)

// SourceTarget defines the structure of a prometheus source target
type SourceTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

func main() {

	logLevel := flag.String("loglevel", "info", "debug, info, warning, error")
	etcdURLRegistry0 := flag.String("registry-etcd-url", "", "ETCD URLs. ex: http://etcd0:2379")
	etcdBase0 := flag.String("registry-etcd-base", "/registry", "ETCD base path for services")
	etcdServiceName0 := flag.String("registry-service-name", "", "Prometheus cluster service name. Ex.: proml1")
	etcdServiceTTL0 := flag.Int("registry-node-ttl", -1, "Node registration TTL in ETCD. After killing Promster instance, it will vanish from ETCD registry after this time")
	etcdURLScrape0 := flag.String("scrape-etcd-url", "", "ETCD URLs for scrape source server. If empty, will be the same as --etcd-url. ex: http://etcd0:2379")
	scrapeEtcdPath0 := flag.String("scrape-etcd-path", "", "Base ETCD path for getting servers to be scrapped")
	scrapePaths0 := flag.String("scrape-paths", "/metrics", "URI for scrape of each target. May contain a list separated by ','.")
	scrapeInterval0 := flag.String("scrape-interval", "30s", "Prometheus scrape interval")
	scrapeTimeout0 := flag.String("scrape-timeout", "30s", "Prometheus scrape timeout")
	scrapeMatch0 := flag.String("scrape-match", "", "Metrics regex filter applied on scraped targets. Commonly used in conjunction with /federate metrics endpoint")
	scrapeShardingEnable0 := flag.Bool("scrape-shard-enable", false, "Enable sharding distribution among targets so that each Promster instance will scrape a different set of targets, enabling distribution of load among instances. Defaults to true.")
	evaluationInterval0 := flag.String("evaluation-interval", "30s", "Prometheus evaluation interval")
	flag.Parse()

	etcdURLRegistry := *etcdURLRegistry0
	etcdURLScrape := *etcdURLScrape0
	etcdBase := *etcdBase0
	etcdServiceName := *etcdServiceName0
	scrapeEtcdPath := *scrapeEtcdPath0
	etcdServiceTTL := *etcdServiceTTL0
	scrapeInterval := *scrapeInterval0
	scrapeTimeout := *scrapeTimeout0
	scrapeMatch := *scrapeMatch0
	scrapeShardingEnable := *scrapeShardingEnable0
	evaluationInterval := *evaluationInterval0
	se := *scrapePaths0
	scrapePaths := strings.Split(se, ",")

	// if etcdURLRegistry == "" {
	// 	panic("--etcd-url-registry should be defined")
	// }
	if etcdURLScrape == "" {
		panic("--etcd-url-scrape should be defined")
	}

	if etcdURLRegistry != "" {
		if etcdBase == "" {
			panic("--etcd-base should be defined")
		}
		if etcdServiceName == "" {
			panic("--etcd-service-name should be defined")
		}
		if etcdServiceTTL == -1 {
			panic("--etcd-node-ttl should be defined")
		}
	}
	if scrapeEtcdPath == "" {
		panic("--scrape-etcd-path should be defined")
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

	logrus.Debugf("Updating prometheus file...")
	time.Sleep(5 * time.Second)
	err := updatePrometheusConfig("/prometheus.yml", scrapeInterval, scrapeTimeout, evaluationInterval, scrapePaths, scrapeMatch)
	if err != nil {
		panic(err)
	}

	logrus.Debugf("Creating rules file...")
	err = createRulesFromENV("/rules.yml")
	if err != nil {
		panic(err)
	}

	nodesChan := make(chan []string, 0)
	if etcdURLRegistry != "" {
		logrus.Debugf("Initializing Registry client. etcdURLRegistry=%s", etcdURLRegistry)
		endpointsRegistry := strings.Split(etcdURLRegistry, ",")
		registry, err := etcdregistry.NewEtcdRegistry(endpointsRegistry, etcdBase, 10*time.Second)
		if err != nil {
			panic(err)
		}
		logrus.Infof("Keeping self node registered on ETCD...")
		go keepSelfNodeRegistered(registry, etcdServiceName, time.Duration(etcdServiceTTL)*time.Second)

		logrus.Debugf("Initializing ETCD client for registry")
		cliRegistry, err := clientv3.New(clientv3.Config{Endpoints: endpointsRegistry, DialTimeout: 10 * time.Second})
		if err != nil {
			logrus.Errorf("Could not initialize ETCD client. err=%s", err)
			panic(err)
		}
		logrus.Infof("Etcd client initialized")
		servicePath := fmt.Sprintf("%s/%s/", etcdBase, etcdServiceName)

		logrus.Infof("Starting to watch registered prometheus nodes...")
		go watchRegisteredNodes(cliRegistry, servicePath, nodesChan)
	} else {
		go func() {
			nodesChan <- []string{getSelfNodeName()}
		}()
	}

	logrus.Debugf("Initializing ETCD client for source scrape targets")
	logrus.Infof("Starting to watch source scrape targets. etcdURLScrape=%s", etcdURLScrape)
	endpointsScrape := strings.Split(etcdURLScrape, ",")
	cliScrape, err := clientv3.New(clientv3.Config{Endpoints: endpointsScrape, DialTimeout: 10 * time.Second})
	if err != nil {
		logrus.Errorf("Could not initialize ETCD client. err=%s", err)
		panic(err)
	}
	logrus.Infof("Etcd client initialized for scrape")
	sourceTargetsChan := make(chan []SourceTarget, 0)
	go watchSourceScrapeTargets(cliScrape, scrapeEtcdPath, sourceTargetsChan)

	promNodes := make([]string, 0)
	scrapeTargets := make([]SourceTarget, 0)
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
		err := updatePrometheusTargets(scrapeTargets, promNodes, scrapeShardingEnable)
		if err != nil {
			logrus.Warnf("Couldn't update Prometheus scrape targets. err=%s", err)
		}
	}
}

func updatePrometheusConfig(prometheusFile string, scrapeInterval string, scrapeTimeout string, evaluationInterval string, scrapePaths []string, scrapeMatch string) error {
	logrus.Infof("updatePrometheusConfig. scrapeInterval=%s,scrapeTimeout=%s,evaluationInterval=%s,scrapePaths=%s,scrapeMatch=%s", scrapeInterval, scrapeTimeout, evaluationInterval, scrapePaths, scrapeMatch)
	input := make(map[string]interface{})
	input["scrapeInterval"] = scrapeInterval
	input["scrapeTimeout"] = scrapeTimeout
	input["evaluationInterval"] = evaluationInterval
	input["scrapePaths"] = scrapePaths
	input["scrapeMatch"] = scrapeMatch
	contents, err := executeTemplate("/", "prometheus.yml.tmpl", input)
	if err != nil {
		return err
	}

	logrus.Debugf("%s: '%s'", prometheusFile, contents)
	err = ioutil.WriteFile(prometheusFile, []byte(contents), 0666)
	if err != nil {
		return err
	}

	_, err = ExecShell("wget --post-data='' http://localhost:9090/-/reload -O -")
	if err != nil {
		logrus.Warnf("Couldn't reload Prometheus config. Maybe it wasn't initialized at this time and will get the config as soon as getting started. Ignoring.")
	}

	return nil
}

func createRulesFromENV(rulesFile string) error {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		env[pair[0]] = pair[1]
	}

	rules := make(map[string]string)
	for i := 1; i < 100; i++ {
		kname := fmt.Sprintf("RECORD_RULE_%d_NAME", i)
		kexpr := fmt.Sprintf("RECORD_RULE_%d_EXPR", i)
		vname, exists := env[kname]
		if !exists {
			break
		}
		vexpr, exists := env[kexpr]
		if !exists {
			break
		}
		rules[vname] = vexpr
	}

	if len(rules) == 0 {
		logrus.Infof("No prometheus rules found in environment variables")
		return nil
	}

	logrus.Debugf("Found %d rules: %s", len(rules), rules)

	rulesContents := `groups:
  - name: env-rules
    rules:`

	for k, v := range rules {
		rc := `%s
    - record: %s
      expr: %s`
		rulesContents = fmt.Sprintf(rc, rulesContents, k, v)
	}

	logrus.Debugf("rulesContents: %s", rulesContents)

	logrus.Debugf("%s: '%s'", rulesFile, rulesContents)
	err := ioutil.WriteFile(rulesFile, []byte(rulesContents), 0666)
	if err != nil {
		return err
	}

	_, err = ExecShell("wget --post-data='' http://localhost:9090/-/reload -O -")
	if err != nil {
		logrus.Warnf("Couldn't reload Prometheus config. Maybe it wasn't initialized at this time and will get the config as soon as getting started. Ignoring.")
	}

	return nil
}

func updatePrometheusTargets(scrapeTargets []SourceTarget, promNodes []string, shardingEnabled bool) error {
	//Apply consistent hashing to determine which scrape endpoints will
	//be handled by this Prometheus instance
	logrus.Debugf("updatePrometheusTargets. scrapeTargets=%s, promNodes=%s", scrapeTargets, promNodes)

	ring := hashring.New(hashList(promNodes))
	selfNodeName := getSelfNodeName()
	selfScrapeTargets := make([]SourceTarget, 0)
	for _, starget := range scrapeTargets {
		hashedPromNode, ok := ring.GetNode(stringSha512(starget.Targets[0]))
		if !ok {
			return fmt.Errorf("Couldn't get prometheus node for %s in consistent hash", starget.Targets[0])
		}
		logrus.Debugf("Target %s - Prometheus %x", starget, hashedPromNode)
		hashedSelf := stringSha512(selfNodeName)
		if !shardingEnabled || hashedSelf == hashedPromNode {
			logrus.Debugf("Target %s - Prometheus %s", starget, selfNodeName)
			selfScrapeTargets = append(selfScrapeTargets, starget)
		}
	}

	//generate json file
	contents, err := json.Marshal(selfScrapeTargets)
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
		return err
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
	logrus.Debugf("Registering Prometheus instance on ETCD registry. service=%s; node=%s", etcdServiceName, node)
	err := reg.RegisterNode(context.TODO(), etcdServiceName, node, ttl)
	if err != nil {
		panic(err)
	}
}

func getSelfNodeName() string {
	hostip, err := ExecShell("ip route get 8.8.8.8 | grep -oE 'src ([0-9\\.]+)' | cut -d ' ' -f 2")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s:9090", strings.TrimSpace(hostip))
}

func watchSourceScrapeTargets(cli *clientv3.Client, sourceTargetsPath string, sourceTargetsChan chan []SourceTarget) {
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
			sourceTargets := make([]SourceTarget, 0)
			for _, kv := range rsp.Kvs {
				record := string(kv.Key)
				targetAddress := path.Base(record)
				serviceName := path.Base(path.Dir(record))
				sourceTargets = append(sourceTargets, SourceTarget{Labels: map[string]string{"prsn": serviceName}, Targets: []string{targetAddress}})
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
