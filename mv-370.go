package mv370

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ziutek/telnet"
)

type Mv370 struct {
	host     string
	username string
	password string
}

type ReadLinesFn func(line string) (done bool, err error)

func New(host string, username string, password string) (Mv370, error) {
	mv := Mv370{
		host:     host,
		username: username,
		password: password,
	}
	return mv, mv.Check()
}

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
	Tel     string    `json:"tel"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
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
			if len(messages[l-1].Message) > 0 {
				messages[l-1].Message += "\n"
			}
			messages[l-1].Message += line
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

type Session struct {
	conn *telnet.Conn
	err  error
}

func (mv Mv370) Session() *Session {
	s := &Session{}

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
		Expect("command:")
}

func (s *Session) Close() error {
	if s.conn != nil {
		s.conn.Write([]byte("logout\n"))
		s.conn.Close()
	}
	return s.err
}

// Expect one the given strings
func (s *Session) Expect(d ...string) *Session {
	if s.err != nil {
		return s
	}
	log.Printf("Expect A: %s", d)
	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		s.err = errors.Join(s.err, fmt.Errorf("expect '%s'", strings.Join(d, "/")))
		return s
	}
	log.Printf("Expect B: %s", d)
	s.err = s.conn.SkipUntil(d...)
	log.Printf("Expect C: %s: %v", d, s.err)
	if s.err != nil {
		log.Printf("Expect ERR: %v", s.err)
		s.err = errors.Join(s.err, fmt.Errorf("expect '%s'", strings.Join(d, "/")))
	}
	return s
}

func (s *Session) ExpectLnContains(str string) *Session {
	if s.err != nil {
		return s
	}
	log.Printf("ExpectLnContains A: %s", str)
	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		return s
	}
	log.Printf("ExpectLnContains B: %s", str)
	var line string
	line, s.err = s.conn.ReadString('\n')
	log.Printf("ExpectLnContains C: %s: %v", line, s.err)
	if s.err != nil {
		log.Printf("ExpectLnContains ERR: %v", s.err)
		s.err = errors.Join(s.err, fmt.Errorf("ExpectLnContains '%s'", line))
	}
	if !strings.Contains(line, str) {
		s.err = fmt.Errorf("expected '%s' in '%s'", str, line)
	}
	return s
}

func (s *Session) WaitLnContains(str string) *Session {
	if s.err != nil {
		return s
	}
	log.Printf("WaitLnContains A: %s", str)
	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
	if s.err != nil {
		return s
	}
	log.Printf("WaitLnContains B: %s", str)
	var line string
	for {
		line, s.err = s.conn.ReadString('\n')
		log.Printf("WaitLnContains C: %s: %v", line, s.err)
		if s.err != nil {
			log.Printf("WaitLnContains ERR: %v", s.err)
			s.err = errors.Join(s.err, fmt.Errorf("WaitLnContains '%s'", line))
		}
		if strings.Contains(line, str) {
			return s
		}
	}
}

// func (s *Session) ReadLn(fn ReadCallback) *Session {
// 	if s.err != nil {
// 		return s
// 	}
// 	s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
// 	if s.err != nil {
// 		s.err = errors.Join(s.err, errors.New("readln"))
// 		return s
// 	}
// 	var b []byte
// 	b, s.err = s.conn.ReadUntil("\n")
// 	log.Printf("ReadLn A: '%s': %v", string(b), s.err)
// 	if s.err != nil {
// 		log.Printf("ReadLn ERR: %v", s.err)
// 		s.err = errors.Join(s.err, fmt.Errorf("readln"))
// 	} else {
// 		s.err = fn(string(b))
// 	}
// 	return s
// }

func (s *Session) ReadLines(fn ReadLinesFn) *Session {
	if s.err != nil {
		return s
	}
	var line string
	var done bool

	for {
		s.err = s.conn.SetReadDeadline(time.Now().Add(timeout))
		if s.err != nil {
			s.err = errors.Join(s.err, errors.New("ReadLinesUntil"))
			return s
		}
		line, s.err = s.conn.ReadString(0x0A)
		line = strings.TrimRight(line, "\n\r")
		log.Printf("ReadLinesUntil A: '%s': %v", line, s.err)
		if s.err != nil {
			log.Printf("ReadLinesUntil ERR: %v", s.err)
			s.err = errors.Join(s.err, fmt.Errorf("ReadLinesUntil"))
		} else {
			log.Printf("ReadLinesUntil B: '%s': %v", line, s.err)
			done, s.err = fn(line)
			log.Printf("ReadLinesUntil C: %t, '%s': %v", done, line, s.err)
			if done {
				return s
			}
		}
	}
}

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
	log.Printf("Sendln '%s'", str)
	return s
}

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
	log.Printf("Send %X", b)
	return s
}
