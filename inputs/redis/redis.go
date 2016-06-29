package redis

import (
	"errors"
	"github.com/blacklightops/libbeat/common"
	"github.com/blacklightops/libbeat/logp"
	"github.com/blacklightops/turnbeat/inputs"
	"github.com/garyburd/redigo/redis"
	"fmt"
	"encoding/json"
	"strconv"
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

/*
        {
            "count": 1,
            "index": "CREASY01-MBP01",
            "line": 715,
            "message": "CREASY01-MBP01.interface-bridge0.if_octets.tx 0.000000 1466995490",
            "metric_name": "interface-bridge0.if_octets.tx",
            "metric_tags": "datacenter=CREASY01 host=MBP01",
            "metric_tags_map": {
                "datacenter": "CREASY01",
                "host": "MBP01"
            },
            "metric_timestamp": "1466995490",
            "metric_value": "0.000000",
            "offset": 45810,
            "shipper": "Jonathans-MacBook-Pro.local",
            "source": "[::1]:61956",
            "timestamp": "2016-06-26T21:44:50-05:00",
            "type": "carbon"
        }
*/

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
	var keysScript = redis.NewScript(1, `return redis.call('KEYS', KEYS[1])`)

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

			reply, err := keysScript.Do(server, "*")
			if err != nil {
				logp.Err("An error occured while executing KEYS command: %s\n", err)
				return
			}

			keys, err := redis.Strings(reply, err)
			if err != nil {
				logp.Err("An error occured while converting reply to String: %s\n", err)
				return
			}

			for _, key := range keys {
				logp.Debug("redisinput", "key is %s", key)
				lineCount, err := l.handleConn(server, output, key)
				if err == nil {
					logp.Debug("redisinput", "Read %v events", lineCount)
					backOffCount = 0
					backOffDuration = time.Duration(backOffCount) * time.Second
					time.Sleep(backOffDuration)
				} else {
					backOffCount++
					backOffDuration = time.Duration(backOffCount) * time.Second
					time.Sleep(backOffDuration)
				}
			}
			defer server.Close()
		}
	}()
	return nil
}

func (l *RedisInput) handleConn(server redis.Conn, output chan common.MapStr, key string) (uint64, error) {
	event_slice, offset, line, err := l.readKey(server, key)

	if err != nil {
		logp.Err("an error reading %s: %s\n", key, err)
	}

	now := func() time.Time {
		t := time.Now()
		return t
	}

	event := common.MapStr{}
	event["offset"] = offset
	event["count"]	= line
	event["type"]	= strings.TrimSpace(l.Type)
	event["source"] = strings.TrimSpace(key)
	event["datapoints"] = event_slice

	event.EnsureTimestampField(now)
	event.EnsureCountField()

	logp.Debug("redisinputlines", "event: %v", event)
	if event_slice != nil {
		output <- event // ship the new event downstream
	}

	logp.Debug("redisinput", "Finished reading from %s", key)
	return line, nil
}

func (l *RedisInput) readKey(server redis.Conn, key string) ([]common.MapStr, uint64, uint64, error) {
	var offset uint64 = 0
	var line uint64 = 0
	var prevTime uint64 = 0
	var thisTime uint64 = 0

	var events []common.MapStr

	var popScript = redis.NewScript(1, `return redis.call('LPOP', KEYS[1])`)
	var pushScript = redis.NewScript(2, `return redis.call('LPUSH', KEYS[1], KEYS[2])`)

	logp.Debug("redisinput", "Reading events from %s", key)

	for {
		reply, err := popScript.Do(server, key)
		if err != nil {
			logp.Info("[RedisInput] Unexpected state reading from %s; error: %s\n", key, err)
			return nil, line, offset, err
		}

		if reply == nil {
			logp.Debug("redisinputlines", "No values to read in LIST: %s", key)
			return events,line, offset, nil
		}

		text, err := redis.String(reply, err)
		if err != nil {
			logp.Info("[RedisInput] Unexpected state converting reply to String; error: %s\n", err)
			return nil, line, offset, err
		}
		offset += uint64(len(text))
		line++

		event := common.MapStr{}
		event["source"] = strings.TrimSpace(key)
		event["offset"] = offset
		event["line"] = line
		event["message"] = &text
		event["type"] = strings.TrimSpace(l.Type)
		expanded_event, err := l.Filter(event)

		metricTime, err := strconv.ParseInt(expanded_event["metric_timestamp"].(string), 10, 64)
		if err != nil {
			logp.Err("An error parsing the metric_timestamp: %s\n", err)
		}
		thisTime = uint64(metricTime)

		_, nowMin, _ := time.Now().Clock()

		prevTime_Time := time.Unix(int64(prevTime), 0)
		_, prevMin, _ := prevTime_Time.Clock()

		thisTime_Time := time.Unix(int64(thisTime), 0)
		_, thisMin, _ := thisTime_Time.Clock()

		event["timestamp"] = thisTime_Time.Format("2006-01-02T15:04:05Z07:00")

		logp.Debug("timestuff", "This Minute: %v, Prev Minute: %v, Now Minute: %v", thisMin, prevMin, nowMin)

		// If it has not been a minute since this event happened, put it back in the list.
		// TODO: change this to see if event is older than 60 seconds
		if nowMin == thisMin {
			logp.Debug("redisinput", "Skipping, not old enough")
			logp.Debug("timestuff", "pushing event: this min is still the current min")
			pushScript.Do(server, key, text)
			if len(events) > 0 {
				logp.Debug("timestuff", "returning previously collected events")
				events, err := l.GroupEvents(events)
				if err != nil {
					logp.Err("An error occured while grouping the events: %v\n", err)
				}
				return events, line, offset, nil
			} else {
				logp.Debug("timestuff", "sleeping 5 seconds, no collected events yet")
				time.Sleep(5 * time.Second)
			}
		} else {
			if thisMin <= prevMin || prevMin == 0 {
				prevTime = thisTime
				logp.Debug("timestuff", "appending event: this min is older than prev min, or prev min is 0")
				events = append(events, expanded_event)
			} else {
				pushScript.Do(server, key, text)
				logp.Debug("timestuff", "pushing event and returning: this min is later than prev minute")
				events, err := l.GroupEvents(events)
				if err != nil {
					logp.Err("An error occured while grouping the events: %v\n", err)
				}
				return events, line, offset, nil
			}
		}
	}
	logp.Debug("timestuff", "exited for loop, returning events")
	events, err := l.GroupEvents(events)
	if err != nil {
		logp.Err("An error occured while grouping the events: %v\n", err)
	}
	return events, line, offset, nil
}

// Seperate events by metric_name, average the values for each metric, emit averaged metrics
func (l *RedisInput) GroupEvents(events []common.MapStr) ([]common.MapStr, error) {
	var metric_name string
	var empty_events []common.MapStr
	sorted_events := map[string][]common.MapStr{}
	for _, event := range events {
		metric_name = event["metric_name"].(string)
		if sorted_events[metric_name] == nil {
			sorted_events[metric_name] = empty_events
		}
		sorted_events[metric_name] = append(sorted_events[metric_name], event)
	}
	output_events, err := l.averageSortedEvents(sorted_events)
	return output_events, err
}

func (l *RedisInput) averageSortedEvents(sorted_events map[string][]common.MapStr) ([]common.MapStr, error) {
	var output_events []common.MapStr
	var merged_event common.MapStr
	var metric_value_string string
	var metric_value_bytes []byte
	metric_value := 0.0
	for _, events := range sorted_events {
		metric_value = 0.0
		merged_event = common.MapStr{}
		for _, event := range events {
			merged_event.Update(event)
			logp.Debug("groupstuff", "metric value: %v", event["metric_value"])
			metric_value_string = event["metric_value"].(string)
			metric_value_bytes = []byte(metric_value_string)
			metric_value += float64(common.Bytes_Ntohll(metric_value_bytes))
		}
		logp.Debug("groupstuff", "the summed values is %v", metric_value)
		logp.Debug("groupstuff", "the length is %v", float64(len(events)))
		metric_value = metric_value / float64(len(events))
		logp.Debug("groupstuff", "the avg value is %v", metric_value)
		merged_event["metric_value"] = metric_value
		output_events = append(output_events, merged_event)
	}
	return output_events, nil
}

func (l *RedisInput) Filter(event common.MapStr) (common.MapStr, error) {
	text := event["message"]
	text_string := text.(*string)
	//logp.Debug("redisinput", "Attempting to expand: %v", event)

	if l.isJSONString(*text_string) {
		data := []byte(*text_string)
		err := json.Unmarshal(data, &event)
		if err != nil {
			logp.Err("redisinput", "Could not expand json data")
			return event, nil
		}
	} else {
		logp.Debug("redisinput", "Message does not appear to be JSON data: %s", text_string)
	}

	now := func() time.Time {
		t := time.Now()
		return t
	}

	event.EnsureTimestampField(now)

	//logp.Debug("redisinput", "Final Event: %v", event)
	return event, nil
}

func (l *RedisInput) isJSONString(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func (l *RedisInput) isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}