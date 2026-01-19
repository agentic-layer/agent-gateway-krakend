ARG KRAKENX_VERSION=2.12.1

# NOTE: golang version must match exactly the one in https://github.com/devopsfaith/krakend-ce/blob/v2.12.1/Makefile
FROM golang:1.25.4-trixie AS builder
ARG KRAKENX_VERSION

RUN apt-get update && \
	apt-get install -y ca-certificates && \
	update-ca-certificates

WORKDIR /krakend-ce
RUN git clone --depth 1 --branch v${KRAKENX_VERSION} https://github.com/devopsfaith/krakend-ce.git /krakend-ce
RUN make build

COPY go /maelstrom
WORKDIR /maelstrom
RUN go get -t ./...

RUN /krakend-ce/krakend check-plugin --format --libc "GLIBC-2.41_(debian-13)"
RUN go build -buildmode=plugin -o openai-a2a.so ./plugin/openai-a2a
RUN go build -buildmode=plugin -o agentcard-rw.so ./plugin/agentcard-rw

FROM gcr.io/distroless/base-debian12
ARG KRAKENX_VERSION

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /krakend-ce/krakend /usr/bin/krakend
COPY --from=builder /maelstrom/*.so /unleash/tentacles/

ENV KRAKENX_VERSION="$KRAKENX_VERSION"
ENV GIN_MODE="release"
ENV USAGE_DISABLE=1
ENV FC_ENABLE=1

USER 1000
VOLUME [ "/etc/krakend" ]
WORKDIR /etc/krakend
ENTRYPOINT [ "/usr/bin/krakend" ]
CMD [ "run", "-c", "/etc/krakend/krakend.json" ]
EXPOSE 8080 9090
