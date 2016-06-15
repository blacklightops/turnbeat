package tail

import (
	"bufio"
	"bytes"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"io"
	"os"
	"time"
)

type TailInput struct {
	Config   inputs.MothershipConfig
	Type     string
	FileName string
	RollTime time.Duration
	LastOpen time.Time
	FileP    *os.File
	offset   int64
}

func (l *TailInput) InputType() string {
	return "TailInput"
}

func (l *TailInput) InputVersion() string {
	return "0.0.1"
}

func (l *TailInput) Init(config inputs.MothershipConfig) error {
	l.Type = "tail"
	l.Config = config
	l.FileName = config.Filename
	l.RollTime = 30 * time.Minute

	logp.Info("[TailInput] Initialized with file " + l.FileName)
	return nil
}

func (l *TailInput) GetConfig() inputs.MothershipConfig {
	return l.Config
}

func (l *TailInput) Run(output chan common.MapStr) error {
	logp.Debug("[TailInput]", "Running File Input")

	// dispatch thread here
	go func(output chan common.MapStr) {
		l.doStuff(output)
	}(output)

	return nil
}

func (l *TailInput) CheckReopen() {
	// periodically reopen the file, in case the file has been rolled
	if time.Since(l.LastOpen) > l.RollTime {
		l.FileP.Close()
		var err error
		l.FileP, err = os.Open(l.FileName)
		if err != nil {
			logp.Err("Error opening file " + err.Error())
			return
		}

		// this time we do not seek to end
		// since in the case of a roll, we want to capture everything
		l.offset = 0
		l.LastOpen = time.Now()
	}
}

func (l *TailInput) doStuff(output chan common.MapStr) {
	now := func() time.Time {
		t := time.Now()
		return t
	}

	var line uint64 = 0
	var read_timeout = 30 * time.Second

	// open file
	// basic error handling, if we hit an error, log and return
	// this ends the currently running thread without impacting other threads
	f, err := os.Open(l.FileName)
	if err != nil {
		logp.Err("Error opening file " + err.Error())
		return
	}
	l.FileP = f

	// seek to end
	// for offset, we use the actual file offset
	// we initialize it to the end of the file at time of open
	l.offset, err = l.FileP.Seek(0, 2)
	if err != nil {
		logp.Err("Error seeking in file " + err.Error())
		return
	}
	l.LastOpen = time.Now()

	buffer := new(bytes.Buffer)
	reader := bufio.NewReader(l.FileP)

	for {
		l.CheckReopen()
		text, bytesread, err := readline(reader, buffer, read_timeout)
		if err != nil && err != io.EOF {
			// EOF errors are expected, since we are tailing the file
			logp.Err("Error reading file " + err.Error())
			return
		}

		if bytesread > 0 {
			l.offset += int64(bytesread)
			line++

			event := common.MapStr{}
			event["filename"] = l.FileName
			event["line"] = line
			event["message"] = text
			event["offset"] = l.offset
			event["type"] = l.Type

			event.EnsureTimestampField(now)
			event.EnsureCountField()

			logp.Debug("tailinput", "InputEvent: %v", event)
			output <- event // ship the new event downstream
		}
	}
}

func readline(reader *bufio.Reader, buffer *bytes.Buffer, eof_timeout time.Duration) (*string, int, error) {
	var is_partial bool = true
	var newline_length int = 1
	start_time := time.Now()

	logp.Debug("tailinputlines", "Readline Called")

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
