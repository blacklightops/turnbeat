FROM perspicaio/golang

RUN apk add --update bash && rm -rf /var/cache/apk/*

RUN mkdir -p /opt/perspica

WORKDIR /opt/perspica

RUN touch /opt/perspica/turnbeat.yml
RUN chmod 0666 /opt/perspica/turnbeat.yml

ADD ./turnbeat /opt/perspica/turnbeat
ADD ./entry.sh /opt/perspica/entry.sh

RUN chmod 0777 /opt/perspica/entry.sh

ENV DEBUGOPTS -v
ENV TCPIN true
ENV PORT 2014
ENV TYPE tcp
ENV EVENTTYPE carbon
ENV FILTERS graphite
ENV REDIS true
ENV REDISHOST redis
ENV REDISPORT 6379
ENV REDISKEY turnbeat
ENV REDISDB 0
ENV STDOUT false
ENV REDISIN false
ENV REDISINHOST redis
ENV REDISINPORT 6379
ENV REDISINDB 0
ENV REDISINKEY turnbeat
ENV REDISINTYPE carbon

EXPOSE $PORT

CMD ["/bin/bash", "-c", "/opt/perspica/entry.sh"]
