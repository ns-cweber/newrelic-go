FROM golang

RUN mkdir -p /go/src/github.com/ns-cweber/nrql2csv

COPY . /go/src/github.com/ns-cweber/nrql2csv

RUN go install github.com/ns-cweber/nrql2csv/cmd/nrqld

CMD /go/bin/nrqld
