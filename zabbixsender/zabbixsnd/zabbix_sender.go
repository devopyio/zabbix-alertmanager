package zabbixsnd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"time"
)

var Header = []byte("ZBXD\x01")

type Metric struct {
	Host  string `json:"host"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Clock int64  `json:"clock"`
}

type Packet struct {
	Request string    `json:"request"`
	Data    []*Metric `json:"data"`
	Clock   int64     `json:"clock"`
}

//NewPacket Packet class cunstructor
func NewPacket(data []*Metric, clock ...int64) *Packet {
	p := &Packet{Request: `sender data`, Data: data}
	// use current time, if `clock` is not specified
	if p.Clock = time.Now().Unix(); len(clock) > 0 {
		p.Clock = int64(clock[0])
	}
	return p
}

// DataLen Packet class method, return 8 bytes with packet length in little endian order
func (p *Packet) DataLen() ([]byte, error) {
	dataLen := make([]byte, 8)
	JSONData, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(dataLen, uint32(len(JSONData)))
	return dataLen, nil
}

type Sender struct {
	Host string
	Port int
}

// Method Sender class, read data from connection
func (s *Sender) read(conn *net.TCPConn) ([]byte, error) {
	res, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, err
	}

	return res, nil
}

//Send method Sender class, send packet to zabbix
func (s *Sender) Send(packet *Packet) ([]byte, error) {
	// Open connection to zabbix host
	iaddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", s.Host, s.Port))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, iaddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dataPacket, err := json.Marshal(packet)
	if err != nil {
		return nil, err
	}

	datalen, err := packet.DataLen()
	if err != nil {
		return nil, err
	}
	// Fill buffer
	buffer := append(Header, datalen...)

	buffer = append(buffer, dataPacket...)

	// Sent packet to zabbix
	_, err = conn.Write(buffer)
	if err != nil {
		return nil, err
	}

	res, err := s.read(conn)
	if err != nil {
		return nil, err
	}

	return res, nil
}
