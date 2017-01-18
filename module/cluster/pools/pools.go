package pools

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

	if err := mb.Registry.AddMetricSet("cluster", "pools", New); err != nil {
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

	logp.Info("Cluster configuration path: %s", config.ClusterConfPath)

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
func (m *MetricSet) Fetch() ([]common.MapStr, error) {

	// Wait for the latest OSD map.
	// If we don't do it, the list of pools will be unconsistent.
	for m.cluster.WaitForLatestOSDMap() != nil {
		logp.Err("Unable to wait for latest OSD map")
		time.Sleep(30 * time.Second)
	}

	// List pools
	pools, err := m.cluster.ListPools()
	if err != nil {
		logp.Err("Unable to get pools")
	}

	// Get stats on each pool
	events := make([]common.MapStr, len(pools))
	i := 0
	for _, pool_name := range pools {

		// Open a pool IOContext
		ioctx, err := m.cluster.OpenIOContext(pool_name)
		if err != nil {
			logp.Err("Unable to open IOContext of pool %s", pool_name)
			continue
		}

		// Get pool stats
		stats, err := ioctx.GetPoolStats()
		if err != nil {
			logp.Err("Unable to get stats")
			continue
		}

		event := common.MapStr{
			"pool_name": pool_name,
			"stats":     stats,
		}

		events[i] = event
		i += 1
	}

	logp.Info("Stats of %d pools fetched", len(pools))

	return events, nil
}
