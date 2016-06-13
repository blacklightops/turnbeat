FROM perspicaio/golang

RUN apk add --update bash && rm -rf /var/cache/apk/*

RUN mkdir -p /opt/perspica

WORKDIR /opt/perspica

ADD ./turnbeat /opt/perspica/turnbeat
ADD ./turnbeat.yml /opt/perspica/turnbeat.yml

ENV PORT 2004
EXPOSE 2004

CMD ["/opt/perspica/turnbeat", "-v", "-d", "reader,tcpinput,udpinput"]
