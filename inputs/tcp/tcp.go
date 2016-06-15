package tcp

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
  "crypto/tls"
	"io"
	"net"
	"strconv"
	"time"
)

type TcpInput struct {
	Config inputs.MothershipConfig
	Port   int    /* the port to listen on */
  Tls    boolean /* Whether or not to use TLS */
	Type   string /* the type to add to events */
  Key    string /* the TLS Key File */
  Cert   string /* the TLS Cert File */
}

func (l *TcpInput) InputType() string {
	return "TcpInput"
}

func (l *TcpInput) InputVersion() string {
	return "0.0.1"
}

func (l *TcpInput) Init(config inputs.MothershipConfig) error {

	l.Config = config
	if config.Port == 0 {
		return errors.New("No Input Port specified")
	}
	l.Port = config.Port

	if config.Type == "" {
		return errors.New("No Event Type specified")
	}
	l.Type = config.Type

  if config.Tls {
    l.Tls = true
  	if config.Key == "" {
  		return errors.New("No TLS Key specified")
  	}
  	if config.Cert == "" {
  		return errors.New("No TLS Certificate specified")
  	}
    l.Key = config.Key
    l.Cert = config.Cert
  } else {
    l.Tls = false
  }

	logp.Debug("tcpinput", "Using Port %d", l.Port)
	logp.Debug("tcpinput", "Using TLS %b", l.Tls)
	logp.Debug("tcpinput", "Adding Event Type %s", l.Type)

	return nil
}

func (l *TcpInput) GetConfig() inputs.MothershipConfig {
	return l.Config
}

func (l *TcpInput) Run(output chan common.MapStr) error {
	logp.Info("[TcpInput] Running TCP Input")
  if l.Tls {
    cer, err := tls.LoadX509KeyPair(l.Key, l.Cert)
    if err != nil {
        log.Println(err)
        return
    }
    config := &tls.Config{Certificates: []tls.Certificate{cer}}
    server, err := tls.Listen("tcp", ":"+strconf.Itoa(l.Port), config)
    defer server.Close()
  } else {
  	server, err := net.Listen("tcp", ":"+strconv.Itoa(l.Port))
  }
	if err != nil {
		logp.Err("couldn't start listening: " + err.Error())
		return nil
	}
	logp.Info("[TcpInput] Listening on port %d", l.Port)

	// dispatch the master listen thread
	go func(server net.Listener) {
		for {
			// accept incoming connections
			conn, err := server.Accept()
			if err != nil {
				logp.Err("Error accepting: ", err.Error())
			} else {
				// dispatch individual connection handler threads
				go l.handleConn(conn, output)
			}
		}
	}(server)
	return nil
}

func (l *TcpInput) handleConn(client net.Conn, output chan common.MapStr) {
	reader := bufio.NewReader(client)
	buffer := new(bytes.Buffer)

	var source string = client.RemoteAddr().String()
	var offset int64 = 0
	var line uint64 = 0
	var read_timeout = 10 * time.Second

	logp.Debug("tcpinput", "Handling New Connection from %s", source)

	now := func() time.Time {
		t := time.Now()
		return t
	}

	for {
		text, bytesread, err := l.readline(reader, buffer, read_timeout)

		if err != nil {
			logp.Info("Unexpected state reading from %v; error: %s\n", client.RemoteAddr().String, err)
			return
		}

		logp.Debug("tcpinputlines", "New Line: %s", &text)

		line++

		event := common.MapStr{}
		event["source"] = &source
		event["offset"] = offset
		event["line"] = line
		event["message"] = text
		event["type"] = l.Type

		event.EnsureTimestampField(now)
		event.EnsureCountField()

		offset += int64(bytesread)

		logp.Debug("tcpinput", "InputEvent: %v", event)
		output <- event // ship the new event downstream
		client.Write([]byte("OK"))
	}
	logp.Debug("tcpinput", "Closed Connection from %s", source)
}

func (l *TcpInput) readline(reader *bufio.Reader, buffer *bytes.Buffer, eof_timeout time.Duration) (*string, int, error) {
	var is_partial bool = true
	var newline_length int = 1
	start_time := time.Now()

	logp.Debug("tcpinputlines", "Readline Called")

	for {
		segment, err := reader.ReadBytes('\n')

		if segment != nil && len(segment) > 0 {
			if segment[len(segment)-1] == '\n' {
				// Found a complete line
				is_partial = false

				// Check if also a CR present
				if len(segment) > 1 && segment[len(segment)-2] == '\r' {
					newline_length++
				}
			}

			// TODO(sissel): if buffer exceeds a certain length, maybe report an error condition? chop it?
			buffer.Write(segment)
		}

		if err != nil {
			if err == io.EOF && is_partial {
				time.Sleep(1 * time.Second) // TODO(sissel): Implement backoff

				// Give up waiting for data after a certain amount of time.
				// If we time out, return the error (eof)
				if time.Since(start_time) > eof_timeout {
					return nil, 0, err
				}
				continue
			} else {
				//emit("error: Harvester.readLine: %s", err.Error())
				return nil, 0, err // TODO(sissel): don't do this?
			}
		}

		// If we got a full line, return the whole line without the EOL chars (CRLF or LF)
		if !is_partial {
			// Get the str length with the EOL chars (LF or CRLF)
			bufferSize := buffer.Len()
			str := new(string)
			*str = buffer.String()[:bufferSize-newline_length]
			// Reset the buffer for the next line
			buffer.Reset()
			return str, bufferSize, nil
		}
	} /* forever read chunks */

	return nil, 0, nil
}
