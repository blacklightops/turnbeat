package redis

import (
	"errors"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"github.com/garyburd/redigo/redis"
	"fmt"
	"time"
	"strings"
)

type RedisInput struct {
	Config inputs.MothershipConfig
	Host	string	/* the host to connect to */
	Port	int		/* the port to connect to */
	DB		int		/* the database to read from */
	Key		string	/* the key to POP from */
	Type	string	/* the type to add to events */
}

func (l *RedisInput) InputType() string {
	return "RedisInput"
}

func (l *RedisInput) InputVersion() string {
	return "0.0.1"
}

func (l *RedisInput) Init(config inputs.MothershipConfig) error {

	l.Config = config

	if config.Host == "" {
		return errors.New("No Input Host specified")
	}
	l.Host = config.Host

	if config.Port == 0 {
		return errors.New("No Input Port specified")
	}
	l.Port = config.Port

	l.DB = config.DB

	if config.Key == "" {
		return errors.New("No Input Key specified")
	}
	l.Key = strings.TrimSpace(config.Key)

	if config.Type == "" {
		return errors.New("No Event Type specified")
	}
	l.Type = strings.TrimSpace(config.Type)

	logp.Debug("redisinput", "Using Host %s", l.Host)
	logp.Debug("redisinput", "Using Port %d", l.Port)
	logp.Debug("redisinput", "Using Database %d", l.DB)
	logp.Debug("redisinput", "Using Key %s", l.Key)
	logp.Debug("redisinput", "Adding Event Type %s", l.Type)

	return nil
}

func (l *RedisInput) GetConfig() inputs.MothershipConfig {
	return l.Config
}

func (l *RedisInput) Run(output chan common.MapStr) error {
	logp.Debug("redisinput", "Running Redis Input")
	// dispatch the master listen thread
	var existsScript = redis.NewScript(1, `return redis.call('EXISTS', KEYS[1])`)

	go func() {
		redisURL := fmt.Sprintf("redis://%s:%d/%d", l.Host, l.Port, l.DB)
		dialConnectTimeout := redis.DialConnectTimeout(3 * time.Second)
		dialReadTimeout := redis.DialReadTimeout(10 * time.Second)
		var backOffCount = 0
		var backOffDuration time.Duration = 5 * time.Second
		for {
			logp.Debug("redisinput", "Connecting to: %s", redisURL)
			server, err := redis.DialURL(redisURL, dialConnectTimeout, dialReadTimeout)
			if err != nil {
				logp.Err("couldn't start listening: " + err.Error())
				return
			}
			logp.Debug("redisinput", "Connected to Redis Server")
			reply, err := existsScript.Do(server, l.Key)
			if err != nil {
				logp.Err("An error occured while executing EXISTS command: %s\n", err)
				return
			}
			exists, err := redis.Int(reply, err)
			if err != nil {
				logp.Err("An error occured while converting reply to Int: %s\n", err)
				return
			}
			if exists == 1 {
				lineCount, err := l.handleConn(server, output)
				if err == nil {
					backOffCount = 0
					backOffDuration = time.Duration(backOffCount) * time.Second
					time.Sleep(backOffDuration)
				} else {
					backOffCount++
					backOffDuration = time.Duration(backOffCount) * time.Second
					time.Sleep(backOffDuration)
				}
				logp.Debug("redisinput", "Read %v events", lineCount)
			} else {
				logp.Info("[RedisInput] Key %s does not exist", l.Key)
				backOffCount++
				backOffDuration = time.Duration(backOffCount) * time.Second
				time.Sleep(backOffDuration)
			}
			defer server.Close()
		}
	}()
	return nil
}

func (l *RedisInput) handleConn(server redis.Conn, output chan common.MapStr) (uint64, error) {
	var offset int64 = 0
	var line uint64 = 0
	var bytesread uint64 = 0
	var popScript = redis.NewScript(1, `return redis.call('LPOP', KEYS[1])`)

	logp.Debug("redisinput", "Reading events from %s", l.Key)

	now := func() time.Time {
		t := time.Now()
		return t
	}

	for {
		reply, err := popScript.Do(server, l.Key)
		if err != nil {
			logp.Info("[RedisInput] Unexpected state reading from %s; error: %s\n", l.Key, err)
			return line, err
		}
		if reply == nil {
			logp.Debug("redisinputlines", "No values to read in LIST: %s", l.Key)
			return line, nil
		}
		text, err := redis.String(reply, err)
		if err != nil {
			logp.Info("[RedisInput] Unexpected state converting reply to String; error: %s\n", err)
			return line, err
		}
		bytesread += uint64(len(text))

		logp.Debug("redisinputlines", "New Line: %s", &text)

		line++

		event := common.MapStr{}
		event["source"] = strings.TrimSpace(l.Key)
		event["offset"] = offset
		event["line"] = line
		event["message"] = &text
		event["type"] = strings.TrimSpace(l.Type)

		event.EnsureTimestampField(now)
		event.EnsureCountField()

		offset += int64(bytesread)

		logp.Debug("redisinput", "InputEvent: %v", event)
		output <- event // ship the new event downstream
	}
	logp.Debug("redisinput", "Finished reading from %s", l.Key)
	return line, nil
}