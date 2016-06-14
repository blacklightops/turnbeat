FROM perspicaio/golang

RUN apk add --update bash && rm -rf /var/cache/apk/*

RUN mkdir -p /opt/perspica

WORKDIR /opt/perspica

RUN touch /opt/perspica/turnbeat.yml
RUN chmod 0666 /opt/perspica/turnbeat.yml

ADD ./turnbeat /opt/perspica/turnbeat
ADD ./entry.sh /opt/perspica/entry.sh

RUN chmod 0777 /opt/perspica/entry.sh

ENV PORT 2014
ENV TYPE tcp
ENV EVENTTYPE carbon
ENV FILTERS graphite
EXPOSE $PORT

CMD ["/bin/bash", "-c", "/opt/perspica/entry.sh"]
