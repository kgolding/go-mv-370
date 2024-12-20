package mv370

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/ziutek/telnet"
)

type Mv370 struct {
	host     string
	username string
	password string
	log      *slog.Logger
}

type ReadLinesFn func(line string) (done bool, err error)

func New(host string, username string, password string, logger *slog.Logger) (Mv370, error) {
	if logger == nil {
		// Create a dummy logger
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mv := Mv370{
		host:     host,
		username: username,
		password: password,
		log:      logger,
	}
	mv.log.Info("New Mv370", "host", host)
	return mv, mv.Check()
}

// Check creates a session, authenticates and runs the info command to check the MV370 can be contacted
func (mv Mv370) Check() error {
	return mv.Session().
		Sendln("info").
		Expect("info").
		Close()
}

func (mv Mv370) SendSMS(tel string, message string) error {
	return mv.Session().
		Sendln("module1").
		Expect("to release").
		Sendln("at+cmgf=1").
		Expect("0").
		Sendln(`at+cmgs="` + tel + `"`).
		Expect(">").
		Sendln(message).
		Send([]byte{0x1A}). // Ctrl-Z
		Close()
}

type Message struct {
	Tel  string    `json:"tel"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

func (mv Mv370) ReadSMS() ([]Message, error) {
	messages := make([]Message, 0)

	callback := func(line string) (bool, error) {
		if strings.HasPrefix(line, "+CMGL:") {
			// New message
			msg := Message{}
			elements, err := ParseCSVLine(line[6:])
			if err != nil {
				return true, err
			}
			for i, e := range elements {
				e = strings.TrimSpace(e)
				if len(e) > 1 && strings.HasPrefix(e, `"`) && strings.HasSuffix(e, `"`) {
					e = e[1 : len(e)-2]
				}
				switch i {
				case 0: // Index
				case 1: // Message box
				case 2: // Telephone
					msg.Tel = e
				case 3: // Unknown
				case 4: // Date
					var err error
					if len(e) < 17 {
						err = fmt.Errorf("invlaid date '%s'", e)
					} else {
						msg.Time, err = time.Parse("06/01/02,15:04:05", e[:17])
					}
					if err != nil {
						return true, err
					}
				}
			}
			messages = append(messages, msg)
		} else if line == "0" {
			return true, nil
		} else if l := len(messages); l > 0 {
			if len(messages[l-1].Text) > 0 {
				messages[l-1].Text += "\n"
			}
			messages[l-1].Text += line
		}
		return false, nil
	}

	return messages, mv.Session().
		Sendln("module1").
		WaitLnContains("to release").
		Sendln("at+cmgf=1").
		Expect("0").
		Sendln(`AT+CMGL="ALL"`).
		ReadLines(callback).
		Sendln("AT+CMGD=0,1"). // Delete all read messages
		Expect("0").
		Close()
}

const timeout = 10 * time.Second

// Session facilitates an authenticated sessions, and provides chainable methods to send/receive commands
type Session struct {
	conn *telnet.Conn
	err  error
	log  *slog.Logger
}

// Session creates an authenticated connection to the MV370 for further use
func (mv Mv370) Session() *Session {
	s := &Session{
		log: mv.log,
	}

	mv.log.Info("New session")

	s.conn, s.err = telnet.DialTimeout("tcp", mv.host, timeout)
	if s.err != nil {
		return s
	}

	s.conn.SetUnixWriteMode(true)

	return s.
		Expect("username:").
		Sendln(mv.username).
		Expect("password").
		Sendln(mv.password).
		WaitLnContains("command:")
}

// Close a session, by sending "logout" and then closing the TCP connection
func (s *Session) Close() error {
	if s.conn != nil {
		s.conn.Write([]byte("logout\n"))
		s.conn.Close()
	}
	s.log.Info("Session closed")
	return s.err
}

// Expect one the given strings
func (s *Session) Expect(d ...string) *Session {
	if s.err != nil {
		return s
	}
	defer s.log.Debug("Expect", "delims", d, "result", s.err == nil)

	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("expect '%s'", strings.Join(d, "/")))
		return s
	}
	s.err = s.conn.SkipUntil(d...)
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("expect '%s'", strings.Join(d, "/")))
	}
	return s
}

// ExpectLnContains read the next line and expects it to contain str
func (s *Session) ExpectLnContains(str string) *Session {
	if s.err != nil {
		return s
	}
	defer s.log.Debug("ExpectLnContains", "str", str, "result", s.err == nil)

	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		return s
	}
	var line string
	line, s.err = s.conn.ReadString('\n')
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("ExpectLnContains '%s'", line))
	}
	if !strings.Contains(line, str) {
		s.err = fmt.Errorf("expected '%s' in '%s'", str, line)
	}
	return s
}

// WaitLnContains reads the lines until a line contains str
func (s *Session) WaitLnContains(str string) *Session {
	if s.err != nil {
		return s
	}
	defer s.log.Debug("WaitLnContains", "str", str, "result", s.err == nil)

	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		return s
	}
	var line string
	for {
		line, s.err = s.conn.ReadString(telnet.LF)
		if s.err != nil {
			s.err = errors.Join(s.err, fmt.Errorf("WaitLnContains '%s'", line))
			return s
		}
		if strings.Contains(line, str) {
			return s
		}
	}
}

// ReadLines read lines anc call fn with line, until fn returns an error or done
func (s *Session) ReadLines(fn ReadLinesFn) *Session {
	if s.err != nil {
		return s
	}
	var line string
	var done bool
	count := 0

	defer s.log.Debug("ReadLines", "count", count, "result", s.err == nil)

	for {
		s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
		if s.err != nil {
			s.err = errors.Join(s.err, errors.New("ReadLines"))
			return s
		}
		line, s.err = s.conn.ReadString(0x0A)
		line = strings.TrimRight(line, "\n\r")
		if s.err != nil {
			s.err = errors.Join(s.err, fmt.Errorf("ReadLines"))
		} else {
			count++
			done, s.err = fn(line)
			if done {
				return s
			}
		}
	}
}

// Sendln sends str
func (s *Session) Sendln(str string) *Session {
	if s.err != nil {
		return s
	}
	s.err = s.conn.SetWriteDeadline(time.Now().Add(timeout))
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("sendln '%s'", str))
		return s
	}
	buf := make([]byte, len(str)+1)
	copy(buf, str)
	buf[len(str)] = '\n'
	_, s.err = s.conn.Write(buf)
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("sendln '%s'", str))
	}
	s.log.Info("Sendln", "str", str)
	return s
}

// Send sends the raw bytes
func (s *Session) Send(b []byte) *Session {
	if s.err != nil {
		return s
	}
	s.err = s.conn.SetWriteDeadline(time.Now().Add(timeout))
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("send %X", b))
		return s
	}
	_, s.err = s.conn.Write(b)
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("sendln %X", b))
	}
	s.log.Info("Send", "b", b)
	return s
}
