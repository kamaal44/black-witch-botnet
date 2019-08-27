package client

import (
	"crypto/tls"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net"
	"soulless_network/relations"
	"time"
)

type Client struct {
	Addr       string
	Conn       net.Conn
	ReconnTime time.Duration
}

// TODO: v1.0.0
// realize own proto
// implement cd

// TODO: next
// hello message from server (as part as internal monitor maybe?)
// save logs to file
// add daemon file
// Что будет если коннекшин разорветься здесь когда мы в консоли (разрыв на получении данных)
// Command.Data split to command and args

func (c *Client) Run() {
	c.connect()

	for {
		log.Print("*")
		msg, err := c.read()

		if err != nil {
			log.Printf("Error read response %s\n", err)
			c.reconnect()
			continue
		}

		var res *relations.Response
		ch := make(chan struct{}, 1)

		go func() {
			res = c.handle(msg)
			ch <- struct{}{}
		}()

		select {
		case <-ch:
			err = c.write(res)

			if err != nil {
				log.Printf("Error write response %s\n", err)
			}
		case <-time.After(10 * time.Second):
			res = &relations.Response{
				Type: relations.TypeErrorResult,
				Data: &relations.ErrorResult{
					Code: 2,
					Data: "run command timeout",
				},
			}

			err = c.write(res)

			if err != nil {
				log.Printf("Error write response %s\n", err)
			}
		}
	}
}

func (c *Client) connect() {
	conn, err := c.dial()

	if err != nil {
		log.Println("[TCP] Dialing connection", err)
		c.reconnect()
		return
	}

	log.Printf("[TCP] Successfully connected %s", c.Addr)
	c.Conn = conn
}

func (c *Client) reconnect() {
	log.Printf("[*] Reconnecting in %d seconds\n", c.ReconnTime)
	time.Sleep(c.ReconnTime * time.Second)
	c.connect()
}

func (c *Client) dial() (*tls.Conn, error) {
	dialer := &net.Dialer{KeepAlive: 1 * time.Second}
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	return tls.DialWithDialer(dialer, "tcp", c.Addr, conf)
}

func (c *Client) write(res *relations.Response) error {
	b, err := bson.Marshal(res)

	if err != nil {
		log.Printf("[BSON] Marshaling message %s\n", err)
		return err
	}

	_, err = c.Conn.Write(append(b, '\r'))

	if err != nil {
		log.Printf("[TCP] Writing the message %s\n", err)
		return err
	}

	return nil
}

func (c *Client) read() (*relations.Command, error) {
	reader := bufio.NewReader(c.Conn)
	b, err := reader.ReadBytes('\r')

	if err != nil {
		log.Printf("[TCP] Reading the sent message %s\n", err)
		return nil, err
	}

	cmd := &relations.Command{}
	err = bson.Unmarshal(b, cmd)

	if err != nil {
		log.Printf("[BSON] Unmarshaling message %s\n", err)
		return nil, err
	}

	return cmd, nil
}

func (c *Client) handle(cmd *relations.Command) *relations.Response {
	h := Handler{cmd}
	return h.handle()
}
