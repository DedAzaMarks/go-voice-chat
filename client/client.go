package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"

	voicechat "github.com/DedAzaMarks/go-voice-chat"
	uuid "github.com/satori/go.uuid"
)

var DefaultReceiveBufferSize = 1024
var DefaultSendBufferSize = 1024

type Client struct {
	ClientID    uuid.UUID
	Receive     chan voicechat.AudioFrame
	send        chan voicechat.AudioFrame
	audioPacket voicechat.Packet

	Echo bool

	remoteAddr *net.UDPAddr
	conn       *net.UDPConn

	exit chan struct{}
	wg   *sync.WaitGroup
}

func NewClient(addr string) (*Client, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	clientID := uuid.NewV4()
	c := &Client{
		ClientID: clientID,
		Receive:  make(chan voicechat.AudioFrame, DefaultReceiveBufferSize),
		send:     make(chan voicechat.AudioFrame, DefaultSendBufferSize),
		audioPacket: voicechat.Packet{
			Type:     voicechat.PacketAudio,
			ClientID: clientID,
		},
		remoteAddr: udpAddr,
		conn:       conn,
		exit:       make(chan struct{}),
		wg:         &sync.WaitGroup{},
	}

	c.wg.Add(2)
	go c.sendLoop()
	go c.readLoop()

	return c, nil
}

func (c *Client) Close() error {
	close(c.exit)
	err := c.conn.Close()
	c.wg.Wait()

	return err
}

func (c *Client) Join() error {
	p := voicechat.Packet{
		ClientID: c.ClientID,
		Type:     voicechat.PacketJoin,
	}
	buf, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	fmt.Println(hex.Dump(buf))
	_, err = c.conn.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Leave() error {
	p := voicechat.Packet{
		ClientID: c.ClientID,
		Type:     voicechat.PacketLeave,
	}
	buf, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	fmt.Println(hex.Dump(buf))
	_, err = c.conn.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Send(frame voicechat.AudioFrame) {
	c.send <- frame
}

func (c *Client) sendLoop() {
	defer c.wg.Done()
	buf := bytes.NewBuffer(make([]byte, 0, 1500))
	for {
		select {
		case _, ok := <-c.exit:
			if !ok {
				return
			}
		case frame := <-c.send:
			c.audioPacket.AudioFrames = []voicechat.AudioFrame{frame}
			buf.Reset()
			_, err := c.audioPacket.WriteTo(buf)
			if err != nil {
				log.Printf("error : %v", err)
				break
			}

			_, err = c.conn.Write(buf.Bytes())
			if err != nil {
				log.Printf("error : %v", err)
				break
			}
		}
	}
}

func (c *Client) readLoop() {
	defer c.wg.Done()
	buf := make([]byte, 1500)
	for {
		select {
		case _, ok := <-c.exit:
			if !ok {
				return
			}
		default:
			n, err := c.conn.Read(buf)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
					// ignore
				} else {
					log.Printf("error : %v", err)
				}
				break
			}
			var p voicechat.Packet
			err = p.UnmarshalBinary(buf[0:n])
			if err != nil {
				log.Printf("error : %v", err)
				break
			}

			if p.Type != voicechat.PacketAudio {
				// ignore
				break
			}

			if !c.Echo && uuid.Equal(p.ClientID, c.ClientID) {
				// ignore
				break
			}

			for _, frame := range p.AudioFrames {
				c.Receive <- frame
			}
		}
	}
}
