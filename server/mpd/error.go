package mpd

import "fmt"

// ACK error codes
// https://github.com/MusicPlayerDaemon/MPD/blob/master/src/protocol/Ack.hxx
type ackError uint

const (
	ackErrorNotList    ackError = 1
	ackErrorArg                 = 2
	ackErrorPassword            = 3
	ackErrorPermission          = 4
	ackErrorUnknown             = 5

	ackErrorNoExist       = 50
	ackErrorPlaylistMax   = 51
	ackErrorSystem        = 52
	ackErrorPlaylistLoad  = 53
	ackErrorUpdateAlready = 54
	ackErrorPlayerSync    = 55
	ackErrorExist         = 56
)

type errorResponse struct {
	code       ackError
	cmdListNum uint
	cmdName    string
	err        error
}

func newError(code ackError, cmdIdx uint, cmdName string, err error) *errorResponse {
	return &errorResponse{code, cmdIdx, cmdName, err}
}

func (e errorResponse) String() string {
	return fmt.Sprintf("ACK [%d@%d] {%s} %s", e.code, e.cmdListNum, e.cmdName, e.err.Error())
}

func (e errorResponse) Error() string {
	return fmt.Sprintf("%s: %v", e.cmdName, e.err)
}

func (e errorResponse) Unwrap() error {
	return e.err
}
