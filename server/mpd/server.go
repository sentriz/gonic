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

type client struct {
	srv *Server

	lines *lineReader
	out   io.Writer

	cmdIdx  uint
	cmdName string
}

func newClient(srv *Server, conn *net.TCPConn) *client {
	lines := newLineReader(bufio.NewReader(conn))

	return &client{
		srv: srv,

		lines: lines,
		out:   conn,
	}
}

func (c *client) writeLine(line string) error {
	_, err := io.WriteString(c.out, line+"\n")
	return err
}

func (c *client) writePair(name, value string) error {
	return c.writeLine(fmt.Sprintf("%s: %s", name, value))
}

func (c *client) newErr(code ackError, err error) error {
	return newError(code, c.cmdIdx, c.cmdName, err)
}

func (c *client) nextCmd() (string, *argParser, error) {
	cmdIdx, line, err := c.lines.Next()
	if err != nil {
		return "", nil, fmt.Errorf("could not read command %d: %w", cmdIdx, err)
	}

	args := newArgParser(line)

	cmd, ok := args.Next()
	if !ok {
		return "", nil, errors.New("empty command")
	}

	return cmd, args, nil
}

func (s *Server) handle(c *client) error {
	c.writeLine(protocolHello)

	for {
		cmd, args, err := c.nextCmd()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		err = doCmd(c, cmd, args)
		if err != nil {
			if errors.Is(err, errCmdListEnd) {
				err = c.newErr(ackErrorNotList, errors.New("no command list in progress"))
			}

			var errRsp *errorResponse
			if errors.As(err, &errRsp) {
				c.writeLine(errRsp.String())
				return nil
			}

			return err
		}

		err = c.writeLine("OK")
		if err != nil {
			return err
		}
	}
}

type cmdHandler func(c *client, args *argParser) error

var cmdHandlers map[string]cmdHandler

//nolint:noinit // initializing `cmdHandlers` inline causes a ref-loop build error
func init() {
	cmdHandlers = map[string]cmdHandler{
		"command_list_begin":    doCmdListBegin,
		"command_list_end":      doCmdListEnd,
		"command_list_ok_begin": doCmdListOkBegin,
		"currentsong":           doCurrentSong,
		"pause":                 doPause,
		"play":                  doPlay,
		"status":                doStatus,
	}
}

func doCmd(c *client, name string, args *argParser) error {
	fmt.Println("->", args.line)

	handler, ok := cmdHandlers[name]
	if !ok {
		return c.newErr(ackErrorNotList, fmt.Errorf("unknown command: %s", name))
	}

	return handler(c, args)
}

// parseArgs returns an array of values matching names, or an error.
func parseArgs(c *client, args *argParser, names ...string) ([]string, error) {
	values := make([]string, 0, len(names))

	for i := range names {
		val, ok := args.Next()
		if !ok {
			return nil, c.newErr(ackErrorArg, fmt.Errorf("got only %d args, missing: %s", i+1, names[i:]))
		}

		values = append(values, val)
	}

	extra, ok := args.Next()
	if ok {
		return nil, c.newErr(ackErrorArg, fmt.Errorf("too many args, first extra value: %s", extra))
	}

	return values, nil
}

func doCmdListOkBegin(c *client, args *argParser) error {
	return handleCmdList(c, args, true)
}

func doCmdListBegin(c *client, args *argParser) error {
	return handleCmdList(c, args, false)
}

func handleCmdList(c *client, args *argParser, sendOk bool) error {
	if _, err := parseArgs(c, args); err != nil {
		return err
	}

	for {
		cmd, args, err := c.nextCmd()
		if err != nil {
			return err
		}

		err = doCmd(c, cmd, args)
		if err != nil {
			if errors.Is(err, errCmdListEnd) {
				return nil
			}

			return err
		}

		if sendOk {
			c.writeLine("list_OK")
		}
	}
}

func doCmdListEnd(c *client, args *argParser) error {
	if _, err := parseArgs(c, args); err != nil {
		return err
	}

	return errCmdListEnd
}

func doCurrentSong(c *client, args *argParser) error {
	if _, err := parseArgs(c, args); err != nil {
		return err
	}

	if c.srv.jukebox == nil {
		return nil
	}

	status, err := c.srv.jukebox.GetStatus()
	if err != nil {
		return err
	}

	if status.CurrentIndex < 0 {
		return nil
	}

	c.writePair("file", status.CurrentFilename)

	return nil
}

func doPlay(c *client, args *argParser) error {
	songpos, ok := args.Next()
	if !ok {
		return c.srv.jukebox.Play()
	}

	i, err := strconv.Atoi(songpos)
	if err != nil {
		return c.newErr(ackErrorArg, fmt.Errorf("invalid SONGPOS: %w", err))
	}

	return c.srv.jukebox.SkipToPlaylistIndex(i, 0)
}

func doPause(c *client, args *argParser) error {
	state, ok := args.Next()
	switch {
	case !ok: // no arg, toggle
		return c.srv.jukebox.TogglePlay()

	case state == "1" || state == "0":
		return c.srv.jukebox.SetPlay(state == "0")

	default:
		return c.newErr(ackErrorArg, fmt.Errorf("play state must be 0 or 1, got: %s", state))
	}
}

func doStatus(c *client, args *argParser) error {
	if _, err := parseArgs(c, args); err != nil {
		return err
	}

	if c.srv.jukebox == nil {
		c.writePair("state", "stop")
		return nil
	}

	status, err := c.srv.jukebox.GetStatus()
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
