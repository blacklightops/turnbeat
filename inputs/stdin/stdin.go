package stdin

/* an empty input, useful as a starting point for a new input */

import (
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"os"
	"fmt"
	"time"
	"io"
    "bufio"
	"bytes"
)

type StdinInput struct {
	Config inputs.MothershipConfig
	Type   string
}

func (l *StdinInput) InputType() string {
	return "StdinInput"
}

func (l *StdinInput) InputVersion() string {
	return "0.0.1"
}

func (l *StdinInput) Init(config inputs.MothershipConfig) error {
	l.Type = "stdin"
	l.Config = config
	logp.Info("[StdinInput] Initialized")
	return nil
}

func (l *StdinInput) GetConfig() inputs.MothershipConfig {
	return l.Config
}

// below is an example of a run for an "interrupt" style input
// see bottom for a "periodic" style input
func (l *StdinInput) Run(output chan common.MapStr) error {
	logp.Debug("[StdinInput]", "Running stdin Input")

	// dispatch thread here
	go func(output chan common.MapStr) {
		l.doStuff(output)
	}(output)

	return nil
}

func (l *StdinInput) doStuff(output chan common.MapStr) {
	reader := bufio.NewReader(os.Stdin)
	buffer := new(bytes.Buffer)

	var source string = fmt.Sprintf("%s:%s", os.Getenv("REMOTE_HOST"), os.Getenv("REMOTE_PORT"))
	var ssl_client_dn string = os.Getenv("SSL_CLIENT_DN")
	var offset int64 = 0
	var line uint64 = 0
	var read_timeout = 10 * time.Second

	logp.Debug("stdinput", "Handling New Connection from %s", source)

	now := func() time.Time {
		t := time.Now()
		return t
	}

	for {
		text, bytesread, err := l.readline(reader, buffer, read_timeout)

		if err != nil {
			logp.Info("Unexpected state reading from %v; error: %s\n", os.Getenv("SSL_CLIENT_DN"), err)
			return
		}

		logp.Debug("stdinputlines", "New Line: %s", &text)

		line++

		event := common.MapStr{}
		event["ssl_client_dn"] = &ssl_client_dn
		event["source"] = &source
		event["offset"] = offset
		event["line"] = line
		event["message"] = text
		event["type"] = l.Type

		event.EnsureTimestampField(now)
		event.EnsureCountField()

		offset += int64(bytesread)

		logp.Debug("stdinput", "InputEvent: %v", event)
		output <- event // ship the new event downstream
		os.Stdout.Write([]byte("OK"))
	}
	logp.Debug("stdinput", "Closed Connection from %s", source)
}

func (l *StdinInput) readline(reader *bufio.Reader, buffer *bytes.Buffer, eof_timeout time.Duration) (*string, int, error) {
	var is_partial bool = true
	var newline_length int = 1
	start_time := time.Now()

	logp.Debug("stdinputlines", "Readline Called")

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