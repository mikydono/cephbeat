package pools

import (
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/elastic/beats/libbeat/logp"

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
  pool *rados.IOContext
	poolName string
	cluster *rados.Conn
	clusterConfPath string
}

type tag struct {
	poolName string
}

// New create a new instance of the MetricSet
// Part of new is also setting up the configuration by processing additional
// configuration entries if needed.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {

	// Get parameters from cephbeat.yml configuration file
	//   ClusterConfPath: path to ceph cluster configuration file (contains monitors
	//                    and osd endpoints, path to keyring, etc.
	config := struct{
		ClusterConfPath string `config:"cluster_conf_path"`
		PoolName string `config:"pool_name"`
	}{
		ClusterConfPath: "/etc/ceph/cluster.conf",
		PoolName: "rbd",
	}

	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	// Connection to Ceph cluster
	logp.Info("Cluster configuration path: %s", config.ClusterConfPath)
	logp.Info("Connection to Ceph cluster...")
	conn, _ := rados.NewConn()
        conn.ReadConfigFile(config.ClusterConfPath)
        if conn.Connect() != nil {
                logp.Err("Unable to connect cluster")
        }

	// Open a pool IOContext
	ioctx, err := conn.OpenIOContext(config.PoolName)
	if err != nil {
		logp.Err("Unable to open IOContext of pool %s", config.PoolName)
	}

	return &MetricSet{
		BaseMetricSet: base,
		pool: ioctx,
		poolName: config.PoolName,
		cluster: conn,
		clusterConfPath: config.ClusterConfPath,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right format
// It returns the event which is then forward to the output. In case of an error, a
// descriptive error must be returned.
func (m *MetricSet) Fetch() ([]common.MapStr, error) {

	// Because of a mysterious behavour of ListPools method,
	// we have to initiate a new connection to the Ceph cluster
	// for every Fetch. If we don't do it, ListPools does not
	// return new created pools.
	logp.Info("Connection to Ceph cluster...")
	conn, _ := rados.NewConn()
				conn.ReadConfigFile(m.clusterConfPath)
				if conn.Connect() != nil {
								logp.Err("Unable to connect cluster")
				}

	// List pools
	pools, err := conn.ListPools()
	if err != nil {
		logp.Err("Unable to get pools")
	}
	logp.Info("%s", pools)

	// Get stats on each pool
	events := make([]common.MapStr, len(pools))
	i := 0
	for _,pool_name := range pools {

		// Open a pool IOContext
		ioctx, err := conn.OpenIOContext(pool_name)
		if err != nil {
			logp.Err("Unable to open IOContext of pool %s", pool_name)
		}

		// Get pool stats
		stats, err := ioctx.GetPoolStats()
		if err != nil {
			logp.Err("Unable to get stats")
		}

		event := common.MapStr{
			"pool_name": pool_name,
			"stats": stats,
		}

		events[i] = event
		i += 1
	}

	return events, nil
}
