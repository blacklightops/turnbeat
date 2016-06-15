package redis

import (
	"errors"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"github.com/garyburd/redigo/redis"
	"fmt"
	"time"
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
	l.Key = config.Key

	if config.Type == "" {
		return errors.New("No Event Type specified")
	}
	l.Type = config.Type

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
	logp.Info("[RedisInput] Running Redis Input")
	redisHostname := fmt.Sprintf("%s:%d", l.Host, l.Port)
	server, err := redis.Dial("tcp", redisHostname)
	if err != nil {
		logp.Err("couldn't start listening: " + err.Error())
		return nil
	}
	logp.Info("[RedisInput] Connected to Redis Server")

	// dispatch the master listen thread
	go func(server redis.Conn) {
		for {
			exists, err := redis.Bool(server.Do("EXISTS", redis.Args{}.Add(l.Key)))
			if err != nil {
				logp.Err("An error occured while executing EXISTS command")
				return
			}
			if exists != true {
				logp.Err("Key %s does not exist!", l.Key)
				return
			}
			l.handleConn(server, output)
		}
	}(server)
	return nil
}

func (l *RedisInput) handleConn(server redis.Conn, output chan common.MapStr) {
	var offset int64 = 0
	var line uint64 = 0
	var bytesread uint64 = 0

	logp.Debug("redisinput", "Reading events from %s", l.Key)

	now := func() time.Time {
		t := time.Now()
		return t
	}

	for {
		reply, err := server.Do("LPOP", redis.Args{}.Add(l.Key))
		text, err := redis.String(reply, err)
		bytesread += uint64(len(text))

		if err != nil {
			logp.Info("Unexpected state reading from %s; error: %s\n", l.Key, err)
			return
		}

		logp.Debug("redisinputlines", "New Line: %s", &text)

		line++

		event := common.MapStr{}
		event["source"] = l.Key
		event["offset"] = offset
		event["line"] = line
		event["message"] = text
		event["type"] = l.Type

		event.EnsureTimestampField(now)
		event.EnsureCountField()

		offset += int64(bytesread)

		logp.Debug("redisinput", "InputEvent: %v", event)
		output <- event // ship the new event downstream
	}
	logp.Debug("redisinput", "Finished reading from %s", l.Key)
}