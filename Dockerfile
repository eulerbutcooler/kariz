FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/surang-server ./cmd/surang-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/surang-server /usr/local/bin/surang-server
VOLUME [ "/data" ]
EXPOSE 5555 8000 9000
ENTRYPOINT [ "/usr/local/bin/surang-server" ]
