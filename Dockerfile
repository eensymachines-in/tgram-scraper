FROM golang:1.21.1-alpine3.17
RUN apk add git
RUN mkdir -p /usr/src/eensymachines/ && mkdir -p /usr/bin/eensymachines
WORKDIR /usr/src/eensymachines/
COPY . .
RUN go mod download 
RUN chmod -R +x /usr/bin/eensymachines
RUN go build -o /usr/bin/eensymachines/scraper .
ENTRYPOINT /usr/bin/eensymachines/scraper