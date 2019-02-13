package etcdregistry

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
)

//ETCDClient struct
type ETCDClient struct {
	ctx context.Context
	cli *clientv3.Client
	kv  clientv3.KV
}

func newETCDClient(ctx0 context.Context, endpoints []string) (*ETCDClient, error) {
	d := &ETCDClient{}
	d.ctx = ctx0

	cli0, err := clientv3.New(clientv3.Config{Context: ctx0, Endpoints: endpoints, DialTimeout: 5 * time.Second})
	if err != nil {
		logrus.Errorf("Could not initialize ETCD client. err=%s", err)
		return nil, err
	}
	d.cli = cli0
	d.kv = clientv3.NewKV(cli0)
	logrus.Debugf("ETCD client initialized")
	return d, nil
}

func (d *ETCDClient) setValueTTL(key string, value string, ttlSeconds int64) error {
	lease, err := d.cli.Grant(d.ctx, ttlSeconds)
	if err != nil {
		return err
	}
	_, err = d.kv.Put(d.ctx, key, value, clientv3.WithLease(lease.ID))
	if err != nil {
		return err
	}
	return nil
}

func (d *ETCDClient) getValues(key string) ([]string, error) {
	gr, err := d.kv.Get(d.ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	if len(gr.Kvs) == 0 {
		return []string{}, nil
	}
	values := make([]string, 0)
	for _, v := range gr.Kvs {
		values = append(values, v.String())
	}
	return values, nil
}

func (d *ETCDClient) close() error {
	return d.cli.Close()
}
