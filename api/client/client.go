package client

//go:generate mockery -name Client

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/quilt/quilt/api"
	"github.com/quilt/quilt/api/pb"
	"github.com/quilt/quilt/db"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	// The timeout for making requests to the daemon once we've connected.
	requestTimeout = time.Minute

	// The timeout for connecting to the daemon.
	connectTimeout = 5 * time.Second
)

// Client provides methods to interact with the Quilt daemon.
type Client interface {
	// Close the grpc connection.
	Close() error

	// QueryMachines retrieves the machines tracked by the Quilt daemon.
	QueryMachines() ([]db.Machine, error)

	// QueryContainers retrieves the containers tracked by the Quilt daemon.
	QueryContainers() ([]db.Container, error)

	// QueryEtcd retrieves the etcd information tracked by the Quilt daemon.
	QueryEtcd() ([]db.Etcd, error)

	// QueryConnections retrieves the connection information tracked by the
	// Quilt daemon.
	QueryConnections() ([]db.Connection, error)

	// QueryLabels retrieves the label information tracked by the Quilt daemon.
	QueryLabels() ([]db.Label, error)

	// QueryClusters retrieves cluster information tracked by the Quilt daemon.
	QueryClusters() ([]db.Cluster, error)

	// QueryCounters retrieves the debugging counters tracked with the Quilt daemon.
	QueryCounters() ([]pb.Counter, error)

	// Deploy makes a request to the Quilt daemon to deploy the given deployment.
	Deploy(deployment string) error

	// Version retrieves the Quilt version of the remote daemon.
	Version() (string, error)
}

// Getter obtains a client connected to the given address.
type Getter func(string) (Client, error)

type clientImpl struct {
	pbClient pb.APIClient
	cc       *grpc.ClientConn
}

// New creates a new Quilt client connected to `lAddr`.
func New(lAddr string) (Client, error) {
	proto, addr, err := api.ParseListenAddress(lAddr)
	if err != nil {
		return nil, err
	}

	dialer := func(dialAddr string, t time.Duration) (net.Conn, error) {
		return net.DialTimeout(proto, dialAddr, t)
	}
	cc, err := grpc.Dial(addr, grpc.WithDialer(dialer), grpc.WithInsecure(),
		grpc.WithBlock(), grpc.WithTimeout(connectTimeout))
	if err != nil {
		if err == context.DeadlineExceeded {
			err = daemonTimeoutError{
				host:         lAddr,
				connectError: err,
			}
		}
		return nil, err
	}

	pbClient := pb.NewAPIClient(cc)
	return clientImpl{
		pbClient: pbClient,
		cc:       cc,
	}, nil
}

func query(pbClient pb.APIClient, table db.TableType) (interface{}, error) {
	ctx, _ := context.WithTimeout(context.Background(), requestTimeout)
	reply, err := pbClient.Query(ctx, &pb.DBQuery{Table: string(table)})
	if err != nil {
		return nil, err
	}

	replyBytes := []byte(reply.TableContents)
	switch table {
	case db.MachineTable:
		var machines []db.Machine
		if err := json.Unmarshal(replyBytes, &machines); err != nil {
			return nil, err
		}
		return machines, nil
	case db.ContainerTable:
		var containers []db.Container
		if err := json.Unmarshal(replyBytes, &containers); err != nil {
			return nil, err
		}
		return containers, nil
	case db.EtcdTable:
		var etcds []db.Etcd
		if err := json.Unmarshal(replyBytes, &etcds); err != nil {
			return nil, err
		}
		return etcds, nil
	case db.LabelTable:
		var labels []db.Label
		if err := json.Unmarshal(replyBytes, &labels); err != nil {
			return nil, err
		}
		return labels, nil
	case db.ConnectionTable:
		var connections []db.Connection
		if err := json.Unmarshal(replyBytes, &connections); err != nil {
			return nil, err
		}
		return connections, nil
	case db.ClusterTable:
		var clusters []db.Cluster
		if err := json.Unmarshal(replyBytes, &clusters); err != nil {
			return nil, err
		}
		return clusters, nil
	default:
		panic(fmt.Sprintf("unsupported table type: %s", table))
	}
}

// Close the grpc connection.
func (c clientImpl) Close() error {
	return c.cc.Close()
}

// QueryMachines retrieves the machines tracked by the Quilt daemon.
func (c clientImpl) QueryMachines() ([]db.Machine, error) {
	rows, err := query(c.pbClient, db.MachineTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Machine), nil
}

// QueryContainers retrieves the containers tracked by the Quilt daemon.
func (c clientImpl) QueryContainers() ([]db.Container, error) {
	rows, err := query(c.pbClient, db.ContainerTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Container), nil
}

// QueryEtcd retrieves the etcd information tracked by the Quilt daemon.
func (c clientImpl) QueryEtcd() ([]db.Etcd, error) {
	rows, err := query(c.pbClient, db.EtcdTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Etcd), nil
}

// QueryConnections retrieves the connection information tracked by the Quilt daemon.
func (c clientImpl) QueryConnections() ([]db.Connection, error) {
	rows, err := query(c.pbClient, db.ConnectionTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Connection), nil
}

// QueryLabels retrieves the label information tracked by the Quilt daemon.
func (c clientImpl) QueryLabels() ([]db.Label, error) {
	rows, err := query(c.pbClient, db.LabelTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Label), nil
}

// QueryClusters retrieves the cluster information tracked by the Quilt daemon.
func (c clientImpl) QueryClusters() ([]db.Cluster, error) {
	rows, err := query(c.pbClient, db.ClusterTable)
	if err != nil {
		return nil, err
	}

	return rows.([]db.Cluster), nil
}

// QueryCounters retrieves the debugging counters tracked with the Quilt daemon.
func (c clientImpl) QueryCounters() ([]pb.Counter, error) {
	ctx, _ := context.WithTimeout(context.Background(), requestTimeout)
	reply, err := c.pbClient.QueryCounters(ctx, &pb.CountersRequest{})
	if err != nil {
		return nil, err
	}

	var counters []pb.Counter
	for _, c := range reply.Counters {
		counters = append(counters, *c)
	}

	return counters, nil
}

// Deploy makes a request to the Quilt daemon to deploy the given deployment.
func (c clientImpl) Deploy(deployment string) error {
	ctx, _ := context.WithTimeout(context.Background(), requestTimeout)
	_, err := c.pbClient.Deploy(ctx, &pb.DeployRequest{Deployment: deployment})
	return err
}

// Version retrieves the Quilt version of the remote daemon.
func (c clientImpl) Version() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), requestTimeout)
	version, err := c.pbClient.Version(ctx, &pb.VersionRequest{})
	if err != nil {
		return "", err
	}
	return version.Version, nil
}

// daemonTimeoutError represents when we are unable to connect to the Quilt
// daemon because of a timeout.
type daemonTimeoutError struct {
	host         string
	connectError error
}

func (err daemonTimeoutError) Error() string {
	return fmt.Sprintf("Unable to connect to the Quilt daemon at %s: %s. "+
		"Is the quilt daemon running? If not, you can start it with "+
		"`quilt daemon`.", err.host, err.connectError.Error())
}
