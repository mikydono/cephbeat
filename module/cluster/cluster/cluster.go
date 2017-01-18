package cluster

import (
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/metricbeat/mb"

	"github.com/ceph/go-ceph/rados"
)

// init registers the MetricSet with the central registry.
// The New method will be called after the setup of the module and before starting to fetch data
func init() {

	if err := mb.Registry.AddMetricSet("cluster", "cluster", New); err != nil {
		panic(err)
	}
}

// MetricSet type defines all fields of the MetricSet
// As a minimum it must inherit the mb.BaseMetricSet fields, but can be extended with
// additional entries. These variables can be used to persist data or configuration between
// multiple fetch calls.
type MetricSet struct {
	mb.BaseMetricSet
	cluster *rados.Conn
}

// New create a new instance of the MetricSet
// Part of new is also setting up the configuration by processing additional
// configuration entries if needed.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {

	// Get parameters from cephbeat.yml configuration file
	//   ClusterConfPath: path to ceph cluster configuration file (contains monitors
	//                    and osd endpoints, path to keyring, etc.
	config := struct {
		ClusterConfPath string `config:"cluster_conf_path"`
	}{
		ClusterConfPath: "/etc/ceph/cluster.conf",
	}

	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	// Connection to Ceph cluster
	logp.Info("Connection to Ceph cluster...")
	conn, _ := rados.NewConn()
	conn.ReadConfigFile(config.ClusterConfPath)
	for conn.Connect() != nil {
		logp.Err("Unable to connect cluster")
		time.Sleep(30 * time.Second)
		conn, _ = rados.NewConn()
		conn.ReadConfigFile(config.ClusterConfPath)
	}

	return &MetricSet{
		BaseMetricSet: base,
		cluster:       conn,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right format
// It returns the event which is then forward to the output. In case of an error, a
// descriptive error must be returned.
func (m *MetricSet) Fetch() (common.MapStr, error) {

	stats, err := m.cluster.GetClusterStats()
	if err != nil {
		logp.Err("Unable to get stats")
	}
	fsid, err := m.cluster.GetFSID()
	if err != nil {
		logp.Err("Unable to get fsid")
	}
	pools, err := m.cluster.ListPools()
	if err != nil {
		logp.Err("Unable to get pools")
	}

	event := common.MapStr{
		"stats": stats,
		"fsid":  fsid,
		"pools": pools,
	}

	return event, nil
}
