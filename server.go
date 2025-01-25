package goquery

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
)

type QueryServer struct {
	queryInfo    map[string]string
	queryInfoMut sync.Mutex

	players    []string
	playersMut sync.Mutex

	tokenGen tokenGenerator
	listener *net.UDPConn
	stop     chan bool
}

func New(queryInfo map[string]string, players []string) *QueryServer {
	query := &QueryServer{queryInfo: queryInfo, tokenGen: tokenGenerator{}, players: players}
	query.tokenGen.generateToken()
	return query
}

func (q *QueryServer) SetQueryInfo(queryInfo map[string]string) {
	q.queryInfoMut.Lock()
	q.queryInfo = queryInfo
	q.queryInfoMut.Unlock()
}

func (q *QueryServer) Set(key, value string) {
	q.queryInfoMut.Lock()
	q.queryInfo[key] = value
	q.queryInfoMut.Unlock()
}

func (q *QueryServer) SetPlayers(players []string) {
	q.playersMut.Lock()
	q.players = players
	q.playersMut.Unlock()
}

func (q *QueryServer) Handle(buf *bytes.Buffer, addr net.Addr) error {
	pk := request{}
	if err := pk.Unmarshal(buf); err != nil {
		return err
	}

	var resp response = response{}
	switch pk.RequestType {
	case queryTypeHandshake:
		resp = response{ResponseType: queryTypeHandshake, SequenceNumber: pk.SequenceNumber, ResponseNumber: int32(getTokenString(q.tokenGen.token, addr.String()))}
	case queryTypeInformation:
		if pk.ResponseNumber != int32(getTokenString(q.tokenGen.token, addr.String())) {
			return errors.New("mismatched response number")
		}
		q.queryInfoMut.Lock()
		q.playersMut.Lock()
		resp = response{ResponseType: queryTypeInformation, SequenceNumber: pk.SequenceNumber, ResponseNumber: pk.ResponseNumber, Information: q.queryInfo, Players: q.players}
		q.queryInfoMut.Unlock()
		q.playersMut.Unlock()
	}
	buf.Reset()
	resp.Marshal(buf)
	return nil
}

func (q *QueryServer) Serve(addr string) error {
	q.stop = make(chan bool)

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	q.listener = conn
	if err != nil {
		return err
	}
	for {
		var buf = make([]byte, 512)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-q.stop:
				return nil
			default:
				continue
			}
		}

		r := bytes.NewBuffer(buf[:n])

		if err := q.Handle(r, addr); err != nil {
			fmt.Println(err)
			continue
		}

		conn.WriteToUDP(r.Bytes(), addr)
	}
}

func (q *QueryServer) Close() error {
	close(q.stop)
	if q.listener == nil {
		return errors.New("listener is nil")
	}
	return q.listener.Close()
}
