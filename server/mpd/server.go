package mpd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"

	"go.senan.xyz/gonic/jukebox"
)

const (
	protocolVersion = "0"
	protocolHello   = "OK MPD " + protocolVersion
)

var errCmdListEnd = errors.New("command_list_end")

type Server struct {
	jukebox *jukebox.Jukebox
}

func New(jukebx *jukebox.Jukebox) (*Server, error) {
	s := &Server{
		jukebx,
	}

	return s, nil
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	addrPort, err := netip.ParseAddrPort(addr)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(addrPort))
	if err != nil {
		return nil
	}

	return s.Serve(ctx, l)
}

func (s *Server) Serve(ctx context.Context, l *net.TCPListener) error {
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			return err
		}

		go func() {
			conn := connection{s, conn}

			err := conn.handle(ctx)
			if err != nil {
				log.Printf("error handling MPD client: %s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

type connection struct { // TODO?: remove this type
	*Server
	*net.TCPConn
}

func (c *connection) writeLine(line string) error {
	n, err := io.WriteString(c.TCPConn, line+"\n")
	_ = n
	return err
}

func (c *connection) writePair(name, value string) error {
	return c.writeLine(fmt.Sprintf("%s: %s", name, value))
}

func (c *connection) handle(ctx context.Context) error {
	defer c.Close()

	c.writeLine(protocolHello)

	lines := newLineReader(bufio.NewReader(c.TCPConn))

	err := c.handleCmd(lines)
	if err != nil {
		var errRsp *errorResponse
		if errors.As(err, &errRsp) {
			c.writeLine(errRsp.String())
			return nil
		}

		return err
	}

	return c.writeLine("OK")
}

func (c *connection) handleCmd(lines *lineReader) error {
	cmdIdx, line, err := lines.Next()
	if err != nil {
		return fmt.Errorf("could not read command %d: %w", cmdIdx, err)
	}

	args := newArgParser(line)

	_, cmd, ok := args.Next()
	if !ok {
		return errors.New("empty command")
	}

	return c.doCmd(0, cmd, args, lines)
}

func (c *connection) doCmd(idx uint, name string, args *argParser, lines *lineReader) error {
	fmt.Println("->", name)

	switch name {
	case "command_list_ok_begin":
		return c.doCmdListOkBegin(args, lines)
	case "command_list_end":
		return c.doCmdListEnd(args)
	case "currentsong":
		return c.doCurrentSong(args)
	case "status":
		return c.doStatus(args)
	}

	return newError(ackErrorNotList, idx, name, fmt.Errorf("unknown command: %s", name))
}

// parseArgs returns an array of values matching names, or an error.
func (c *connection) parseArgs(args *argParser, names ...string) ([]string, error) {
	values := make([]string, 0, len(names))

	for i := range names {
		_, val, ok := args.Next()
		if !ok {
			idx := uint(0) // FIXME
			name := ""     // FIXME
			return nil, newError(ackErrorArg, idx, name,
				fmt.Errorf("got only %d args, missing: %s", i+1, names[i:]))
		}

		values = append(values, val)
	}

	_, extra, ok := args.Next()
	if ok {
		idx := uint(0) // FIXME
		name := ""     // FIXME
		return nil, newError(ackErrorArg, idx, name,
			fmt.Errorf("too many args, first extra value: %s", extra))
	}

	return values, nil
}

func (c *connection) doCmdListOkBegin(args *argParser, lines *lineReader) error {
	if _, err := c.parseArgs(args); err != nil {
		return err
	}

	for {
		err := c.handleCmd(lines)
		if err != nil {
			if errors.Is(err, errCmdListEnd) {
				return nil
			}

			return err
		}

		c.writeLine("list_OK")
	}
}

func (c *connection) doCmdListEnd(args *argParser) error {
	if _, err := c.parseArgs(args); err != nil {
		return err
	}

	return errCmdListEnd
}

func (c *connection) doCurrentSong(args *argParser) error {
	if _, err := c.parseArgs(args); err != nil {
		return err
	}

	if c.jukebox == nil {
		return nil
	}

	status, err := c.jukebox.GetStatus()
	if err != nil {
		return err
	}

	if status.CurrentIndex < 0 {
		return nil
	}

	c.writePair("file", status.CurrentFilename)

	return nil
}

func (c *connection) doStatus(args *argParser) error {
	if _, err := c.parseArgs(args); err != nil {
		return err
	}

	if c.jukebox == nil {
		c.writePair("state", "stop")
		return nil
	}

	status, err := c.jukebox.GetStatus()
	if err != nil {
		return err
	}

	if status.CurrentIndex < 0 {
		c.writePair("state", "stop")
		return nil
	}

	if status.Playing {
		c.writePair("state", "play")
	} else {
		c.writePair("state", "pause")
	}

	return nil
}
