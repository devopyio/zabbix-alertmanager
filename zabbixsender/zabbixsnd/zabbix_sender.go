package zabbixsnd

import (
	"encoding/binary"
	"encoding/json"
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

//NewPacket creates new packet
func NewPacket(data []*Metric, clock ...int64) *Packet {
	p := &Packet{Request: `sender data`, Data: data}
	// use current time, if `clock` is not specified
	if p.Clock = time.Now().Unix(); len(clock) > 0 {
		p.Clock = int64(clock[0])
	}
	return p
}

// DataLen Packet return 8 bytes with packet length in little endian order
func (p *Packet) DataLen() ([]byte, error) {
	dataLen := make([]byte, 8)
	JSONData, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(dataLen, uint32(len(JSONData)))
	return dataLen, nil
}

// Sender sends data to zabbix
// Read more: https://www.zabbix.com/documentation/3.4/manual/config/items/itemtypes/trapper
type Sender struct {
	addr *net.TCPAddr
}

// New creates new sender
func New(addr string) (*Sender, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &Sender{
		addr: tcpAddr,
	}, nil
}

// Send method Sender class, send packet to zabbix
func (s *Sender) Send(packet *Packet) ([]byte, error) {
	conn, err := net.DialTCP("tcp", nil, s.addr)
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

	buffer := append(Header, datalen...)
	buffer = append(buffer, dataPacket...)

	_, err = conn.Write(buffer)
	if err != nil {
		return nil, err
	}

	res, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, err
	}

	return res, nil
}
