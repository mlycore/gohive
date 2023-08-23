package gohive

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"
	bgohive "github.com/beltran/gohive"
	hiveserver2 "github.com/beltran/gohive/hiveserver"
)

type drv struct{}

func (d drv) Open(dsn string) (driver.Conn, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	socket, err := thrift.NewTSocket(cfg.Addr)
	if err != nil {
		return nil, err
	}
	var transport thrift.TTransport
	if cfg.Auth == "NOSASL" {
		transport = thrift.NewTBufferedTransport(socket, 4096)
		if transport == nil {
			return nil, fmt.Errorf("BufferedTransport is nil")
		}
	} else if cfg.Auth == "PLAIN" || cfg.Auth == "GSSAPI" || cfg.Auth == "LDAP" {
		saslCfg := map[string]string{
			"username": cfg.User,
			"password": cfg.Passwd,
		}
		bgTransport, err := bgohive.NewTSaslTransport(socket, cfg.Addr, cfg.Auth, saslCfg)
		if err != nil {
			return nil, fmt.Errorf("create SasalTranposrt failed: %v", err)
		}
		bgTransport.SetMaxLength(uint32(cfg.Batch))
		transport = bgTransport
	} else if cfg.Auth == "KERBEROS" {
		configuration := bgohive.NewConnectConfiguration()
		configuration.Service = "hive"
		// Previously kinit should have done: kinit -kt ./secret.keytab hive/hs2.example.com@EXAMPLE.COM
		// connection, errConn := bgohive.Connect("hs2.example.com", 10000, "KERBEROS", configuration)
		host := strings.Split(cfg.Addr, ":")[0]
		port := strings.Split(cfg.Addr, ":")[1]
		p, _ := strconv.Atoi(port)
		connection, errConn := bgohive.Connect(host, p, "KERBEROS", configuration)
		if errConn != nil {
			return nil, fmt.Errorf("create Kerberos failed: %v", err)
		}
		options := hiveOptions{PollIntervalSeconds: 5, BatchSize: int64(cfg.Batch)}
		conn := &hiveConnection{
			thrift:  connection.Client,
			session: connection.SessionHandle,
			options: options,
			ctx:     context.Background(),
		}

	} else {
		return nil, fmt.Errorf("unrecognized auth mechanism: %s", cfg.Auth)
	}
	if err = transport.Open(); err != nil {
		return nil, err
	}

	protocol := thrift.NewTBinaryProtocolFactoryDefault()
	client := hiveserver2.NewTCLIServiceClientFactory(transport, protocol)
	s := hiveserver2.NewTOpenSessionReq()
	s.ClientProtocol = hiveserver2.TProtocolVersion_HIVE_CLI_SERVICE_PROTOCOL_V6
	if cfg.User != "" {
		s.Username = &cfg.User
		if cfg.Passwd != "" {
			s.Password = &cfg.Passwd
		}
	}
	config := cfg.SessionCfg
	if cfg.DBName != "" {
		config["use:database"] = cfg.DBName
	}
	s.Configuration = config
	session, err := client.OpenSession(context.Background(), s)
	if err != nil {
		return nil, err
	}

	options := hiveOptions{PollIntervalSeconds: 5, BatchSize: int64(cfg.Batch)}
	conn := &hiveConnection{
		thrift:  client,
		session: session.SessionHandle,
		options: options,
		ctx:     context.Background(),
	}
	return conn, nil
}

func init() {
	sql.Register("hive", &drv{})
}
